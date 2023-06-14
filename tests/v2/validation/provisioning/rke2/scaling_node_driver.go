package rke2

import (
	"testing"

	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	"github.com/stretchr/testify/require"
)

func TestScalingRKE2NodePools(t *testing.T, client *rancher.Client, cluster *v1.SteveAPIObject, updatedCluster *apisV1.Cluster, machineConfig *v1.SteveAPIObject) {
	err := machinepools.ScaleNewWorkerMachinePool(client, cluster, updatedCluster, machineConfig)
	require.NoError(t, err)
}
