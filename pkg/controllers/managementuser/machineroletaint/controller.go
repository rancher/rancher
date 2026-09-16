package machineroletaint

import (
	"context"
	"fmt"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
	provcluster "github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta2"
	provcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	rkecontrollers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	corew "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

const (
	controllerName      = "machine-role-taint"
	workerLabel         = "node-role.kubernetes.io/worker"
	rkeControlPlaneKind = "RKEControlPlane"
)

type handler struct {
	capiCluster          types.NamespacedName
	nodeClient           corew.NodeClient
	nodeCache            corew.NodeCache
	capiClusterCache     capicontrollers.ClusterCache
	rkeControlPlaneCache rkecontrollers.RKEControlPlaneCache
}

// Register sets up the machine role taint controller.
// This controller watches CAPI Machine objects from the management cluster
// and reconciles node taints and worker labels in the downstream cluster
// when machine roles change.
func Register(ctx context.Context, userContext *config.UserContext, capiCtx *wrangler.CAPIContext, clusterRec *apimgmtv3.Cluster) {
	capiCluster, err := capiClusterRef(clusterRec, userContext.Management.Wrangler.Provisioning.Cluster().Cache())
	if err != nil {
		logrus.Errorf("[%s] not registering for cluster %s: %v", controllerName, clusterRec.Name, err)
		return
	}
	if capiCluster == nil {
		// Nothing to reconcile: this cluster has no CAPI machines backing its nodes.
		logrus.Debugf("[%s] cluster %s is not backed by a CAPI cluster, skipping registration", controllerName, clusterRec.Name)
		return
	}

	h := &handler{
		capiCluster:          *capiCluster,
		nodeClient:           userContext.Corew.Node(),
		nodeCache:            userContext.Corew.Node().Cache(),
		capiClusterCache:     capiCtx.CAPI.Cluster().Cache(),
		rkeControlPlaneCache: userContext.Management.Wrangler.RKE.RKEControlPlane().Cache(),
	}

	// Watch all CAPI machines but filter to this cluster in the handler
	capiCtx.CAPI.Machine().OnChange(ctx, controllerName, h.OnMachineChange)

	logrus.Debugf("[%s] registered for cluster %s (CAPI cluster %s)", controllerName, clusterRec.Name, capiCluster)
}

// capiClusterRef resolves the CAPI Cluster backing the given management cluster. The management
// cluster name (c-m-xxxxx) is never the CAPI cluster name, so it cannot be compared against the
// cluster-name label on a Machine directly.
func capiClusterRef(cluster *apimgmtv3.Cluster, provClusterCache provcontrollers.ClusterCache) (*types.NamespacedName, error) {
	ownerName := cluster.Labels[capr.CAPIClusterOwnerLabel]
	ownerNS := cluster.Labels[capr.CAPIClusterOwnerNSLabel]

	if (ownerName == "") != (ownerNS == "") {
		return nil, fmt.Errorf("mgmt cluster %s carries only one of %s/%s; both must be set for a CAPI-native cluster",
			cluster.Name, capr.CAPIClusterOwnerLabel, capr.CAPIClusterOwnerNSLabel)
	}

	if ownerName != "" {
		return &types.NamespacedName{Namespace: ownerNS, Name: ownerName}, nil
	}

	provClusters, err := provClusterCache.GetByIndex(provcluster.ByCluster, cluster.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get provisioning cluster for %s: %w", cluster.Name, err)
	}
	switch len(provClusters) {
	case 0:
		return nil, nil
	case 1:
		return &types.NamespacedName{Namespace: provClusters[0].Namespace, Name: provClusters[0].Name}, nil
	default:
		return nil, fmt.Errorf("expected 1 provisioning cluster for cluster %s, got %d", cluster.Name, len(provClusters))
	}
}

// OnMachineChange is called when a Machine object changes.
// It reconciles node taints and worker labels based on the machine's role labels.
func (h *handler) OnMachineChange(key string, machine *capi.Machine) (*capi.Machine, error) {
	if machine == nil || machine.DeletionTimestamp != nil {
		return machine, nil
	}

	// CRITICAL: filter to only machines for THIS downstream cluster. This handler is registered
	// once per downstream cluster on the shared management-side Machine controller, so every
	// machine event reaches every cluster's handler.
	if machine.Namespace != h.capiCluster.Namespace || machine.Labels[capi.ClusterNameLabel] != h.capiCluster.Name {
		return machine, nil
	}

	// Only reconcile machines whose cluster is backed by an RKE control plane.
	capiCluster, err := h.capiClusterCache.Get(h.capiCluster.Namespace, h.capiCluster.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// The cluster is gone or not created yet, nothing to reconcile.
			return machine, nil
		}
		return machine, fmt.Errorf("failed to get CAPI cluster %s: %w", h.capiCluster, err)
	}

	if !capiCluster.Spec.ControlPlaneRef.IsDefined() ||
		capiCluster.Spec.ControlPlaneRef.APIGroup != capr.RKEAPIGroup ||
		capiCluster.Spec.ControlPlaneRef.Kind != rkeControlPlaneKind {
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
	cp, err := h.rkeControlPlaneCache.Get(capiCluster.Namespace, capiCluster.Spec.ControlPlaneRef.Name)
	if err != nil {
		return machine, fmt.Errorf("failed to get RKEControlPlane %s/%s: %w",
			capiCluster.Namespace, capiCluster.Spec.ControlPlaneRef.Name, err)
	}
	runtime := capr.GetRuntime(cp.Spec.KubernetesVersion)

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
	workerLabelVal, hasWorkerLabel := newNode.Labels[workerLabel]

	if hasWorkerRole && (!hasWorkerLabel || workerLabelVal != "true") {
		// Add/normalize worker label
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
		expected, shouldExist := expectedMap[key]
		if !shouldExist {
			// This default taint should be removed
			toRemoveIndices = append(toRemoveIndices, i)
		} else if taint.Value != expected.Value {
			// Managed taint exists but its value has drifted from the expected
			// value. Remove the stale taint; the expected taint stays in
			// expectedMap and is re-added below with the correct value.
			toRemoveIndices = append(toRemoveIndices, i)
		} else {
			// This taint exists and matches, remove from expected map
			delete(expectedMap, key)
		}
	}

	// Remaining taints in expectedMap need to be added. Range over the
	// expectedTaints slice (not the map) to preserve a deterministic order
	// consistent with capr.GetExpectedDefaultTaints.
	for _, taint := range expectedTaints {
		key := fmt.Sprintf("%s:%s", taint.Key, taint.Effect)
		if _, ok := expectedMap[key]; ok {
			toAdd = append(toAdd, taint)
		}
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
