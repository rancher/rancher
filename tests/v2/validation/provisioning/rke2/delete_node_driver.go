package rke2

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	nodestat "github.com/rancher/rancher/tests/framework/extensions/nodes"
	"github.com/stretchr/testify/require"
)

func TestDeletingRKE2Cluster(t *testing.T, client *rancher.Client, cluster *v1.SteveAPIObject) {
	err := clusters.DeleteK3SRKE2Cluster(client, cluster)
	require.NoError(t, err)

	err = nodestat.IsNodeDeleted(client, cluster.ID)
	require.NoError(t, err)
}
