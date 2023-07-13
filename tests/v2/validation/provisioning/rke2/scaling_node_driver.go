package rke2

import (
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	"testing"

	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	"github.com/stretchr/testify/require"
)

func TestScalingRKE2NodePools(t *testing.T, client *rancher.Client, cluster *v1.SteveAPIObject, updatedCluster *apisV1.Cluster, machineConfig *v1.SteveAPIObject) {

	// Create the Worker Machine Pool with 3 worker nodes
	err, createdClusterResp, createdNodePoolCluster := machinepools.CreateNewWorkerMachinePool(client, cluster, updatedCluster, machineConfig, 3)
	require.NoError(t, err)

	// Verify 6 damonsets are ready (2 * number of new worker nodes created)
	err = pods.VerifyReadyDaemonsetPods(t, client, cluster, 6)
	require.NoError(t, err)

	// Scale up Worker Machine Pool to 4 nodes
	err, scaledClusterResp := machinepools.ScaleNewWorkerMachinePool(client, createdClusterResp, createdNodePoolCluster, 4)
	require.NoError(t, err)

	// Verify ready daemonset pods increases by one
	err = pods.VerifyReadyDaemonsetPods(t, client, cluster, 7)
	require.NoError(t, err)

	// Scale down Worker Machine Pool to 3 nodes
	err, scaledClusterResp = machinepools.ScaleNewWorkerMachinePool(client, createdClusterResp, createdNodePoolCluster, 3)
	require.NoError(t, err)

	// Verify ready daemonset pods decreases back to pre scale amount
	err = pods.VerifyReadyDaemonsetPods(t, client, cluster, 6)
	require.NoError(t, err)

	// Delete Worker Machine Pool
	err = machinepools.DeleteWorkerMachinePool(client, cluster, scaledClusterResp, createdNodePoolCluster)
	require.NoError(t, err)
}
