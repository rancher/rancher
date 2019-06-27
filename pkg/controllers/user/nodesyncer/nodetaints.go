package nodesyncer

import (
	"reflect"

	"github.com/pkg/errors"
	nodehelper "github.com/rancher/rancher/pkg/node"
	"github.com/rancher/rancher/pkg/taints"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (n *nodesSyncer) syncTaints(key string, obj *v3.Node) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return obj, nil
	}

	if !v3.NodeConditionRegistered.IsTrue(obj) {
		return obj, nil
	}

	if obj.Spec.UpdateTaintsFromAPI == nil {
		return obj, nil
	}
	node, err := nodehelper.GetNodeForMachine(obj, n.nodeLister)
	if err != nil {
		return obj, err
	}
	toAdd, toDel := taints.GetToDiffTaints(node.Spec.Taints, obj.Spec.DesiredNodeTaints)
	newObj := obj.DeepCopy()
	if len(toAdd) != 0 || len(toDel) != 0 {
		newNode := node.DeepCopy()
		var taintList []corev1.Taint
		for index, taint := range newNode.Spec.Taints {
			if _, ok := toDel[index]; !ok {
				taintList = append(taintList, taint)
			}
		}
		for _, taintStr := range toAdd {
			taintList = append(taintList, taintStr)
		}
		newNode.Spec.Taints = taintList
		if _, err := n.nodeClient.Update(newNode); err != nil {
			return obj, errors.Wrapf(err, "failed to update corev1.Node %s from v3.Node %s in node taint controller", node.Name, obj.Name)
		}
		if !reflect.DeepEqual(newObj.Spec.DesiredNodeTaints, newObj.Spec.InternalNodeSpec.Taints) {
			newObj.Spec.InternalNodeSpec.Taints = taintList
		}
	}
	newObj.Spec.DesiredNodeTaints = nil
	newObj.Spec.UpdateTaintsFromAPI = nil
	return n.machines.Update(newObj)
}
