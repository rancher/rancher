package pods

import (
	"testing"
	"time"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/workloads"
	"github.com/rancher/shepherd/clients/rancher"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// VerifyReadyDaemonsetPods tries to poll the Steve API to verify the expected number of daemonset pods are in the Ready
// state
func VerifyReadyDaemonsetPods(t *testing.T, client *rancher.Client, cluster *v1.SteveAPIObject) {
	status := &provv1.ClusterStatus{}
	err := v1.ConvertToK8sType(cluster.Status, status)
	require.NoError(t, err)

	daemonsetequals := false

	err = wait.Poll(500*time.Millisecond, 5*time.Minute, func() (dameonsetequals bool, err error) {
		daemonsets, err := client.Steve.SteveType(workloads.DaemonsetSteveType).ByID(status.ClusterName)
		require.NoError(t, err)

		daemonsetsStatusType := &appv1.DaemonSetStatus{}
		err = v1.ConvertToK8sType(daemonsets.Status, daemonsetsStatusType)
		require.NoError(t, err)

		if daemonsetsStatusType.DesiredNumberScheduled == daemonsetsStatusType.NumberAvailable {
			return true, nil
		}
		return false, err
	})
	require.NoError(t, err)

	daemonsets, err := client.Steve.SteveType(workloads.DaemonsetSteveType).ByID(status.ClusterName)
	require.NoError(t, err)

	daemonsetsStatusType := &appv1.DaemonSetStatus{}
	err = v1.ConvertToK8sType(daemonsets.Status, daemonsetsStatusType)
	require.NoError(t, err)

	if daemonsetsStatusType.DesiredNumberScheduled == daemonsetsStatusType.NumberAvailable {
		daemonsetequals = true
	}

	assert.Truef(t, daemonsetequals, "Ready Daemonset Pods didn't match expected")
}
