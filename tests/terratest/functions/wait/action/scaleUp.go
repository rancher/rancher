package functions

import (
	"testing"
	"time"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/terratest/tests"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/wait"
)

func ScaleUp(t *testing.T, client *rancher.Client, clusterID string) (done bool, err error) {
	clusterConfig := new(tests.TerratestConfig)
	config.LoadConfig("terratest", clusterConfig)

	wait_err := wait.Poll(100*time.Millisecond, 30*time.Minute, func() (done bool, err error) {
		cluster, err := client.Management.Cluster.ByID(clusterID)
		require.NoError(t, err)

		if err != nil {
			return false, err
		}

		if cluster.NodeCount == clusterConfig.ScaledUpNodeCount {
			return true, nil
		}

		return false, nil
	})
	require.NoError(t, wait_err)

	return true, nil
}
