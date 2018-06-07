package nodesyncer

import (
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/labels"
)

func (m *NodesSyncer) syncCordonFields(key string, obj *v3.Node) error {
	if obj == nil || obj.DeletionTimestamp != nil || obj.Spec.DesiredNodeUnschedulable == "" {
		return nil
	}
	nodes, err := m.nodeLister.List("", labels.NewSelector())
	if err != nil {
		return err
	}
	node, err := m.getNode(obj, nodes)
	if err != nil {
		return err
	}
	desiredValue := convert.ToBool(obj.Spec.DesiredNodeUnschedulable)
	if node.Spec.Unschedulable != desiredValue {
		toUpdate := node.DeepCopy()
		toUpdate.Spec.Unschedulable = desiredValue
		if _, err := m.nodeClient.Update(toUpdate); err != nil {
			return err
		}
	}
	nodeCopy := obj.DeepCopy()
	nodeCopy.Spec.DesiredNodeUnschedulable = ""
	if _, err := m.machines.Update(nodeCopy); err != nil {
		return err
	}
	return nil
}
