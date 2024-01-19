package deleting

import (
	"testing"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/nodes"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

const (
	fleetNamespace        = "fleet-default"
	initNodeLabelKey      = "rke.cattle.io/init-node"
	True                  = "true"
	local                 = "local"
	machineNameSteveLabel = "rke.cattle.io/machine-name"
	machinePlanSecretType = "rke.cattle.io/machine-plan"
    machineSteveResourceType = "cluster.x-k8s.io.machine"
)

func deleteInitMachine(t *testing.T, client *rancher.Client, clusterID string) {
	initMachine, err := nodes.GetInitMachine(client, clusterID)
	require.NoError(t, err)

	err = nodes.DeleteMachineRKE2K3S(client, initMachine)
	require.NoError(t, err)

	logrus.Info("Awaiting machine replacement...")

	err = clusters.WaitClusterToBeUpgraded(client, clusterID)
	require.NoError(t, err)

	err = clusters.WaitClusterToBeUpgraded(client, clusterID)
	require.NoError(t, err)

	err = nodes.VerifyDeletedMachineRKE2K3S(client, initMachine)
	require.NoError(t, err)
}