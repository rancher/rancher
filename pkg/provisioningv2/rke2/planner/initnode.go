package planner

import (
	"errors"
	"fmt"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/sirupsen/logrus"
)

// clearInitNodeMark removes the init node label on the given machine and updates the machine directly against the api
// server, effectively immediately demoting it from being an init node
func (p *Planner) clearInitNodeMark(entry *planEntry) error {
	if err := p.store.removePlanSecretLabel(entry, rke2.InitNodeLabel); err != nil {
		return err
	}
	// We've changed state, so let the caches sync up again
	return generic.ErrSkip
}

// setInitNodeMark sets the init node label on the given machine and updates the machine directly against the api
// server. It returns the modified/updated machine object
func (p *Planner) setInitNodeMark(entry *planEntry) error {
	if entry.Metadata.Labels[rke2.InitNodeLabel] == "true" {
		return nil
	}

	entry.Metadata.Labels[rke2.InitNodeLabel] = "true"
	if err := p.store.updatePlanSecretLabelsAndAnnotations(entry); err != nil {
		return err
	}

	// We've changed state, so let the caches sync up again
	return generic.ErrSkip
}

// findAndDesignateFixedInitNode is used for rancherd where an exact machine (determined by labeling the
// rkecontrolplane object) is desired to be the init node
func (p *Planner) findAndDesignateFixedInitNode(rkeControlPlane *rkev1.RKEControlPlane, plan *plan.Plan) (bool, string, error) {
	logrus.Debugf("rkecluster %s/%s: finding and designating fixed init node", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName)
	fixedMachineID := rkeControlPlane.Labels[rke2.InitNodeMachineIDLabel]
	if fixedMachineID == "" {
		return false, "", fmt.Errorf("fixed machine ID label did not exist on rkecontrolplane")
	}
	entries := collect(plan, func(entry *planEntry) bool {
		return entry.Metadata.Labels[rke2.MachineIDLabel] == fixedMachineID
	})
	if len(entries) > 1 {
		return false, "", fmt.Errorf("multiple machines found with identical machine ID label %s=%s", rke2.MachineIDLabel, fixedMachineID)
	} else if len(entries) == 0 {
		return false, "", fmt.Errorf("fixed machine with ID %s not found", fixedMachineID)
	}
	if rkeControlPlane.Labels[rke2.InitNodeMachineIDDoneLabel] == "" {
		logrus.Debugf("rkecluster %s/%s: setting designated init node to fixedMachineID: %s", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName, fixedMachineID)
		allInitNodes := collect(plan, isEtcd)
		// clear all init node marks and return a generic.ErrSkip if we invalidated caches during clearing
		cachesInvalidated := false
		for _, entry := range allInitNodes {
			if entry.Machine.Labels[rke2.MachineIDLabel] == fixedMachineID {
				continue
			}
			err := p.clearInitNodeMark(entry)
			if err != nil && !errors.Is(err, generic.ErrSkip) {
				// if we received a strange error attempting to clear the init node mark
				return false, "", err
			} else if errors.Is(err, generic.ErrSkip) {
				cachesInvalidated = true
			}
		}
		if cachesInvalidated {
			return false, "", generic.ErrSkip
		}
		if err := p.setInitNodeMark(entries[0]); err != nil && !errors.Is(err, generic.ErrSkip) {
			return false, "", err
		}
		rkeControlPlane = rkeControlPlane.DeepCopy()
		rkeControlPlane.Labels[rke2.InitNodeMachineIDDoneLabel] = "true"
		_, err := p.rkeControlPlanes.Update(rkeControlPlane)
		if err != nil {
			return false, "", err
		}
		// if we set the designated init node on this iteration, return an errSkip so we know our cache is invalidated
		return true, entries[0].Metadata.Annotations[rke2.JoinURLAnnotation], generic.ErrSkip
	}
	logrus.Debugf("rkecluster %s/%s: designated init node %s found", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName, fixedMachineID)
	return true, entries[0].Metadata.Annotations[rke2.JoinURLAnnotation], nil
}

// findInitNode searches the given cluster for the init node. It returns a bool which is whether an init node was
// found, the init node join URL, and an error for a few conditions, i.e. if multiple init nodes were found or if there
// is a more suitable init node. Notably, if multiple init nodes are found, it will return false as it could not come to
// consensus on a single init node
func (p *Planner) findInitNode(rkeControlPlane *rkev1.RKEControlPlane, plan *plan.Plan) (bool, string, error) {
	logrus.Debugf("rkecluster %s/%s searching for init node", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName)
	// if the rkecontrolplane object has an InitNodeMachineID label, we need to find the fixedInitNode.
	if rkeControlPlane.Labels[rke2.InitNodeMachineIDLabel] != "" {
		return p.findAndDesignateFixedInitNode(rkeControlPlane, plan)
	}

	joinURL := ""
	currentInitNodes := collect(plan, isInitNode)

	if len(currentInitNodes) > 1 {
		// if multiple init nodes are found, we don't know which one to return so return false with an error to hopefully trigger a re-election
		return false, "", fmt.Errorf("multiple init nodes found")
	}

	initNodeFound := false

	// this loop should never execute more than once
	for _, entry := range currentInitNodes {
		if canBeInitNode(entry) {
			initNodeFound = true
			joinURL = entry.Metadata.Annotations[rke2.JoinURLAnnotation]
			logrus.Debugf("rkecluster %s/%s found current init node %s with joinURL: %s", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName, entry.Machine.Name, joinURL)
			if joinURL != "" {
				return true, joinURL, nil
			}
		}
	}

	logrus.Debugf("rkecluster %s/%s: initNodeFound was %t and joinURL is empty", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName, initNodeFound)
	// If the current init node has an empty joinURL annotation, we can look to see if there are other init nodes that are more suitable
	if initNodeFound {
		// if the init node was found but doesn't have a joinURL, let's see if there is possible a more suitable init node.
		possibleInitNodes := collect(plan, canBeInitNode)
		for _, entry := range possibleInitNodes {
			if entry.Metadata.Annotations[rke2.JoinURLAnnotation] != "" {
				// if a non-blank JoinURL was found, return that we found an init node but with an error
				return true, "", fmt.Errorf("non-populated init node found, but more suitable alternative is available")
			}
		}
		// if we got through all possibleInitNodes (or there weren't any other possible init nodes), return true that we found an init node with no error.
		logrus.Debugf("rkecluster %s/%s: init node with empty JoinURLAnnotation was found, no suitable alternatives exist", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName)
		return true, "", nil
	}

	return false, "", fmt.Errorf("init node not found")
}

// electInitNode returns a joinURL and error (if one exists) of an init node. It will first search to see if an init node exists
// (using findInitNode), then will perform a re-election of the most suitable init node (one with a joinURL) and fall back to simply
// electing the first possible init node if no fully populated init node is found.
func (p *Planner) electInitNode(rkeControlPlane *rkev1.RKEControlPlane, plan *plan.Plan) (string, error) {
	logrus.Debugf("rkecluster %s/%s: determining if election of init node is necessary", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName)
	initNodeFound, joinURL, err := p.findInitNode(rkeControlPlane, plan)
	if (initNodeFound && err == nil) || errors.Is(err, generic.ErrSkip) {
		logrus.Debugf("rkecluster %s/%s: init node was already elected and found with joinURL: %s", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName, joinURL)
		return joinURL, err
	}
	// If the joinURL (or an errSkip) was not found, re-elect the init node.
	logrus.Debugf("rkecluster %s/%s: performing election of init node", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName)

	possibleInitNodes := collect(plan, canBeInitNode)
	if len(possibleInitNodes) == 0 {
		logrus.Debugf("[planner] rkecluster %s/%s: no possible init nodes exist", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName)
		return joinURL, nil
	}

	// keep track of whether we invalidate our machine cache when we clear init node marks across nodes.
	cachesInvalidated := false
	// clear all etcd init node marks because we are re-electing our init node
	etcdEntries := collect(plan, isEtcd)
	for _, entry := range etcdEntries {
		// Ignore all etcd nodes that are not init nodes
		if !isInitNode(entry) {
			continue
		}
		logrus.Debugf("rkecluster %s/%s: clearing init node mark on machine %s", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName, entry.Machine.Name)
		if err := p.clearInitNodeMark(entry); err != nil && !errors.Is(err, generic.ErrSkip) {
			return "", err
		} else if errors.Is(err, generic.ErrSkip) {
			cachesInvalidated = true
		}
	}

	if cachesInvalidated {
		return "", generic.ErrSkip
	}

	var fallbackInitNode *planEntry
	fallbackInitNodeSet := false

	// Mark the first init node that has a joinURL as our new init node.
	for _, entry := range possibleInitNodes {
		if !fallbackInitNodeSet {
			// set the falbackInitNode to the first possible init node we encounter
			fallbackInitNode = entry
			fallbackInitNodeSet = true
		}
		if entry.Metadata.Annotations[rke2.JoinURLAnnotation] != "" {
			logrus.Debugf("rkecluster %s/%s: found %s as fully suitable init node with joinURL: %s", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName, entry.Machine.Name, entry.Metadata.Annotations[rke2.JoinURLAnnotation])
			// it is likely that the error returned by `electInitNode` is going to be `generic.ErrSkip`
			return entry.Metadata.Annotations[rke2.JoinURLAnnotation], p.setInitNodeMark(entry)
		}
	}

	if fallbackInitNodeSet {
		logrus.Debugf("rkecluster %s/%s: no fully suitable init node was found, marking %s as init node as fallback", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName, fallbackInitNode.Machine.Name)
		return "", p.setInitNodeMark(fallbackInitNode)
	}

	logrus.Debugf("rkecluster %s/%s: failed to elect init node, no suitable init nodes were found", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName)
	return "", ErrWaiting("waiting for possible init node")
}

// designateInitNode is used to force-designate an init node in the cluster. This is especially useful for things like
// local etcd snapshot restore, where a snapshot may be contained on a specific node and that node needs to be the node that
// the snapshot is restored on.
func (p *Planner) designateInitNode(rkeControlPlane *rkev1.RKEControlPlane, plan *plan.Plan, nodeName string) (string, error) {
	logrus.Infof("rkecluster %s/%s: ensuring designated init node: %s", rkeControlPlane.Namespace, rkeControlPlane.Spec.ClusterName, nodeName)
	entries := collect(plan, isEtcd)
	cacheInvalidated := false
	joinURL := ""
	initNodeFound := false
	for _, entry := range entries {
		if entry.Machine.Status.NodeRef != nil &&
			entry.Machine.Status.NodeRef.Name == nodeName {
			// this is our new initNode
			initNodeFound = true
			if err := p.setInitNodeMark(entry); err != nil {
				if errors.Is(err, generic.ErrSkip) {
					cacheInvalidated = true
					continue
				}
				return "", err
			}
			joinURL = entry.Metadata.Annotations[rke2.JoinURLAnnotation]
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
		return "", fmt.Errorf("rkecluster %s/%s: init node %s was not found during designation", rkeControlPlane.Namespace, rkeControlPlane.Name, nodeName)
	}
	if cacheInvalidated {
		return joinURL, generic.ErrSkip
	}
	return joinURL, nil
}
