package windows

import (
	"fmt"

	"github.com/rancher/rancher/pkg/taints"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	nodeTaint = v1.Taint{
		Key:    "cattle.io/os",
		Value:  "linux",
		Effect: v1.TaintEffectNoSchedule,
	}
	HostOSLabels = []labels.Set{
		labels.Set(map[string]string{
			"beta.kubernetes.io/os": "linux",
		}),
		labels.Set(map[string]string{
			"kubernetes.io/os": "linux",
		}),
	}
)

// NodeTaintsController This controller will only run on the cluster with windowsPreferred is true.
// It will add taints to the nodes with label beta.kubernetes.io/os=linux.
type NodeTaintsController struct {
	nodeClient v3.NodeInterface
}

func (n *NodeTaintsController) sync(key string, obj *v3.Node) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return obj, nil
	}
	found := false
	for _, hostOSLabel := range HostOSLabels {
		if hostOSLabel.AsSelector().Matches(labels.Set(obj.Status.NodeLabels)) {
			found = true
			break
		}
	}
	if !found {
		return obj, nil
	}

	// NodeTaints is updating, skip this sync
	if obj.Spec.UpdateTaintsFromAPI != nil {
		return obj, nil
	}

	taintSet := taints.GetTaintSet(obj.Spec.InternalNodeSpec.Taints)
	// taint exists on nodes
	if _, ok := taintSet[taints.GetTaintsString(nodeTaint)]; ok {
		return obj, nil
	}

	newObj := obj.DeepCopy()
	newObj.Spec.DesiredNodeTaints = append(newObj.Spec.DesiredNodeTaints, newObj.Spec.InternalNodeSpec.Taints...)
	newObj.Spec.DesiredNodeTaints = append(newObj.Spec.DesiredNodeTaints, nodeTaint)
	falseValue := false
	newObj.Spec.UpdateTaintsFromAPI = &falseValue
	if _, err := n.nodeClient.Update(newObj); err != nil {
		return nil, fmt.Errorf("failed to update node taints for node %s/%s, error: %s", obj.Namespace, obj.Name, err.Error())
	}
	return obj, nil
}
