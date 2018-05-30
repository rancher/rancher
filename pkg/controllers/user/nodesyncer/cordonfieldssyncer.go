package nodesyncer

import (
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/labels"
)

func (m *NodesSyncer) syncCordonFields(key string, obj *v3.Node) error {
	if obj == nil || obj.DeletionTimestamp != nil {
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
	if node.Spec.Unschedulable != obj.Spec.InternalNodeSpec.Unschedulable {
		toUpdate := node.DeepCopy()
		toUpdate.Spec.Unschedulable = obj.Spec.InternalNodeSpec.Unschedulable
		if _, err := m.nodeClient.Update(toUpdate); err != nil {
			return err
		}
	}
	return nil
}
