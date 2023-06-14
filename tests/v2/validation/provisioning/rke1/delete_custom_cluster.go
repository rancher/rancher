package rke1

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	nodestat "github.com/rancher/rancher/tests/framework/extensions/nodes"
	"github.com/stretchr/testify/require"
)

func TestDeletingRKE1CustomCluster(t *testing.T, client *rancher.Client, cluster *management.Cluster) {
	err := clusters.DeleteRKE1Cluster(client, cluster)
	require.NoError(t, err)

	err = nodestat.IsNodeDeleted(client, cluster.ID)
	require.NoError(t, err)
}
