package machinerole

import (
	"context"
	"reflect"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	nodepkg "github.com/rancher/rancher/pkg/node"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/types/config"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

type handler struct {
	clusterName   string
	nodes         corecontrollers.NodeClient
	secretCache   corecontrollers.SecretCache
	clusterCache  mgmtcontrollers.ClusterCache
	mgmtNodeCache mgmtcontrollers.NodeCache
}

func Register(ctx context.Context, context *config.UserContext) {
	h := handler{
		clusterName:   context.ClusterName,
		nodes:         context.Corew.Node(),
		secretCache:   context.Management.Wrangler.Core.Secret().Cache(),
		clusterCache:  context.Management.Wrangler.Mgmt.Cluster().Cache(),
		mgmtNodeCache: context.Management.Wrangler.Mgmt.Node().Cache(),
	}
	context.Corew.Node().OnChange(ctx, "machine-worker-label", h.WorkerLabelSync)
	context.Corew.Node().OnChange(ctx, "machine-lifecycle-label", h.ImportedLabelSync)
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

	mgmtNode, err := h.mgmtNodeCache.List(h.clusterName, labels.SelectorFromSet(labels.Set{
		nodepkg.LabelNodeName: node.Name,
	}))
	if err != nil {
		return node, err
	}

	if len(mgmtNode) == 0 {
		// wait for label to be populated
		return node, nil
	}

	newMgmtNode := node.DeepCopy()
	if newMgmtNode.Labels == nil {
		newMgmtNode.Labels = map[string]string{}
	}

	lifecycleLabels, err := planv1alpha1.ObjToClusterLifecycleLabels(cluster)
	if err != nil {
		return node, err
	}

	for k, v := range lifecycleLabels {
		newMgmtNode.Labels[k] = v
	}

	lifecycleLabels, err = planv1alpha1.ObjToMachineLifecycleLabels(mgmtNode[0])
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
