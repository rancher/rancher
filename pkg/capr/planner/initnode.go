package planner

import (
	"errors"
	"fmt"
	"maps"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// protectedInitTarget returns the deleting init node that must remain the join target during ordinary reconciliation.
func protectedInitTarget(controlPlane *rkev1.RKEControlPlane, plan *plan.Plan) *planEntry {
	if controlPlane.Labels[capr.InitNodeMachineIDLabel] != "" {
		return nil
	}
	for _, entry := range collect(plan, isInitNode) {
		if isEtcd(entry) && isDeleting(entry) && entry.Metadata.Annotations[capr.JoinURLAnnotation] != "" {
			return entry
		}
	}
	return nil
}

// protectedInitReplacement returns the machine that takes over the init mark from the deleting target, or nil.
// A candidate that already carries the init mark comes first: the move was interrupted and the saved mark is the decision.
// Otherwise the first candidate joined to the target that has its own join URL and is an etcd voter.
func (p *Planner) protectedInitReplacement(controlPlane *rkev1.RKEControlPlane, plan *plan.Plan, target *planEntry) *planEntry {
	joinURL := target.Metadata.Annotations[capr.JoinURLAnnotation]
	candidates := collect(plan, canBeInitNode)
	for _, candidate := range candidates {
		if isInitNode(candidate) && candidate.Metadata.Annotations[capr.JoinedToAnnotation] == joinURL {
			return candidate
		}
	}
	for _, candidate := range candidates {
		if candidate.Metadata.Annotations[capr.JoinedToAnnotation] == joinURL &&
			candidate.Metadata.Annotations[capr.JoinURLAnnotation] != "" &&
			p.hasEtcdVoter(controlPlane, candidate) {
			return candidate
		}
	}
	return nil
}

// findInitNodeForOperation lets snapshot, certificate and encryption-key operations wait during a protected
// hand-off. It reports no init node without an error so callers can skip the operation and allow ordinary
// reconciliation to continue. The existing pause check still applies.
func (p *Planner) findInitNodeForOperation(rkeControlPlane *rkev1.RKEControlPlane, plan *plan.Plan) (bool, string, *planEntry, error) {
	if target := protectedInitTarget(rkeControlPlane, plan); target != nil {
		logrus.Infof("[planner] rkecluster %s/%s: deferring operation while init node %s is handing off to its replacement", rkeControlPlane.Namespace, rkeControlPlane.Name, target.Machine.Name)
		return false, "", nil, nil
	}
	return p.findInitNode(rkeControlPlane, plan)
}

// hasEtcdVoter checks for the candidate's cached EtcdIsVoter=True condition, published by K3s/RKE2 after observing it
// as a voting etcd member. The planner combines this evidence with the plan's joined-to URL before promoting a
// replacement to init node. This records a past join, not current health; missing data or cache errors provide no proof.
func (p *Planner) hasEtcdVoter(controlPlane *rkev1.RKEControlPlane, candidate *planEntry) bool {
	if p.managementNodes == nil || controlPlane.Spec.ManagementClusterName == "" || candidate.Machine.UID == "" {
		return false
	}
	nodes, err := p.managementNodes.List(controlPlane.Spec.ManagementClusterName, labels.Everything())
	if err != nil {
		logrus.Warnf("[planner] rkecluster %s/%s: failed to list management nodes for etcd voter check on machine %s/%s: %v",
			controlPlane.Namespace, controlPlane.Name, candidate.Machine.Namespace, candidate.Machine.Name, err)
		return false
	}
	// Find the candidate's Node by the Machine UID label copied from downstream.
	for _, node := range nodes {
		if node.Status.NodeLabels[capr.MachineUIDLabel] != string(candidate.Machine.UID) {
			continue
		}
		for _, condition := range node.Status.InternalNodeStatus.Conditions {
			if condition.Type == corev1.NodeConditionType("EtcdIsVoter") && condition.Status == corev1.ConditionTrue {
				return true
			}
		}
	}
	return false
}

// moveInitNodeMark marks newInit before clearing oldInit's mark.
// newInit must have passed the promotion gate or already be the persisted choice.
func (p *Planner) moveInitNodeMark(oldInit, newInit *planEntry) error {
	setErr := p.setInitNodeMark(newInit)
	if setErr != nil && !errors.Is(setErr, generic.ErrSkip) {
		return setErr
	}

	if clearErr := p.clearInitNodeMark(oldInit); clearErr != nil {
		return clearErr
	}

	return setErr
}

// clearInitNodeMark removes the init node label from the plan Secret, demoting it in the current plan.
func (p *Planner) clearInitNodeMark(entry *planEntry) error {
	if entry.Metadata.Labels[capr.InitNodeLabel] == "" {
		return nil
	}

	if err := p.store.removePlanSecretLabel(entry, capr.InitNodeLabel); err != nil {
		return err
	}
	// We've changed state, so let the caches sync up again
	return generic.ErrSkip
}

// setInitNodeMark persists the init node label in the plan Secret.
func (p *Planner) setInitNodeMark(entry *planEntry) error {
	if entry.Metadata.Labels[capr.InitNodeLabel] == "true" {
		return nil
	}

	updated := *entry
	metadata := *entry.Metadata
	metadata.Labels = maps.Clone(metadata.Labels)
	metadata.Labels[capr.InitNodeLabel] = "true"
	updated.Metadata = &metadata

	if err := p.store.updatePlanSecretLabelsAndAnnotations(&updated); err != nil {
		return err
	}

	entry.Metadata.Labels = updated.Metadata.Labels
	entry.Plan = updated.Plan

	return generic.ErrSkip
}

// findAndDesignateFixedInitNode is used for rancherd where an exact machine (determined by labeling the
// rkecontrolplane object) is desired to be the init node
func (p *Planner) findAndDesignateFixedInitNode(rkeControlPlane *rkev1.RKEControlPlane, plan *plan.Plan) (bool, string, *planEntry, error) {
	logrus.Debugf("rkecluster %s/%s: finding and designating fixed init node", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName)
	fixedMachineID := rkeControlPlane.Labels[capr.InitNodeMachineIDLabel]
	if fixedMachineID == "" {
		return false, "", nil, fmt.Errorf("fixed machine ID label did not exist on rkecontrolplane")
	}
	entries := collect(plan, func(entry *planEntry) bool {
		return entry.Metadata != nil && entry.Metadata.Labels[capr.MachineIDLabel] == fixedMachineID
	})
	if len(entries) > 1 {
		return false, "", nil, fmt.Errorf("multiple machines found with identical machine ID label %s=%s", capr.MachineIDLabel, fixedMachineID)
	} else if len(entries) == 0 {
		return false, "", nil, fmt.Errorf("fixed machine with ID %s not found", fixedMachineID)
	}
	if entries[0].Metadata.Labels[capr.InitNodeLabel] != "true" {
		logrus.Debugf("rkecluster %s/%s: setting designated init node to fixedMachineID: %s", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName, fixedMachineID)
		allInitNodes := collect(plan, isEtcd)
		// clear all init node marks and return a generic.ErrSkip if we invalidated caches during clearing
		cachesInvalidated := false
		for _, entry := range allInitNodes {
			if entry.Machine.Labels[capr.MachineIDLabel] == fixedMachineID {
				continue
			}
			err := p.clearInitNodeMark(entry)
			if err != nil && !errors.Is(err, generic.ErrSkip) {
				// if we received a strange error attempting to clear the init node mark
				return false, "", nil, err
			} else if errors.Is(err, generic.ErrSkip) {
				cachesInvalidated = true
			}
		}
		if cachesInvalidated {
			return false, "", nil, generic.ErrSkip
		}

		return true, entries[0].Metadata.Annotations[capr.JoinURLAnnotation], entries[0], p.setInitNodeMark(entries[0])
	}
	logrus.Debugf("rkecluster %s/%s: designated init node %s found", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName, fixedMachineID)
	return true, entries[0].Metadata.Annotations[capr.JoinURLAnnotation], entries[0], nil
}

// findInitNode searches the given cluster for the init node. It returns a bool which is whether an init node was
// found, the init node join URL, and an error for a few conditions, i.e. if multiple init nodes were found or if there
// is a more suitable init node. Notably, if multiple init nodes are found, it will return false as it could not come to
// consensus on a single init node
func (p *Planner) findInitNode(rkeControlPlane *rkev1.RKEControlPlane, plan *plan.Plan) (bool, string, *planEntry, error) {
	logrus.Debugf("rkecluster %s/%s searching for init node", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName)
	// if the rkecontrolplane object has an InitNodeMachineID label, we need to find the fixedInitNode.
	if rkeControlPlane.Labels[capr.InitNodeMachineIDLabel] != "" {
		return p.findAndDesignateFixedInitNode(rkeControlPlane, plan)
	}

	currentInitNodes := collect(plan, isInitNode)

	if len(currentInitNodes) > 1 {
		// if multiple init nodes are found, we don't know which one to return so return false with an error to hopefully trigger a re-election
		return false, "", nil, fmt.Errorf("multiple init nodes found")
	}

	initNodeFound := false
	var initNode *planEntry
	// this loop should never execute more than once
	for _, entry := range currentInitNodes {
		if canBeInitNode(entry) {
			initNodeFound = true
			initNode = entry
			joinURL := entry.Metadata.Annotations[capr.JoinURLAnnotation]
			logrus.Debugf("rkecluster %s/%s found current init node %s with joinURL: %s", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName, entry.Machine.Name, joinURL)
			if joinURL != "" {
				return true, joinURL, entry, nil
			}
		}
	}

	logrus.Debugf("rkecluster %s/%s: initNodeFound was %t and joinURL is empty", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName, initNodeFound)
	// If the current init node has an empty joinURL annotation, we can look to see if there are other init nodes that are more suitable
	if initNodeFound {
		// if the init node was found but doesn't have a joinURL, let's see if there is possible a more suitable init node.
		possibleInitNodes := collect(plan, canBeInitNode)
		for _, entry := range possibleInitNodes {
			if entry.Metadata.Annotations[capr.JoinURLAnnotation] != "" {
				// if a non-blank JoinURL was found, return that we found an init node but with an error
				return true, "", initNode, fmt.Errorf("non-populated init node found, but more suitable alternative is available")
			}
		}
		// if we got through all possibleInitNodes (or there weren't any other possible init nodes), return true that we found an init node with no error.
		logrus.Debugf("rkecluster %s/%s: init node with empty JoinURLAnnotation was found, no suitable alternatives exist", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName)
		return true, "", initNode, nil
	}

	return false, "", nil, fmt.Errorf("init node not found")
}

// electInitNode returns a joinURL and error (if one exists) of an init node. It will first search to see if an init node exists
// (using findInitNode), then will perform a re-election of the most suitable init node (one with a joinURL) and fall back to simply
// electing the first possible init node if no fully populated init node is found.
func (p *Planner) electInitNode(rkeControlPlane *rkev1.RKEControlPlane, plan *plan.Plan, allowReelection bool) (string, error) {
	logrus.Debugf("rkecluster %s/%s: determining if election of init node is necessary", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName)
	if initNodeFound, joinURL, _, err := p.findInitNode(rkeControlPlane, plan); (initNodeFound && err == nil) || errors.Is(err, generic.ErrSkip) {
		logrus.Debugf("rkecluster %s/%s: init node was already elected and found with joinURL: %s", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName, joinURL)
		return joinURL, err
	} else if !initNodeFound && rkeControlPlane.Labels[capr.InitNodeMachineIDLabel] != "" {
		return "", errWaitingf("unable to find designated init node matching machine ID %s", rkeControlPlane.Labels[capr.InitNodeMachineIDLabel])
	}
	// If the joinURL (or an errSkip) was not found, re-elect the init node.
	logrus.Debugf("rkecluster %s/%s: performing election of init node", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName)

	// keep track of whether we invalidate our machine cache when we clear init node marks across nodes.
	cachesInvalidated := false
	// clear all etcd init node marks because we are re-electing our init node
	for _, entry := range collect(plan, isInitNode) {
		if !allowReelection {
			return "", errWaitingf("rkecluster %s/%s: re-election of init machine %s/%s disallowed", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName, entry.Machine.Namespace, entry.Machine.Name)
		}
		logrus.Debugf("rkecluster %s/%s: clearing init node mark on machine %s", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName, entry.Machine.Name)
		if err := p.clearInitNodeMark(entry); errors.Is(err, generic.ErrSkip) {
			cachesInvalidated = true
		} else if err != nil {
			return "", err
		}
	}

	if cachesInvalidated {
		return "", generic.ErrSkip
	}

	possibleInitNodes := collect(plan, canBeInitNode)
	// Mark the first init node that has a joinURL as our new init node.
	for _, entry := range possibleInitNodes {
		if joinURL := entry.Metadata.Annotations[capr.JoinURLAnnotation]; joinURL != "" {
			logrus.Debugf("rkecluster %s/%s: found %s as fully suitable init node with joinURL: %s", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName, entry.Machine.Name, joinURL)
			// it is likely that the error returned by `electInitNode` is going to be `generic.ErrSkip`
			return joinURL, p.setInitNodeMark(entry)
		}
	}

	if len(possibleInitNodes) > 0 {
		fallbackInitNode := possibleInitNodes[0]
		logrus.Debugf("rkecluster %s/%s: no fully suitable init node was found, marking %s as init node as fallback", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName, fallbackInitNode.Machine.Name)
		return "", p.setInitNodeMark(fallbackInitNode)
	}

	logrus.Debugf("rkecluster %s/%s: failed to elect init node, no suitable init nodes were found", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName)
	return "", errWaiting("waiting for viable init node")
}

// designateInitNodeByID is used to force-designate an init node in the cluster. This is especially useful for things like
// local etcd snapshot restore, where a snapshot may be contained on a specific node and that node needs to be the node that
// the snapshot is restored on.
func (p *Planner) designateInitNodeByMachineID(rkeControlPlane *rkev1.RKEControlPlane, plan *plan.Plan, machineID string) (string, error) {
	if machineID == "" {
		return "", fmt.Errorf("machineID cannot be empty when designating init node")
	}
	logrus.Debugf("rkecluster %s/%s: ensuring designated init node for machine ID: %s", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName, machineID)
	entries := collect(plan, isEtcd)
	cacheInvalidated := false
	joinURL := ""
	initNodeFound := false
	for _, entry := range entries {
		if entry.Machine.Labels[capr.MachineIDLabel] == machineID {
			// this is our new initNode
			initNodeFound = true
			if err := p.setInitNodeMark(entry); err != nil {
				if errors.Is(err, generic.ErrSkip) {
					cacheInvalidated = true
					continue
				}
				return "", err
			}
			joinURL = entry.Metadata.Annotations[capr.JoinURLAnnotation]
		} else {
			if err := p.clearInitNodeMark(entry); err != nil {
				if errors.Is(err, generic.ErrSkip) {
					cacheInvalidated = true
					continue
				}
				return "", err
			}
		}
	}
	if !initNodeFound {
		return "", fmt.Errorf("rkecluster %s/%s: init node with machine ID %s was not found during designation", rkeControlPlane.Namespace, rkeControlPlane.Name, machineID)
	}
	if cacheInvalidated {
		return joinURL, generic.ErrSkip
	}
	return joinURL, nil
}
