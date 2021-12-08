package nodesyncer

import (
	"reflect"
	"strings"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"github.com/pkg/errors"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	nodehelper "github.com/rancher/rancher/pkg/node"
	"github.com/rancher/rancher/pkg/taints"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (m *nodesSyncer) syncTaints(key string, obj *v3.Node) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return obj, nil
	}

	if !v32.NodeConditionRegistered.IsTrue(obj) {
		return obj, nil
	}

	if obj.Spec.UpdateTaintsFromAPI == nil {
		return obj, nil
	}
	node, err := nodehelper.GetNodeForMachine(obj, m.nodeLister)
	if err != nil || node == nil {
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
		if _, err := m.nodeClient.Update(newNode); err != nil && !isDuplicate(err) {
			return obj, errors.Wrapf(err, "failed to update corev1.Node %s from v3.Node %s in node taint controller", node.Name, obj.Name)
		} else if isDuplicate(err) {
			// If the node has duplicated taints, we should skip the error and set desired taints to nil and stop trying again.
			// The taints will be duplicated if they have same key and effect in k8s version >= 1.14, and same key only in version k8s <=1.13
			logrus.Errorf("failed to update corev1.Node %s from v3.Node %s in node taint controller, error: %s", node.Name, obj.Name, err.Error())
		} else if !reflect.DeepEqual(newObj.Spec.DesiredNodeTaints, newObj.Spec.InternalNodeSpec.Taints) {
			newObj.Spec.InternalNodeSpec.Taints = taintList
		}
	}
	newObj.Spec.DesiredNodeTaints = nil
	newObj.Spec.UpdateTaintsFromAPI = nil
	return m.machines.Update(newObj)
}

func isDuplicate(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "taints must be unique by key and effect pair")
}
