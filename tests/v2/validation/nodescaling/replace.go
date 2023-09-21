package nodescaling

import (
	"testing"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	nodestat "github.com/rancher/rancher/tests/framework/extensions/nodes"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	namespace                    = "fleet-default"
	ProvisioningSteveResouceType = "provisioning.cattle.io.cluster"
	machineSteveResourceType     = "cluster.x-k8s.io.machine"
	machineSteveAnnotation       = "cluster.x-k8s.io/machine"
	etcdLabel                    = "node-role.kubernetes.io/etcd"
	clusterLabel                 = "cluster.x-k8s.io/cluster-name"
)

func MatchNodeToRole(t *testing.T, client *rancher.Client, clusterID string, isEtcd bool, isControlPlane bool, isWorker bool) (int, *management.Node) {
	machines, err := client.Management.Node.List(&types.ListOpts{Filters: map[string]interface{}{
		"clusterId": clusterID,
	}})
	require.NoError(t, err)
	numOfNodes := 0
	lastMatchingNode := &management.Node{}

	for _, machine := range machines.Data {
		if machine.Etcd == isEtcd && machine.ControlPlane == isControlPlane && machine.Worker == isWorker {
			lastMatchingNode = &machine
			numOfNodes++
		}
	}
	require.NotEmpty(t, lastMatchingNode.NodeName, "matching node name is empty")
	return numOfNodes, lastMatchingNode
}

// ReplaceNodes replaces the last node with the specified role(s) in a k3s/rke2 cluster
func ReplaceNodes(t *testing.T, client *rancher.Client, clusterName string, isEtcd bool, isControlPlane bool, isWorker bool) {
	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	require.NoError(t, err)
	numOfNodesBeforeDeletion, nodeToDelete := MatchNodeToRole(t, client, clusterID, isEtcd, isControlPlane, isWorker)

	machineToDelete, err := client.Steve.SteveType(machineSteveResourceType).ByID("fleet-default/" + nodeToDelete.Annotations[machineSteveAnnotation])
	require.NoError(t, err)

	logrus.Info("Deleting, " + nodeToDelete.Name + " node..")
	err = client.Steve.SteveType(machineSteveResourceType).Delete(machineToDelete)
	require.NoError(t, err)

	err = clusters.WaitClusterToBeUpgraded(client, clusterID)
	require.NoError(t, err)

	err = nodestat.AllMachineReady(client, clusterID)
	require.NoError(t, err)

	isNodeReplaced, err := nodestat.IsNodeReplaced(client, machineToDelete.ID, clusterID, numOfNodesBeforeDeletion, isEtcd, isControlPlane, isWorker)
	require.NoError(t, err)
	require.True(t, isNodeReplaced)
}

// ReplaceRKE1Nodes replaces the last node with the specified role(s) in a rke1 cluster
func ReplaceRKE1Nodes(t *testing.T, client *rancher.Client, clusterName string, isEtcd bool, isControlPlane bool, isWorker bool) {
	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	require.NoError(t, err)
	numOfNodesBeforeDeletion, nodeToDelete := MatchNodeToRole(t, client, clusterID, isEtcd, isControlPlane, isWorker)

	logrus.Info("Deleting, " + nodeToDelete.NodeName + " node..")
	err = client.Management.Node.Delete(nodeToDelete)
	require.NoError(t, err)

	err = clusters.WaitClusterToBeUpgraded(client, clusterID)
	require.NoError(t, err)

	err = nodestat.AllManagementNodeReady(client, clusterID)
	require.NoError(t, err)

	isNodeReplaced, err := nodestat.IsNodeReplaced(client, nodeToDelete.ID, clusterID, numOfNodesBeforeDeletion, isEtcd, isControlPlane, isWorker)
	require.NoError(t, err)

	require.True(t, isNodeReplaced)

	podResults, podErrors := pods.StatusPods(client, clusterID)
	assert.NotEmpty(t, podResults)
	assert.Empty(t, podErrors)
}
