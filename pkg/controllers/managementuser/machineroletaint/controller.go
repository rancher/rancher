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
	machineCache         capicontrollers.MachineCache
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
		machineCache:         capiCtx.CAPI.Machine().Cache(),
		capiClusterCache:     capiCtx.CAPI.Cluster().Cache(),
		rkeControlPlaneCache: userContext.Management.Wrangler.RKE.RKEControlPlane().Cache(),
	}

	// Watch all CAPI machines but filter to this cluster in the handler
	capiCtx.CAPI.Machine().OnChange(ctx, controllerName, h.OnMachineChange)
	userContext.Corew.Node().OnChange(ctx, controllerName, h.OnNodeChange)

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

	runtime, ok, err := h.clusterRuntime()
	if err != nil || !ok {
		return machine, err
	}

	// Get the node from downstream cluster
	nodeName := machine.Status.NodeRef.Name
	node, err := h.nodeCache.Get(nodeName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Tracef("[%s] machine %s/%s: node %s not found in downstream cluster",
				controllerName, machine.Namespace, machine.Name, nodeName)
			return machine, nil
		}
		return machine, fmt.Errorf("failed to get node %s: %w", nodeName, err)
	}

	// Reconcile node metadata (taints and worker label)
	if err := h.reconcileNodeMetadata(machine, node, runtime); err != nil {
		return machine, fmt.Errorf("failed to reconcile node metadata: %w", err)
	}

	return machine, nil
}

// OnNodeChange reconciles a downstream node against the CAPI machine that owns it. It covers the
// case where the node appears (or is re-created) after the machine has already been reconciled.
func (h *handler) OnNodeChange(_ string, node *corev1.Node) (*corev1.Node, error) {
	if node == nil || node.DeletionTimestamp != nil {
		return node, nil
	}

	machineName := node.Annotations[capi.MachineAnnotation]
	machineNS := node.Annotations[capi.ClusterNamespaceAnnotation]
	if machineName == "" || machineNS != h.capiCluster.Namespace {
		// CAPI has not linked this node to a machine yet, or the link points somewhere
		// this handler does not own.
		return node, nil
	}

	machine, err := h.machineCache.Get(machineNS, machineName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return node, nil
		}
		return node, fmt.Errorf("failed to get machine %s/%s: %w", machineNS, machineName, err)
	}

	if machine.DeletionTimestamp != nil ||
		machine.Labels[capi.ClusterNameLabel] != h.capiCluster.Name ||
		!capr.InfrastructureReady.IsTrue(machine) {
		return node, nil
	}

	runtime, ok, err := h.clusterRuntime()
	if err != nil || !ok {
		return node, err
	}

	if err := h.reconcileNodeMetadata(machine, node, runtime); err != nil {
		return node, fmt.Errorf("failed to reconcile node metadata: %w", err)
	}

	return node, nil
}

// clusterRuntime returns the runtime (rke2/k3s) of this cluster. The second return value is false
// when the cluster is not backed by an RKEControlPlane, in which case there is nothing to reconcile.
func (h *handler) clusterRuntime() (string, bool, error) {
	capiCluster, err := h.capiClusterCache.Get(h.capiCluster.Namespace, h.capiCluster.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// The cluster is gone or not created yet, nothing to reconcile.
			return "", false, nil
		}
		return "", false, fmt.Errorf("failed to get CAPI cluster %s: %w", h.capiCluster, err)
	}

	if !capiCluster.Spec.ControlPlaneRef.IsDefined() ||
		capiCluster.Spec.ControlPlaneRef.APIGroup != capr.RKEAPIGroup ||
		capiCluster.Spec.ControlPlaneRef.Kind != rkeControlPlaneKind {
		return "", false, nil
	}

	cp, err := h.rkeControlPlaneCache.Get(capiCluster.Namespace, capiCluster.Spec.ControlPlaneRef.Name)
	if err != nil {
		return "", false, fmt.Errorf("failed to get RKEControlPlane %s/%s: %w",
			capiCluster.Namespace, capiCluster.Spec.ControlPlaneRef.Name, err)
	}

	return capr.GetRuntime(cp.Spec.KubernetesVersion), true, nil
}

// reconcileNodeMetadata reconciles both taints and worker label on the node.
func (h *handler) reconcileNodeMetadata(machine *capi.Machine, node *corev1.Node, runtime string) error {
	hasWorkerRole := machine.Labels[capr.WorkerRoleLabel] == "true"
	needsUpdate := false
	newNode := node.DeepCopy()

	// 1. Reconcile taints
	configured, err := configuredTaintKeys(machine)
	if err != nil {
		return err
	}
	expectedTaints := capr.GetExpectedDefaultTaints(machine, runtime)
	taintsToAdd, taintsToRemoveIndices, taintsNeedUpdate := h.taintsNeedUpdate(node.Spec.Taints, expectedTaints, configured)
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

	if _, err := h.nodeClient.Update(newNode); err != nil {
		return fmt.Errorf("failed to update node %s: %w", node.Name, err)
	}

	logrus.Infof("[%s] machine %s/%s: successfully updated node %s",
		controllerName, machine.Namespace, machine.Name, node.Name)
	return nil
}

// configuredTaintKeys returns the key:effect of every taint the user explicitly configured on the
// machine pool (rke.cattle.io/taints). Those belong to the user even when they collide with one of
// Rancher's default taints, so this controller must not remove or rewrite them.
func configuredTaintKeys(machine *capi.Machine) (map[string]bool, error) {
	taints, err := capr.ParseTaintsAnnotation(machine.Annotations)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s on machine %s/%s: %w",
			capr.TaintsAnnotation, machine.Namespace, machine.Name, err)
	}
	if len(taints) == 0 {
		return nil, nil
	}

	keys := make(map[string]bool, len(taints))
	for _, taint := range taints {
		keys[taintKey(taint)] = true
	}
	return keys, nil
}

func taintKey(taint corev1.Taint) string {
	return fmt.Sprintf("%s:%s", taint.Key, taint.Effect)
}

// taintsNeedUpdate compares expected vs actual default taints.
// Returns: taints to add, indices to remove, and whether update is needed.
func (h *handler) taintsNeedUpdate(nodeTaints []corev1.Taint, expectedTaints []corev1.Taint, configured map[string]bool) ([]corev1.Taint, []int, bool) {
	var toAdd []corev1.Taint
	var toRemoveIndices []int

	// Build map of expected default taints for quick lookup
	expectedMap := make(map[string]corev1.Taint)
	for _, taint := range expectedTaints {
		key := taintKey(taint)
		if configured[key] {
			continue
		}
		expectedMap[key] = taint
	}

	// Find default taints that should be removed
	for i, taint := range nodeTaints {
		key := taintKey(taint)
		if !capr.IsDefaultTaint(taint) || configured[key] {
			// Not an implicitly managed taint, preserve it
			continue
		}

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
		if _, ok := expectedMap[taintKey(taint)]; ok {
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
