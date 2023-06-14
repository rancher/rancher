package rke1

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	rke1 "github.com/rancher/rancher/tests/framework/extensions/rke1/nodepools"
	"github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates"
	"github.com/stretchr/testify/require"
)

func TestScalingRKE1NodePools(t *testing.T, client *rancher.Client, cluster *management.Cluster, nodesAndRoles []rke1.NodeRoles, nodeTemplate *nodetemplates.NodeTemplate) {
	err := rke1.ScaleNewWorkerNodePool(client, nodesAndRoles, cluster, nodeTemplate)
	require.NoError(t, err)
}
