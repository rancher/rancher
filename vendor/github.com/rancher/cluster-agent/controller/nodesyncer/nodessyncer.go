package nodesyncer

import (
	"fmt"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NodeSyncer struct {
	ClusterNodes v3.MachineInterface
	Clusters     v3.ClusterInterface
	clusterName  string
}

func Register(workload *config.ClusterContext) {
	n := &NodeSyncer{
		clusterName:  workload.ClusterName,
		ClusterNodes: workload.Management.Management.Machines(""),
		Clusters:     workload.Management.Management.Clusters(""),
	}

	workload.Core.Nodes("").Controller().AddHandler(n.sync)
}

func (n *NodeSyncer) sync(key string, node *v1.Node) error {
	if node == nil {
		return n.deleteClusterNode(key)
	}
	return n.createOrUpdateClusterNode(node)
}

func (n *NodeSyncer) deleteClusterNode(nodeName string) error {
	clusterNode, err := n.getClusterNode(nodeName)
	if err != nil {
		return err
	}
	logrus.Infof("Deleting cluster node [%s]", nodeName)

	if clusterNode == nil {
		logrus.Infof("ClusterNode [%s] is already deleted")
		return nil
	}
	err = n.ClusterNodes.Delete(clusterNode.ObjectMeta.Name, nil)
	if err != nil {
		return fmt.Errorf("Failed to delete cluster node [%s] %v", nodeName, err)
	}
	logrus.Infof("Deleted cluster node [%s]", nodeName)
	return nil
}

func (n *NodeSyncer) getClusterNode(nodeName string) (*v3.Machine, error) {
	nodes, err := n.ClusterNodes.List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, node := range nodes.Items {
		if node.Status.NodeName == nodeName {
			return &node, nil
		}
	}

	return nil, nil
}

func (n *NodeSyncer) createOrUpdateClusterNode(node *v1.Node) error {
	existing, err := n.getClusterNode(node.Name)
	if err != nil {
		return err
	}
	cluster, err := n.Clusters.Get(n.clusterName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("Failed to get cluster [%s] %v", n.clusterName, err)
	}
	clusterNode := n.convertNodeToClusterNode(node, cluster)

	if cluster.ObjectMeta.DeletionTimestamp != nil {
		return nil
	}
	if existing == nil {
		logrus.Infof("Creating cluster node [%s]", node.Name)
		clusterNode.Status.Requested = make(map[v1.ResourceName]resource.Quantity)
		clusterNode.Status.Limits = make(map[v1.ResourceName]resource.Quantity)
		_, err := n.ClusterNodes.Create(clusterNode)
		if err != nil {
			return fmt.Errorf("Failed to create cluster node [%s] %v", node.Name, err)
		}
		logrus.Infof("Created cluster node [%s]", node.Name)
	} else {
		logrus.Debugf("Updating cluster node [%s]", node.Name)
		clusterNode.ResourceVersion = existing.ResourceVersion
		clusterNode.Name = existing.Name
		clusterNode.Status.Requested = existing.Status.Requested
		clusterNode.Status.Limits = existing.Status.Limits
		_, err := n.ClusterNodes.Update(clusterNode)
		if err != nil {
			return fmt.Errorf("Failed to update cluster node [%s] %v", node.Name, err)
		}
		logrus.Debugf("Updated cluster node [%s]", node.Name)
	}
	return nil
}

func (n *NodeSyncer) convertNodeToClusterNode(node *v1.Node, cluster *v3.Cluster) *v3.Machine {
	if node == nil {
		return nil
	}
	clusterNode := &v3.Machine{
		Spec: v3.MachineSpec{
			NodeSpec: node.Spec,
		},
		Status: v3.MachineStatus{
			NodeStatus: node.Status,
		},
	}
	clusterNode.APIVersion = "management.cattle.io/v3"
	clusterNode.Kind = "Machine"
	clusterNode.Status.ClusterName = n.clusterName
	clusterNode.Status.NodeName = node.Name
	clusterNode.ObjectMeta = metav1.ObjectMeta{
		GenerateName: "machine-",
		Labels:       node.Labels,
		Annotations:  node.Annotations,
	}
	ref := metav1.OwnerReference{
		Name:       n.clusterName,
		UID:        cluster.UID,
		APIVersion: cluster.APIVersion,
		Kind:       cluster.Kind,
	}
	clusterNode.OwnerReferences = append(clusterNode.OwnerReferences, ref)
	return clusterNode
}
