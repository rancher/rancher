//go:build (validation || extended) && !infra.any && !infra.aks && !infra.eks && !infra.gke && !infra.rke2k3s && !cluster.any && !cluster.custom && !cluster.nodedriver && !sanity && !stress

package nodescaling

import (
	"errors"
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/pkg/nodes"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

const (
	shutdownCommand = "shutdown"
	fleetNamespace  = "fleet-default"
	deletingState   = "deleting"
)

// ReplaceNodes replaces the last node with the specified role(s) in a k3s/rke2 cluster
func TestAutoReplace(t *testing.T, client *rancher.Client, clusterName string, nodeToReplace *nodes.Node, machineName string) error {
	logrus.Infof("Running node auto-replace on node %s", nodeToReplace)

	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	require.NoError(t, err)

	// Shutdown node using ssh outside of Rancher to simulate unhealthy node
	_, err = nodeToReplace.ExecuteCommand(shutdownCommand)
	if err != nil && !errors.Is(err, &ssh.ExitMissingError{}) {
		return err
	}

	// Verify node gets replaced
	err = clusters.WaitClusterToBeUpgraded(client, clusterID)
	if err != nil {
		logrus.Errorf("Rancher was unable to auto replace node %s successfully", nodeToReplace.PublicIPAddress)
		return err
	}

	return nil
}
