package planner

import (
	"errors"
	"fmt"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/sirupsen/logrus"
)

// clearInitNodeMark removes the init node label on the given machine and updates the machine directly against the api
// server, effectively immediately demoting it from being an init node
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

// setInitNodeMark sets the init node label on the given machine and updates the machine directly against the api
// server. It returns the modified/updated machine object
func (p *Planner) setInitNodeMark(entry *planEntry) error {
	if entry.Metadata.Labels[capr.InitNodeLabel] == "true" {
		return nil
	}

	entry.Metadata.Labels[capr.InitNodeLabel] = "true"
	if err := p.store.updatePlanSecretLabelsAndAnnotations(entry); err != nil {
		return err
	}

	// We've changed state, so let the caches sync up again
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
	// this loop should never execute more than once
	for _, entry := range currentInitNodes {
		if canBeInitNode(entry) {
			initNodeFound = true
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
				return true, "", nil, fmt.Errorf("non-populated init node found, but more suitable alternative is available")
			}
		}
		// if we got through all possibleInitNodes (or there weren't any other possible init nodes), return true that we found an init node with no error.
		logrus.Debugf("rkecluster %s/%s: init node with empty JoinURLAnnotation was found, no suitable alternatives exist", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName)
		return true, "", nil, nil
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
			return "", errWaitingf("rkecluster %s/%s: waiting for existing init machine %s/%s to be deleted", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName, entry.Machine.Namespace, entry.Machine.Name)
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
