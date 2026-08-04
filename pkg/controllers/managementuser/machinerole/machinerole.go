package machinerole

import (
	"context"
	"fmt"
	"reflect"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta2"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	nodepkg "github.com/rancher/rancher/pkg/node"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

type handler struct {
	clusterName      string
	nodes            corecontrollers.NodeClient
	secretCache      corecontrollers.SecretCache
	clusterCache     mgmtcontrollers.ClusterCache
	mgmtNodeCache    mgmtcontrollers.NodeCache
	capiClusterCache capicontrollers.ClusterCache
	capiMachineCache capicontrollers.MachineCache
}

func Register(ctx context.Context, userCtx *config.UserContext, capiCtx *wrangler.CAPIContext) {
	h := handler{
		clusterName:      userCtx.ClusterName,
		nodes:            userCtx.Corew.Node(),
		secretCache:      userCtx.Management.Wrangler.Core.Secret().Cache(),
		clusterCache:     userCtx.Management.Wrangler.Mgmt.Cluster().Cache(),
		mgmtNodeCache:    userCtx.Management.Wrangler.Mgmt.Node().Cache(),
		capiClusterCache: capiCtx.CAPI.Cluster().Cache(),
		capiMachineCache: capiCtx.CAPI.Machine().Cache(),
	}
	userCtx.Corew.Node().OnChange(ctx, "machine-worker-label", h.WorkerLabelSync)
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
// stamped on the downstream node. For turtles-imported CAPI clusters (mgmt cluster carries the
// CAPIClusterOwner labels), those are the CAPI Cluster + CAPI Machine. Otherwise they are the mgmt
// v3 Cluster + mgmt v3 Node — the pre-existing behaviour for imported RKE2/K3s.
//
// Returns (nil, nil, nil) if the backing objects have not yet been created (transient state —
// caller will retry on the next informer event).
func (h *handler) lifecycleObjectsForNode(mgmtCluster *apimgmtv3.Cluster, node *corev1.Node) (runtime.Object, runtime.Object, error) {
	ownerName := mgmtCluster.Labels[capr.CAPIClusterOwnerLabel]
	ownerNS := mgmtCluster.Labels[capr.CAPIClusterOwnerNSLabel]

	if (ownerName == "") != (ownerNS == "") {
		return nil, nil, fmt.Errorf(
			"mgmt cluster %s carries only one of %s/%s; both must be set for a CAPI-native cluster",
			mgmtCluster.Name, capr.CAPIClusterOwnerLabel, capr.CAPIClusterOwnerNSLabel)
	}

	if ownerName != "" {
		capiCluster, err := h.capiClusterCache.Get(ownerNS, ownerName)
		if err != nil {
			return nil, nil, err
		}
		capiClusterTyped := capiCluster.DeepCopy()
		capiClusterTyped.TypeMeta = metav1.TypeMeta{Kind: "Cluster", APIVersion: capi.GroupVersion.String()}

		machines, err := h.capiMachineCache.List(capiCluster.Namespace, labels.SelectorFromSet(labels.Set{
			capi.ClusterNameLabel: capiCluster.Name,
		}))
		if err != nil {
			return nil, nil, err
		}
		for _, m := range machines {
			if m.Status.NodeRef.IsDefined() && m.Status.NodeRef.Name == node.Name {
				machineTyped := m.DeepCopy()
				machineTyped.TypeMeta = metav1.TypeMeta{Kind: "Machine", APIVersion: capi.GroupVersion.String()}
				return capiClusterTyped, machineTyped, nil
			}
		}
		return nil, nil, nil
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

func (h *handler) WorkerLabelSync(_ string, node *corev1.Node) (*corev1.Node, error) {
	if node == nil || node.DeletionTimestamp != nil || node.Labels == nil || node.Annotations == nil {
		return node, nil
	}

	if _, ok := node.Labels["node-role.kubernetes.io/worker"]; ok {
		return node, nil
	}
	machineName := node.Annotations[capi.MachineAnnotation]
	if machineName == "" {
		return node, nil
	}
	machineNS := node.Annotations[capi.ClusterNamespaceAnnotation]
	if machineNS == "" {
		return node, nil
	}

	secrets, err := h.secretCache.List(machineNS, labels.SelectorFromSet(labels.Set{capr.MachineNameLabel: machineName, capr.WorkerRoleLabel: "true"}))
	if err != nil || len(secrets) == 0 {
		return node, err
	}

	node = node.DeepCopy()
	node.Labels["node-role.kubernetes.io/worker"] = "true"

	return h.nodes.Update(node)
}
