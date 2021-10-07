package machinerole

import (
	"context"

	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/types/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type handler struct {
	clusterName string
	nodes       v1.NodeInterface
	machines    capicontrollers.MachineCache
}

func Register(ctx context.Context, context *config.UserContext) {
	h := handler{
		clusterName: context.ClusterName,
		nodes:       context.Core.Nodes(""),
		machines:    context.Management.Wrangler.CAPI.Machine().Cache(),
	}
	context.Core.Nodes("").Controller().AddHandler(ctx, "machine-worker-label", h.WorkerLabelSync)
}

func (h *handler) WorkerLabelSync(key string, node *corev1.Node) (runtime.Object, error) {
	if node == nil || node.DeletionTimestamp != nil || node.Labels == nil || node.Annotations == nil {
		return node, nil
	}

	if _, ok := node.Labels["node-role.kubernetes.io/worker"]; ok {
		return node, nil
	}
	machineName := node.Annotations["cluster.x-k8s.io/machine"]
	if machineName == "" {
		return node, nil
	}
	machineNS := node.Annotations["cluster.x-k8s.io/cluster-namespace"]
	if machineNS == "" {
		return node, nil
	}
	machine, err := h.machines.Get(machineNS, machineName)
	if err != nil {
		return nil, err
	}
	var nodeCopy *corev1.Node
	if val, exists := machine.Labels["rke.cattle.io/worker-role"]; exists && val == "true" {
		nodeCopy = node.DeepCopy()
		nodeCopy.Labels["node-role.kubernetes.io/worker"] = "true"
	}
	if nodeCopy == nil {
		return node, nil
	}
	return h.nodes.Update(nodeCopy)
}
