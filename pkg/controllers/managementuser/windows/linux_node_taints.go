package windows

import (
	"fmt"

	"github.com/rancher/rancher/pkg/taints"
	corew "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	nodeTaint = v1.Taint{
		Key:    "cattle.io/os",
		Value:  "linux",
		Effect: v1.TaintEffectNoSchedule,
	}
	HostOSLabels = []labels.Set{
		labels.Set(map[string]string{
			"kubernetes.io/os": "linux",
		}),
	}
)

// NodeTaintsController This controller will only run on the cluster with windowsPreferred is true.
// It will add taints to the v1.Node.Spec.Taints to the nodes with label kubernetes.io/os=linux.
type NodeTaintsController struct {
	nodeClient corew.NodeClient
}

func (n *NodeTaintsController) sync(_ string, obj *v1.Node) (*v1.Node, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return obj, nil
	}
	found := false
	for _, hostOSLabel := range HostOSLabels {
		if hostOSLabel.AsSelector().Matches(labels.Set(obj.Labels)) {
			found = true
			break
		}
	}
	if !found {
		return obj, nil
	}

	taintSet := taints.GetTaintSet(obj.Spec.Taints)
	// taint exists on nodes
	if _, ok := taintSet[taints.GetTaintsString(nodeTaint)]; ok {
		return obj, nil
	}

	newObj := obj.DeepCopy()
	newObj.Spec.Taints = append(newObj.Spec.Taints, nodeTaint)
	if _, err := n.nodeClient.Update(newObj); err != nil {
		return nil, fmt.Errorf("failed to update node taints for node %s/%s, error: %s", obj.Namespace, obj.Name, err.Error())
	}
	return obj, nil
}
