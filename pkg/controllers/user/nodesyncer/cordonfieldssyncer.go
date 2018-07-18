package nodesyncer

import (
	"github.com/rancher/norman/types/convert"
	nodehelper "github.com/rancher/rancher/pkg/node"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

func (m *NodesSyncer) syncCordonFields(key string, obj *v3.Node) error {
	if obj == nil || obj.DeletionTimestamp != nil || obj.Spec.DesiredNodeUnschedulable == "" {
		return nil
	}
	node, err := nodehelper.GetNodeForMachine(obj, m.nodeLister)
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
