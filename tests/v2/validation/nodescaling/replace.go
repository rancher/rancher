package nodescaling

import (
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rancher/norman/types"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/extensions/nodes"
	nodestat "github.com/rancher/shepherd/extensions/nodes"
	"github.com/rancher/shepherd/extensions/provisioninginput"
	"github.com/rancher/shepherd/extensions/sshkeys"
	"github.com/rancher/shepherd/extensions/workloads/pods"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

const (
	shutdownCommand = "sudo shutdown -h now"
	controlPlane    = "control-plane"
	etcd            = "etcd"
	worker          = "worker"

	unreachableCondition         = "NodeStatusUnknown"
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

// shutdownFirstNodeWithRole uses ssh to shutdown the first node matching the specified role in a given cluster.
func shutdownFirstNodeWithRole(client *rancher.Client, stevecluster *steveV1.SteveAPIObject, clusterID, nodeRole string) (*steveV1.SteveAPIObject, error) {
	steveclient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return nil, err
	}

	query, err := url.ParseQuery("labelSelector=node-role.kubernetes.io/" + nodeRole + "=true")
	if err != nil {
		return nil, err
	}

	nodeList, err := steveclient.SteveType("node").List(query)
	if err != nil {
		return nil, err
	}

	firstMachine := nodeList.Data[0]

	sshUser, err := sshkeys.GetSSHUser(client, stevecluster)
	if err != nil {
		return nil, err
	}

	if sshUser == "" {
		return nil, errors.New("sshUser does not exist")
	}

	sshNode, err := sshkeys.GetSSHNodeFromMachine(client, sshUser, &firstMachine)
	if err != nil {
		return nil, err
	}

	logrus.Infof("Running node auto-replace on node %s", firstMachine.Name)

	// Shutdown node using ssh outside of Rancher to simulate unhealthy node
	_, err = sshNode.ExecuteCommand(shutdownCommand)
	if err != nil && !errors.Is(err, &ssh.ExitMissingError{}) {
		return nil, err
	}

	return &firstMachine, nil
}

// matchNodeToMachinePool takes a given node name and returns the cluster's first matching machinePool from its RKEConfig, if any.
func matchNodeToMachinePool(client *rancher.Client, clusterObject *steveV1.SteveAPIObject, nodeName string) (*provv1.RKEMachinePool, error) {
	clusterSpec := &provv1.ClusterSpec{}
	err := steveV1.ConvertToK8sType(clusterObject.Spec, clusterSpec)
	if err != nil {
		return nil, err
	}

	for _, pool := range clusterSpec.RKEConfig.MachinePools {
		if strings.Contains(nodeName, "-"+pool.Name+"-") {

			return &pool, nil
		}
	}

	return nil, errors.New("could not find matching machine pool for this node")
}

// AutoReplaceFirstNodeWithRole ssh into the first node with the specified role and shuts it down. If the node is replacable,
// wait for the cluster to return to a healthy state. Otherwise, we expect the cluster to never return to active, as the node will remain unreachable.
func AutoReplaceFirstNodeWithRole(t *testing.T, client *rancher.Client, clusterName, nodeRole string) {
	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	require.NoError(t, err)

	_, stevecluster, err := clusters.GetProvisioningClusterByName(client, clusterName, provisioninginput.Namespace)
	require.NoError(t, err)

	machine, err := shutdownFirstNodeWithRole(client, stevecluster, clusterID, nodeRole)
	require.NoError(t, err)

	machinePool, err := matchNodeToMachinePool(client, stevecluster, machine.Name)
	require.NoError(t, err)

	if nodeRole == controlPlane || nodeRole == etcd {
		err = clusters.WaitClusterToBeUpgraded(client, clusterID)
		if machinePool.UnhealthyNodeTimeout.String() == "0s" {
			require.Error(t, err, "UnhealthyNodeTimeout set to 0s, but node was replaced!")
			return
		}
		require.NoError(t, err)
	}

	err = nodes.Isv1NodeConditionMet(client, machine.ID, clusterID, unreachableCondition)
	if machinePool.UnhealthyNodeTimeout.String() == "0s" {
		require.Error(t, err, "UnhealthyNodeTimeout set to 0s, but node was replaced!")
		return
	}
	require.NoError(t, err)

	steveclient, err := client.Steve.ProxyDownstream(clusterID)
	require.NoError(t, err)

	v1NodeList, err := steveclient.SteveType("node").List(nil)
	require.NoError(t, err)

	_, err = nodes.IsNodeReplaced(client, machine.Name, clusterID, len(v1NodeList.Data))
	require.NoError(t, err)

	err = nodes.AllMachineReady(client, clusterID, machinePool.UnhealthyNodeTimeout.Duration+time.Duration(1800))
	require.NoError(t, err)

	err = clusters.WaitClusterToBeUpgraded(client, clusterID)
	require.NoError(t, err)
}
