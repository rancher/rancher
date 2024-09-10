package machinerole

import (
	"context"

	"github.com/rancher/rancher/pkg/capr"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/types/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

type handler struct {
	clusterName   string
	nodes         v1.NodeInterface
	secretsLister v1.SecretLister
}

func Register(ctx context.Context, context *config.UserContext) {
	h := handler{
		clusterName:   context.ClusterName,
		nodes:         context.Core.Nodes(""),
		secretsLister: context.Management.Core.Secrets("").Controller().Lister(),
	}
	context.Core.Nodes("").Controller().AddHandler(ctx, "machine-worker-label", h.WorkerLabelSync)
}

func (h *handler) WorkerLabelSync(_ string, node *corev1.Node) (runtime.Object, error) {
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

	secrets, err := h.secretsLister.List(machineNS, labels.SelectorFromSet(labels.Set{capr.MachineNameLabel: machineName, capr.WorkerRoleLabel: "true"}))
	if err != nil || len(secrets) == 0 {
		return node, err
	}

	node = node.DeepCopy()
	node.Labels["node-role.kubernetes.io/worker"] = "true"

	return h.nodes.Update(node)
}
