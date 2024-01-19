package deleting

import (
	"testing"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/nodes"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func deleteInitNode(t *testing.T, client *rancher.Client, clusterID string) {
	initNode, err := nodes.GetInitNode(client, clusterID)
	require.NoError(t, err)

	err = nodes.DeleteNodeRKE2K3S(client, initNode)
	require.NoError(t, err)

	logrus.Info("Awaiting machine replacement...")

	err = clusters.WaitClusterToBeUpgraded(client, clusterID)
	require.NoError(t, err)

	err = clusters.WaitClusterToBeUpgraded(client, clusterID)
	require.NoError(t, err)

	err = nodes.VerifyDeletedNodeRKE2K3S(client, initNode)
	require.NoError(t, err)
}