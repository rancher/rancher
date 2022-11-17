package tests

import (
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/terratest/functions"
	"github.com/rancher/rancher/tests/terratest/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/wait"
)

func TestEKSProvisionAndScale(t *testing.T) {
	t.Parallel()

	module := "eks"
	active := "active"

	clusterConfig := new(tests.TerratestConfig)
	config.LoadConfig("terratest", clusterConfig)

	// Set terraform.tfvars file
	functions.SetVarsTF(module)

	// Set initial infrastructure by building TFs declarative config file - [main.tf]
	successful, err := functions.SetConfigTF(module, clusterConfig.KubernetesVersion, clusterConfig.Nodepools)
	require.NoError(t, err)
	assert.Equal(t, true, successful)

	terraformOptions := terraform.WithDefaultRetryableErrors(t, &terraform.Options{

		TerraformDir: "../../modules/hosted/" + module,
		NoColor:      true,
	})

	cleanup := func() {
		terraform.Destroy(t, terraformOptions)
		functions.CleanupConfigTF(module)
		functions.CleanupVarsTF(module)
	}

	// Deploys [main.tf] infrastructure and sets up resource cleanup
	defer cleanup()
	terraform.InitAndApply(t, terraformOptions)

	time.Sleep(1 * time.Second)

	// Grab cluster name from TF outputs
	clusterName := terraform.Output(t, terraformOptions, "cluster_name")

	// Create session, client, and grab cluster specs
	testSession := session.NewSession(t)

	client, err := rancher.NewClient("", testSession)
	require.NoError(t, err)

	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	require.NoError(t, err)

	cluster, err := client.Management.Cluster.ByID(clusterID)
	require.NoError(t, err)

	// Test cluster
	assert.Equal(t, clusterName, cluster.Name)
	assert.Equal(t, clusterConfig.NodeCount, cluster.NodeCount)
	assert.Equal(t, module, cluster.Provider)
	assert.Equal(t, active, cluster.State)

	// Scale up cluster
	successful, err = functions.SetConfigTF(module, clusterConfig.KubernetesVersion, clusterConfig.ScaledUpNodepools)
	require.NoError(t, err)
	assert.Equal(t, true, successful)

	terraform.Apply(t, terraformOptions)

	// Wait for active cluster state
	err = wait.Poll(500*time.Millisecond, 30*time.Minute, func() (done bool, err error) {
		cluster, err := client.Management.Cluster.ByID(clusterID)
		require.NoError(t, err)

		if err != nil {
			return false, err
		}

		if cluster.State == "active" {
			return true, nil
		}

		return false, nil
	})
	require.NoError(t, err)

	// Wait for nodes to scale up
	err = wait.Poll(500*time.Millisecond, 30*time.Minute, func() (done bool, err error) {
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
	require.NoError(t, err)

	// Wait for active cluster state
	err = wait.Poll(500*time.Millisecond, 30*time.Minute, func() (done bool, err error) {
		cluster, err := client.Management.Cluster.ByID(clusterID)
		require.NoError(t, err)

		if err != nil {
			return false, err
		}

		if cluster.State == "active" {
			return true, nil
		}

		return false, nil
	})
	require.NoError(t, err)

	// Update cluster object
	cluster, err = client.Management.Cluster.ByID(clusterID)
	require.NoError(t, err)

	// Test cluster
	assert.Equal(t, clusterName, cluster.Name)
	assert.Equal(t, clusterConfig.ScaledUpNodeCount, cluster.NodeCount)
	assert.Equal(t, module, cluster.Provider)
	assert.Equal(t, active, cluster.State)

	// Scale down cluster
	successful, err = functions.SetConfigTF(module, clusterConfig.KubernetesVersion, clusterConfig.ScaledDownNodepools)
	require.NoError(t, err)
	assert.Equal(t, true, successful)

	terraform.Apply(t, terraformOptions)

	// Wait for nodes to scale down
	err = wait.Poll(500*time.Millisecond, 30*time.Minute, func() (done bool, err error) {
		cluster, err := client.Management.Cluster.ByID(clusterID)
		require.NoError(t, err)

		if err != nil {
			return false, err
		}

		if cluster.NodeCount == clusterConfig.ScaledDownNodeCount {
			return true, nil
		}

		return false, nil
	})
	require.NoError(t, err)

	// Wait for active cluster state
	err = wait.Poll(500*time.Millisecond, 30*time.Minute, func() (done bool, err error) {
		cluster, err := client.Management.Cluster.ByID(clusterID)
		require.NoError(t, err)

		if err != nil {
			return false, err
		}

		if cluster.State == "active" {
			return true, nil
		}

		return false, nil
	})
	require.NoError(t, err)

	// Update cluster object
	cluster, err = client.Management.Cluster.ByID(clusterID)
	require.NoError(t, err)

	// Test cluster
	assert.Equal(t, clusterName, cluster.Name)
	assert.Equal(t, clusterConfig.ScaledDownNodeCount, cluster.NodeCount)
	assert.Equal(t, module, cluster.Provider)
	assert.Equal(t, active, cluster.State)

}
