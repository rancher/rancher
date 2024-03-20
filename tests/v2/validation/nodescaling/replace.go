package nodescaling

import (
	"testing"

	"github.com/rancher/norman/types"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/defaults"
	nodestat "github.com/rancher/shepherd/extensions/nodes"
	"github.com/rancher/shepherd/extensions/workloads/pods"
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

func MatchNodeToRole(t *testing.T, client *rancher.Client, clusterID string, isEtcd bool, isControlPlane bool, isWorker bool) (int, []management.Node) {
	machines, err := client.Management.Node.List(&types.ListOpts{Filters: map[string]interface{}{
		"clusterId": clusterID,
	}})
	require.NoError(t, err)

	numOfNodes := 0
	matchingNodes := []management.Node{}

	for _, machine := range machines.Data {
		if machine.Etcd == isEtcd && machine.ControlPlane == isControlPlane && machine.Worker == isWorker {
			matchingNodes = append(matchingNodes, machine)
			numOfNodes++
		}
	}
	require.NotEmpty(t, matchingNodes, "matching node name is empty")

	return numOfNodes, matchingNodes
}

// ReplaceNodes replaces the last node with the specified role(s) in a k3s/rke2 cluster
func ReplaceNodes(t *testing.T, client *rancher.Client, clusterName string, isEtcd bool, isControlPlane bool, isWorker bool) {
	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	require.NoError(t, err)
	numOfNodesBeforeDeletion, nodesToDelete := MatchNodeToRole(t, client, clusterID, isEtcd, isControlPlane, isWorker)

	for i := range nodesToDelete {
		machineToDelete, err := client.Steve.SteveType(machineSteveResourceType).ByID("fleet-default/" + nodesToDelete[i].Annotations[machineSteveAnnotation])
		require.NoError(t, err)

		logrus.Infof("Replacing node: " + nodesToDelete[i].NodeName)
		err = client.Steve.SteveType(machineSteveResourceType).Delete(machineToDelete)
		require.NoError(t, err)

		err = clusters.WaitClusterToBeUpgraded(client, clusterID)
		require.NoError(t, err)

		logrus.Infof("Checking if node %s is replaced", nodesToDelete[i].NodeName)
		_, err = nodestat.IsNodeReplaced(client, machineToDelete.ID, clusterID, numOfNodesBeforeDeletion)
		require.NoError(t, err)

		podErrors := pods.StatusPods(client, clusterID)
		assert.Empty(t, podErrors)
	}
}

// ReplaceRKE1Nodes replaces the last node with the specified role(s) in a rke1 cluster
func ReplaceRKE1Nodes(t *testing.T, client *rancher.Client, clusterName string, isEtcd bool, isControlPlane bool, isWorker bool) {
	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	require.NoError(t, err)
	numOfNodesBeforeDeletion, nodeToDelete := MatchNodeToRole(t, client, clusterID, isEtcd, isControlPlane, isWorker)

	for i := range nodeToDelete {
		logrus.Info("Replacing node: " + nodeToDelete[i].NodeName)
		err = client.Management.Node.Delete(&nodeToDelete[i])
		require.NoError(t, err)

		err = clusters.WaitClusterToBeUpgraded(client, clusterID)
		require.NoError(t, err)

		logrus.Infof("Checking if node %s is replaced", nodeToDelete[i].NodeName)
		err = nodestat.AllManagementNodeReady(client, clusterID, defaults.ThirtyMinuteTimeout)
		require.NoError(t, err)

		_, err = nodestat.IsNodeReplaced(client, nodeToDelete[i].ID, clusterID, numOfNodesBeforeDeletion)
		require.NoError(t, err)

		podErrors := pods.StatusPods(client, clusterID)
		assert.Empty(t, podErrors)
	}
}
