package nodescaling

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/aks"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/eks"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/gke"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	rke1 "github.com/rancher/rancher/tests/framework/extensions/rke1/nodepools"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	"github.com/stretchr/testify/require"
)

const (
	ProvisioningSteveResourceType = "provisioning.cattle.io.cluster"
	defaultNamespace              = "fleet-default"
)

var oneNode int64 = 1
var twoNodes int64 = 2

func scalingRKE2K3SNodePools(t *testing.T, client *rancher.Client, clusterID string, nodeRoles machinepools.NodeRoles) {
	cluster, err := client.Steve.SteveType(ProvisioningSteveResourceType).ByID(clusterID)
	require.NoError(t, err)

	clusterResp, err := machinepools.ScaleMachinePoolNodes(client, cluster, nodeRoles)
	require.NoError(t, err)

	pods.VerifyReadyDaemonsetPods(t, client, cluster)

	nodeRoles.Quantity = -nodeRoles.Quantity
	scaledClusterResp, err := machinepools.ScaleMachinePoolNodes(client, clusterResp, nodeRoles)
	require.NoError(t, err)

	pods.VerifyReadyDaemonsetPods(t, client, scaledClusterResp)
}

func scalingRKE1NodePools(t *testing.T, client *rancher.Client, clusterID string, nodeRoles rke1.NodeRoles) {
	cluster, err := client.Management.Cluster.ByID(clusterID)
	require.NoError(t, err)

	node, err := rke1.MatchRKE1NodeRoles(client, cluster, nodeRoles)
	require.NoError(t, err)

	_, err = rke1.ScaleNodePoolNodes(client, cluster, node, nodeRoles)
	require.NoError(t, err)

	nodeRoles.Quantity = -nodeRoles.Quantity
	_, err = rke1.ScaleNodePoolNodes(client, cluster, node, nodeRoles)
	require.NoError(t, err)
}

func scalingAKSNodePools(t *testing.T, client *rancher.Client, clusterID string, nodePool *aks.NodePool) {
	cluster, err := client.Management.Cluster.ByID(clusterID)
	require.NoError(t, err)

	clusterResp, err := aks.ScalingAKSNodePoolsNodes(client, cluster, nodePool)
	require.NoError(t, err)

	*nodePool.NodeCount = -*nodePool.NodeCount
	_, err = aks.ScalingAKSNodePoolsNodes(client, clusterResp, nodePool)
	require.NoError(t, err)
}

func scalingEKSNodePools(t *testing.T, client *rancher.Client, clusterID string, nodePool *eks.NodeGroupConfig) {
	cluster, err := client.Management.Cluster.ByID(clusterID)
	require.NoError(t, err)

	clusterResp, err := eks.ScalingEKSNodePoolsNodes(client, cluster, nodePool)
	require.NoError(t, err)

	*nodePool.DesiredSize = -*nodePool.DesiredSize
	_, err = eks.ScalingEKSNodePoolsNodes(client, clusterResp, nodePool)
	require.NoError(t, err)
}

func scalingGKENodePools(t *testing.T, client *rancher.Client, clusterID string, nodePool *gke.NodePool) {
	cluster, err := client.Management.Cluster.ByID(clusterID)
	require.NoError(t, err)

	clusterResp, err := gke.ScalingGKENodePoolsNodes(client, cluster, nodePool)
	require.NoError(t, err)

	*nodePool.InitialNodeCount = -*nodePool.InitialNodeCount
	_, err = gke.ScalingGKENodePoolsNodes(client, clusterResp, nodePool)
	require.NoError(t, err)
}
