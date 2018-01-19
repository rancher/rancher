package nodesyncer

import (
	"fmt"

	"reflect"

	"github.com/pkg/errors"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type NodeSyncer struct {
	machinesClient   v3.MachineInterface
	machines         v3.MachineLister
	clusters         v3.ClusterLister
	clusterName      string
	clusterNamespace string
}

func Register(cluster *config.ClusterContext) {
	n := &NodeSyncer{
		clusterName:      cluster.ClusterName,
		clusterNamespace: cluster.ClusterName,
		machinesClient:   cluster.Management.Management.Machines(cluster.ClusterName),
		machines:         cluster.Management.Management.Machines(cluster.ClusterName).Controller().Lister(),
		clusters:         cluster.Management.Management.Clusters("").Controller().Lister(),
	}
	cluster.Core.Nodes("").AddLifecycle("nodesSyncer", n)
}

func (n *NodeSyncer) Remove(node *corev1.Node) (*corev1.Node, error) {
	machine, err := n.getMachine(node.Name)
	if err != nil {
		return nil, err
	}
	logrus.Infof("Deleting cluster node [%s]", node.Name)

	if machine == nil {
		logrus.Debugf("Cluster node [%s] is already deleted")
		return nil, nil
	}
	err = n.machinesClient.Delete(machine.ObjectMeta.Name, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to delete cluster node [%s]", node.Name)
	}
	logrus.Infof("Deleted cluster node [%s]", node.Name)
	return nil, nil
}

func (n *NodeSyncer) getMachine(nodeName string) (*v3.Machine, error) {
	machines, err := n.machines.List(n.clusterNamespace, labels.NewSelector())
	if err != nil {
		return nil, err
	}
	for _, machine := range machines {
		if machine.Status.NodeName == nodeName {
			return machine, nil
		}
		// to handle the case when machine was provisioned first
		if machine.Status.NodeConfig != nil {
			if machine.Status.NodeConfig.HostnameOverride == nodeName {
				return machine, nil
			}
		}
	}

	return nil, nil
}

func resetConditions(machine *v3.Machine) *v3.Machine {
	if machine.Status.NodeStatus.Conditions == nil {
		return machine
	}
	updated := machine.DeepCopy()
	var toUpdateConds []corev1.NodeCondition
	for _, cond := range machine.Status.NodeStatus.Conditions {
		toUpdateCond := cond.DeepCopy()
		toUpdateCond.LastHeartbeatTime = metav1.Time{}
		toUpdateCond.LastTransitionTime = metav1.Time{}
		toUpdateConds = append(toUpdateConds, *toUpdateCond)
	}
	updated.Status.NodeStatus.Conditions = toUpdateConds
	return updated
}

func (n *NodeSyncer) Updated(node *corev1.Node) (*corev1.Node, error) {
	existing, err := n.getMachine(node.Name)
	if err != nil || existing == nil {
		return nil, err
	}
	toUpdate, err := n.convertNodeToMachine(node, existing)
	if err != nil {
		return nil, err
	}
	// update only when nothing changed
	if objectsAreEqual(existing, toUpdate) {
		return nil, nil
	}
	logrus.Debugf("Updating cluster node [%s]", node.Name)
	_, err = n.machinesClient.Update(toUpdate)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to update cluster node [%s]", node.Name)
	}
	logrus.Debugf("Updated cluster node [%s]", node.Name)
	return nil, nil
}

func objectsAreEqual(existing *v3.Machine, toUpdate *v3.Machine) bool {
	// we are updating spec and status only, so compare them
	toUpdateToCompare := resetConditions(toUpdate)
	existingToCompare := resetConditions(existing)
	statusEqual := reflect.DeepEqual(toUpdateToCompare.Status.NodeStatus, existingToCompare.Status.NodeStatus)
	labelsEqual := reflect.DeepEqual(toUpdateToCompare.Status.NodeLabels, existing.Status.NodeLabels)
	annotationsEqual := reflect.DeepEqual(toUpdateToCompare.Status.NodeAnnotations, existing.Status.NodeAnnotations)
	specEqual := reflect.DeepEqual(toUpdateToCompare.Spec.NodeSpec, existingToCompare.Spec.NodeSpec)
	nodeNameEqual := toUpdateToCompare.Status.NodeName == existingToCompare.Status.NodeName
	return statusEqual && specEqual && nodeNameEqual && labelsEqual && annotationsEqual
}

func (n *NodeSyncer) convertNodeToMachine(node *corev1.Node, existing *v3.Machine) (*v3.Machine, error) {
	cluster, err := n.clusters.Get("", n.clusterName)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get cluster [%s]", n.clusterName)
	}
	if cluster.ObjectMeta.DeletionTimestamp != nil {
		return nil, fmt.Errorf("Failed to find cluster [%s]", n.clusterName)
	}

	var machine *v3.Machine
	if existing == nil {
		machine = &v3.Machine{
			Spec:   v3.MachineSpec{},
			Status: v3.MachineStatus{},
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "machine-"},
		}
		machine.Namespace = n.clusterNamespace
		machine.Status.Requested = make(map[corev1.ResourceName]resource.Quantity)
		machine.Status.Limits = make(map[corev1.ResourceName]resource.Quantity)
		machine.Spec.NodeSpec = *node.Spec.DeepCopy()
		machine.Status.NodeStatus = *node.Status.DeepCopy()
	} else {
		machine = existing.DeepCopy()
		machine.Spec.NodeSpec = *node.Spec.DeepCopy()
		machine.Status.NodeStatus = *node.Status.DeepCopy()
		machine.Status.Requested = existing.Status.Requested
		machine.Status.Limits = existing.Status.Limits
	}

	machine.Status.NodeAnnotations = node.Annotations
	machine.Status.NodeLabels = node.Labels
	machine.Status.NodeName = node.Name
	machine.APIVersion = "management.cattle.io/v3"
	machine.Kind = "Machine"
	return machine, nil
}

func (n *NodeSyncer) Create(node *corev1.Node) (*corev1.Node, error) {
	existing, err := n.getMachine(node.Name)
	if err != nil || existing != nil {
		return nil, err
	}
	machine, err := n.convertNodeToMachine(node, nil)
	if err != nil {
		return nil, err
	}

	logrus.Infof("Creating cluster node [%s]", node.Name)
	_, err = n.machinesClient.Create(machine)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to create cluster node [%s]", node.Name)
	}
	logrus.Infof("Created cluster node [%s]", node.Name)
	return nil, nil
}
