package nodescaling

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	rke1 "github.com/rancher/rancher/tests/framework/extensions/rke1/nodepools"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	"github.com/stretchr/testify/require"
)

const (
	ProvisioningSteveResourceType = "provisioning.cattle.io.cluster"
	defaultNamespace              = "fleet-default"
)

func ScalingRKE2K3SNodePools(t *testing.T, client *rancher.Client, clusterID string, nodeRoles machinepools.NodeRoles) {
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

func ScalingRKE1NodePools(t *testing.T, client *rancher.Client, clusterID string, nodeRoles rke1.NodeRoles) {
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
