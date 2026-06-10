package machineroletaint

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/pkg/capr"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta2"
	rkecontrollers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	corew "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

const (
	controllerName = "machine-role-taint"
	workerLabel    = "node-role.kubernetes.io/worker"
)

type handler struct {
	clusterName          string
	nodeClient           corew.NodeClient
	nodeCache            corew.NodeCache
	machineCache         capicontrollers.MachineCache
	capiClusterCache     capicontrollers.ClusterCache
	rkeControlPlaneCache rkecontrollers.RKEControlPlaneCache
}

// Register sets up the machine role taint controller.
// This controller watches CAPI Machine objects from the management cluster
// and reconciles node taints and worker labels in the downstream cluster
// when machine roles change.
func Register(ctx context.Context, userContext *config.UserContext, capi *wrangler.CAPIContext) {
	if capi == nil {
		// Skip if CAPI not available (e.g., local cluster before bootstrap)
		logrus.Debugf("[%s] skipping registration: CAPI context not available", controllerName)
		return
	}

	h := &handler{
		clusterName:          userContext.ClusterName,
		nodeClient:           userContext.Corew.Node(),
		nodeCache:            userContext.Corew.Node().Cache(),
		machineCache:         capi.CAPI.Machine().Cache(),
		capiClusterCache:     capi.CAPI.Cluster().Cache(),
		rkeControlPlaneCache: userContext.Management.Wrangler.RKE.RKEControlPlane().Cache(),
	}

	// Watch all CAPI machines but filter to this cluster in the handler
	capi.CAPI.Machine().OnChange(ctx, controllerName, h.OnMachineChange)

	logrus.Infof("[%s] registered for cluster %s", controllerName, userContext.ClusterName)
}

// OnMachineChange is called when a Machine object changes.
// It reconciles node taints and worker labels based on the machine's role labels.
func (h *handler) OnMachineChange(key string, machine *capi.Machine) (*capi.Machine, error) {
	if machine == nil || machine.DeletionTimestamp != nil {
		return machine, nil
	}

	// CRITICAL: Filter to only machines for THIS downstream cluster
	clusterName := machine.Labels[capi.ClusterNameLabel]
	if clusterName != h.clusterName {
		// This machine belongs to a different cluster, skip
		return machine, nil
	}

	// Check infrastructure ready before checking NodeRef
	if !capr.InfrastructureReady.IsTrue(machine) {
		logrus.Tracef("[%s] machine %s/%s: infrastructure not ready, skipping",
			controllerName, machine.Namespace, machine.Name)
		return machine, nil
	}

	// Check NodeRef exists
	if !machine.Status.NodeRef.IsDefined() {
		// Node not yet registered - this is normal during provisioning
		logrus.Tracef("[%s] machine %s/%s: node not yet registered, skipping",
			controllerName, machine.Namespace, machine.Name)
		return machine, nil
	}

	// Get the node from downstream cluster
	nodeName := machine.Status.NodeRef.Name
	node, err := h.nodeCache.Get(nodeName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Node deleted or not yet created
			logrus.Tracef("[%s] machine %s/%s: node %s not found in downstream cluster",
				controllerName, machine.Namespace, machine.Name, nodeName)
			return machine, nil
		}
		return machine, fmt.Errorf("failed to get node %s: %w", nodeName, err)
	}

	// Get runtime from RKEControlPlane
	runtime, err := h.getRuntime(machine)
	if err != nil {
		return machine, fmt.Errorf("failed to get runtime: %w", err)
	}

	// Reconcile node metadata (taints and worker label)
	if err := h.reconcileNodeMetadata(machine, node, runtime); err != nil {
		return machine, fmt.Errorf("failed to reconcile node metadata: %w", err)
	}

	return machine, nil
}

// reconcileNodeMetadata reconciles both taints and worker label on the node.
func (h *handler) reconcileNodeMetadata(machine *capi.Machine, node *corev1.Node, runtime string) error {
	hasWorkerRole := machine.Labels[capr.WorkerRoleLabel] == "true"
	needsUpdate := false
	newNode := node.DeepCopy()

	// 1. Reconcile taints
	expectedTaints := capr.GetExpectedDefaultTaints(machine, runtime)
	taintsToAdd, taintsToRemoveIndices, taintsNeedUpdate := h.taintsNeedUpdate(node.Spec.Taints, expectedTaints)
	if taintsNeedUpdate {
		newNode.Spec.Taints = h.applyTaintChanges(node.Spec.Taints, taintsToAdd, taintsToRemoveIndices)
		needsUpdate = true
		logrus.Infof("[%s] machine %s/%s: updating taints on node %s (add: %d, remove: %d)",
			controllerName, machine.Namespace, machine.Name, node.Name, len(taintsToAdd), len(taintsToRemoveIndices))
	}

	// 2. Reconcile worker label
	if newNode.Labels == nil {
		newNode.Labels = make(map[string]string)
	}
	hasWorkerLabel := newNode.Labels[workerLabel] == "true"

	if hasWorkerRole && !hasWorkerLabel {
		// Add worker label
		newNode.Labels[workerLabel] = "true"
		needsUpdate = true
		logrus.Infof("[%s] machine %s/%s: adding worker label to node %s",
			controllerName, machine.Namespace, machine.Name, node.Name)
	} else if !hasWorkerRole && hasWorkerLabel {
		// Remove worker label
		delete(newNode.Labels, workerLabel)
		needsUpdate = true
		logrus.Infof("[%s] machine %s/%s: removing worker label from node %s",
			controllerName, machine.Namespace, machine.Name, node.Name)
	}

	if !needsUpdate {
		return nil
	}

	_, err := h.nodeClient.Update(newNode)
	if err != nil {
		return fmt.Errorf("failed to update node %s: %w", node.Name, err)
	}

	logrus.Infof("[%s] machine %s/%s: successfully updated node %s",
		controllerName, machine.Namespace, machine.Name, node.Name)
	return nil
}

// taintsNeedUpdate compares expected vs actual default taints.
// Returns: taints to add, indices to remove, and whether update is needed.
// Only considers default taints - user-defined taints are preserved.
func (h *handler) taintsNeedUpdate(nodeTaints []corev1.Taint, expectedTaints []corev1.Taint) ([]corev1.Taint, []int, bool) {
	var toAdd []corev1.Taint
	var toRemoveIndices []int

	// Build map of expected default taints for quick lookup
	expectedMap := make(map[string]corev1.Taint)
	for _, taint := range expectedTaints {
		key := fmt.Sprintf("%s:%s", taint.Key, taint.Effect)
		expectedMap[key] = taint
	}

	// Find default taints that should be removed
	for i, taint := range nodeTaints {
		if !capr.IsDefaultTaint(taint) {
			// Not a default taint, preserve it
			continue
		}

		key := fmt.Sprintf("%s:%s", taint.Key, taint.Effect)
		if _, shouldExist := expectedMap[key]; !shouldExist {
			// This default taint should be removed
			toRemoveIndices = append(toRemoveIndices, i)
		} else {
			// This taint exists and should exist, remove from expected map
			delete(expectedMap, key)
		}
	}

	// Remaining taints in expectedMap need to be added
	for _, taint := range expectedMap {
		toAdd = append(toAdd, taint)
	}

	needsUpdate := len(toAdd) > 0 || len(toRemoveIndices) > 0
	return toAdd, toRemoveIndices, needsUpdate
}

// applyTaintChanges applies taint additions and removals to create a new taint list.
func (h *handler) applyTaintChanges(currentTaints []corev1.Taint, toAdd []corev1.Taint, toRemoveIndices []int) []corev1.Taint {
	// Create a map of indices to remove for quick lookup
	removeMap := make(map[int]bool)
	for _, idx := range toRemoveIndices {
		removeMap[idx] = true
	}

	// Build new taint list excluding removed taints
	newTaints := make([]corev1.Taint, 0, len(currentTaints)+len(toAdd))
	for i, taint := range currentTaints {
		if !removeMap[i] {
			newTaints = append(newTaints, taint)
		}
	}

	// Add new taints
	newTaints = append(newTaints, toAdd...)

	return newTaints
}

// getRuntime retrieves the runtime (K3s/RKE2) for the machine's cluster.
func (h *handler) getRuntime(machine *capi.Machine) (string, error) {
	// Get cluster name from machine label
	clusterName := machine.Labels[capi.ClusterNameLabel]
	if clusterName == "" {
		return "", fmt.Errorf("machine %s/%s has no cluster label", machine.Namespace, machine.Name)
	}

	// Get CAPI cluster
	capiCluster, err := h.capiClusterCache.Get(machine.Namespace, clusterName)
	if err != nil {
		return "", fmt.Errorf("failed to get CAPI cluster %s/%s: %w", machine.Namespace, clusterName, err)
	}

	// Verify it's an RKE control plane
	if capiCluster.Spec.ControlPlaneRef.Kind != "RKEControlPlane" {
		return "", fmt.Errorf("cluster %s/%s does not have an RKEControlPlane", machine.Namespace, clusterName)
	}

	// Get RKEControlPlane via ControlPlaneRef
	cp, err := h.rkeControlPlaneCache.Get(machine.Namespace, capiCluster.Spec.ControlPlaneRef.Name)
	if err != nil {
		return "", fmt.Errorf("failed to get RKEControlPlane: %w", err)
	}

	return capr.GetRuntime(cp.Spec.KubernetesVersion), nil
}
