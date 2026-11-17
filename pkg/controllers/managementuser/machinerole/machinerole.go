package machinerole

import (
	"context"
	"fmt"
	"reflect"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
	provcluster "github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta2"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	provcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	nodepkg "github.com/rancher/rancher/pkg/node"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

type handler struct {
	clusterName      string
	clusterUID       types.UID
	nodes            corecontrollers.NodeClient
	clusterCache     mgmtcontrollers.ClusterCache
	mgmtNodeCache    mgmtcontrollers.NodeCache
	provClusterCache provcontrollers.ClusterCache
	capiClusterCache capicontrollers.ClusterCache
	capiMachineCache capicontrollers.MachineCache
}

func Register(ctx context.Context, userCtx *config.UserContext, capiCtx *wrangler.CAPIContext) {
	h := handler{
		clusterName:      userCtx.ClusterName,
		clusterUID:       userCtx.ClusterUID,
		nodes:            userCtx.Corew.Node(),
		clusterCache:     userCtx.Management.Wrangler.Mgmt.Cluster().Cache(),
		mgmtNodeCache:    userCtx.Management.Wrangler.Mgmt.Node().Cache(),
		provClusterCache: userCtx.Management.Wrangler.Provisioning.Cluster().Cache(),
		capiClusterCache: capiCtx.CAPI.Cluster().Cache(),
		capiMachineCache: capiCtx.CAPI.Machine().Cache(),
	}
	userCtx.Corew.Node().OnChange(ctx, "machine-lifecycle-label", h.ImportedLabelSync)
}

func (h *handler) ImportedLabelSync(_ string, node *corev1.Node) (*corev1.Node, error) {
	if node == nil || node.DeletionTimestamp != nil || node.Labels == nil || node.Annotations == nil {
		return node, nil
	}

	cluster, err := h.clusterCache.Get(h.clusterName)
	if err != nil {
		return node, err
	}
	if !config.MatchesClusterUID(h.clusterUID, cluster) {
		// A cluster created again under the same name is not this controller's.
		return node, nil
	}

	if cluster.Status.Driver != apimgmtv3.ClusterDriverK3s && cluster.Status.Driver != apimgmtv3.ClusterDriverRke2 {
		return node, err
	}

	clusterObj, machineObj, err := h.lifecycleObjectsForNode(cluster, node)
	if err != nil {
		return node, err
	}
	if clusterObj == nil || machineObj == nil {
		// wait for backing objects to be populated
		return node, nil
	}

	newMgmtNode := node.DeepCopy()
	if newMgmtNode.Labels == nil {
		newMgmtNode.Labels = map[string]string{}
	}

	lifecycleLabels, err := planv1alpha1.ObjToClusterLifecycleLabels(clusterObj)
	if err != nil {
		return node, err
	}
	for k, v := range lifecycleLabels {
		newMgmtNode.Labels[k] = v
	}

	lifecycleLabels, err = planv1alpha1.ObjToMachineLifecycleLabels(machineObj)
	if err != nil {
		return node, err
	}
	for k, v := range lifecycleLabels {
		newMgmtNode.Labels[k] = v
	}

	if !reflect.DeepEqual(node.Labels, newMgmtNode.Labels) {
		return h.nodes.Update(newMgmtNode)
	}

	return node, nil
}

// lifecycleObjectsForNode returns the cluster and machine objects whose lifecycle labels should be
// stamped on the downstream node. Three cluster shapes reach this handler (driver rke2/k3s):
//
//   - turtles-imported CAPI clusters — mgmt cluster carries the CAPIClusterOwner labels,
//   - v2prov (Rancher-provisioned) RKE2/K3s — backed by a provisioning.cattle.io Cluster and a CAPI
//     Cluster whose Machines carry a Status.NodeRef,
//   - truly-imported RKE2/K3s — no CAPI Machines back the nodes.
//
// For the first two (CAPI-backed) shapes the stamped objects are the CAPI Cluster + CAPI Machine, so
// the resulting plan.cattle.io/* labels match exactly what the provisioning planner bakes into
// node-label at boot (see pkg/capr/planner/config.go addLabels). This is what lets the planner stop
// folding those labels into node-label on steady-state reconciles — the labels still land on the
// Node here, via the API, without restarting rke2 (SURE-12151 / rancher/rancher#58320). For the
// truly-imported shape there is no CAPI Machine, so we fall back to the mgmt v3 Cluster + mgmt v3
// Node (the pre-existing behaviour).
//
// Returns (nil, nil, nil) if the backing objects have not yet been created (transient state —
// caller will retry on the next informer event).
func (h *handler) lifecycleObjectsForNode(mgmtCluster *apimgmtv3.Cluster, node *corev1.Node) (runtime.Object, runtime.Object, error) {
	capiRef, err := h.capiClusterRef(mgmtCluster)
	if err != nil {
		return nil, nil, err
	}

	if capiRef != nil {
		capiCluster, err := h.capiClusterCache.Get(capiRef.Namespace, capiRef.Name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				// CAPI cluster not populated yet (transient).
				return nil, nil, nil
			}
			return nil, nil, err
		}

		machines, err := h.capiMachineCache.List(capiCluster.Namespace, labels.SelectorFromSet(labels.Set{
			capi.ClusterNameLabel: capiCluster.Name,
		}))
		if err != nil {
			return nil, nil, err
		}

		for _, m := range machines {
			if m.DeletionTimestamp == nil && m.Status.NodeRef.IsDefined() && m.Status.NodeRef.Name == node.Name {
				capiClusterTyped := capiCluster.DeepCopy()
				capiClusterTyped.TypeMeta = metav1.TypeMeta{Kind: "Cluster", APIVersion: capi.GroupVersion.String()}
				machineTyped := m.DeepCopy()
				machineTyped.TypeMeta = metav1.TypeMeta{Kind: "Machine", APIVersion: capi.GroupVersion.String()}
				return capiClusterTyped, machineTyped, nil
			}
		}

		// The cluster is CAPI-backed but no Machine claims this node yet; wait for the NodeRef to be
		// populated rather than falling back to mgmt v3 objects (which would stamp the wrong labels).
		if len(machines) > 0 {
			return nil, nil, nil
		}
		// No CAPI Machines at all: this is a truly-imported cluster that merely has a provisioning
		// Cluster shell. Fall through to the mgmt v3 objects below.
	}

	mgmtNodes, err := h.mgmtNodeCache.List(h.clusterName, labels.SelectorFromSet(labels.Set{
		nodepkg.LabelNodeName: node.Name,
	}))
	if err != nil {
		return nil, nil, err
	}
	if len(mgmtNodes) == 0 {
		return nil, nil, nil
	}
	return mgmtCluster, mgmtNodes[0], nil
}

// capiClusterRef resolves the CAPI Cluster backing the given management cluster. The management
// cluster name (c-m-xxxxx) is never the CAPI cluster name, so for v2prov clusters it is resolved via
// the provisioning.cattle.io Cluster that owns it. Returns nil when the cluster is not backed by a
// CAPI/provisioning cluster (e.g. a truly-imported RKE2/K3s cluster).
func (h *handler) capiClusterRef(mgmtCluster *apimgmtv3.Cluster) (*types.NamespacedName, error) {
	ownerName := mgmtCluster.Labels[capr.CAPIClusterOwnerLabel]
	ownerNS := mgmtCluster.Labels[capr.CAPIClusterOwnerNSLabel]

	if (ownerName == "") != (ownerNS == "") {
		return nil, fmt.Errorf(
			"mgmt cluster %s carries only one of %s/%s; both must be set for a CAPI-native cluster",
			mgmtCluster.Name, capr.CAPIClusterOwnerLabel, capr.CAPIClusterOwnerNSLabel)
	}

	if ownerName != "" {
		return &types.NamespacedName{Namespace: ownerNS, Name: ownerName}, nil
	}

	provClusters, err := h.provClusterCache.GetByIndex(provcluster.ByCluster, mgmtCluster.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get provisioning cluster for %s: %w", mgmtCluster.Name, err)
	}
	switch len(provClusters) {
	case 0:
		return nil, nil
	case 1:
		return &types.NamespacedName{Namespace: provClusters[0].Namespace, Name: provClusters[0].Name}, nil
	default:
		return nil, fmt.Errorf("expected 1 provisioning cluster for cluster %s, got %d", mgmtCluster.Name, len(provClusters))
	}
}
