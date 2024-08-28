package deleting

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/machinepools"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/extensions/steve"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

const (
	machineSteveResourceType = "cluster.x-k8s.io.machine"
)

func deleteInitMachine(t *testing.T, client *rancher.Client, clusterID string) {
	initMachine, err := machinepools.GetInitMachine(client, clusterID)
	require.NoError(t, err)

	err = client.Steve.SteveType(machineSteveResourceType).Delete(initMachine)
	require.NoError(t, err)

	logrus.Info("Awaiting machine deletion...")
	err = steve.WaitForSteveResourceDeletion(client, defaults.FiveHundredMillisecondTimeout, defaults.TenMinuteTimeout, machineSteveResourceType, initMachine.ID)
	require.NoError(t, err)

	logrus.Info("Awaiting machine replacement...")
	err = clusters.WatchAndWaitForCluster(client, clusterID)
	require.NoError(t, err)
}
