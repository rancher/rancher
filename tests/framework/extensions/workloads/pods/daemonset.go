package pods

import (
	"fmt"
	client2 "github.com/rancher/rancher/pkg/client/generated/project/v3"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/wait"
	"testing"
	"time"
)

const (
	DaemonsetSteveType = "apps.daemonset"
)

// VerifyReadyDaemonsetPods tries to poll the Steve API to verify the expected number of daemonset pods are in the Ready
// state
func VerifyReadyDaemonsetPods(t *testing.T, client *rancher.Client, cluster *v1.SteveAPIObject, expectedDaemonsets int64) error {
	clusterId, err := clusters.GetClusterIDByName(client, cluster.Name)
	require.NoError(t, err)

	steveclient, err := client.Steve.ProxyDownstream(clusterId)
	require.NoError(t, err)

	daemonsetequals := false

	err = wait.Poll(500*time.Millisecond, 5*time.Minute, func() (dameonsetequals bool, err error) {

		daemonsets, err := steveclient.SteveType(DaemonsetSteveType).List(nil)
		if err != nil {
			return false, nil
		}

		daemonsetsStatusType := &client2.DaemonSetStatus{}
		err = v1.ConvertToK8sType(daemonsets.Data[0].Status, daemonsetsStatusType)
		if err != nil {
			return false, nil
		}
		if daemonsetsStatusType.NumberReady == expectedDaemonsets {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		return fmt.Errorf("failed to wait %v damonset pods to be in Ready state: %v", expectedDaemonsets, err)
	}

	daemonsets, err := steveclient.SteveType(DaemonsetSteveType).List(nil)
	if err != nil {
		return err
	}

	daemonsetsStatusType := &client2.DaemonSetStatus{}
	err = v1.ConvertToK8sType(daemonsets.Data[0].Status, daemonsetsStatusType)

	if daemonsetsStatusType.NumberReady == expectedDaemonsets {
		daemonsetequals = true
	}

	assert.Truef(t, daemonsetequals, "Ready Daemonset Pods didn't match expected")
	return err
}
