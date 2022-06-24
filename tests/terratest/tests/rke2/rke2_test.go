package tests

import (
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/terratest/config"
	"github.com/rancher/rancher/tests/terratest/functions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRke2DownStreamCluster(t *testing.T) {
	t.Parallel()

	// Set initial infrastructure by building TFs declarative config file - [main.tf]
	config.Build_Nodes3_Etcd1_Cp1_Wkr1()
	config1 := functions.SetConfigTF(config.Rke2, config.RKE2K8sVersion12210, config.Nodes3_Etcd1_Cp1_Wkr1)
	assert.Equal(t, true, config1)

	terraformOptions := terraform.WithDefaultRetryableErrors(t, &terraform.Options{

		TerraformDir: "../../modules/rke2",
		NoColor:      true,
	})

	cleanup := func() {
		terraform.Destroy(t, terraformOptions)
		functions.CleanupConfigTF(config.Rke2)
	}

	// Deploys [main.tf] infrastructure and sets up resource cleanup
	defer cleanup()
	terraform.InitAndApply(t, terraformOptions)

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
	assert.Equal(t, functions.OutputToInt64(terraform.Output(t, terraformOptions, "expected_node_count_3")), cluster.NodeCount)
	assert.Equal(t, terraform.Output(t, terraformOptions, "expected_provider"), cluster.Provider)
	assert.Equal(t, terraform.Output(t, terraformOptions, "expected_state_active"), cluster.State)
	assert.Equal(t, terraform.Output(t, terraformOptions, "expected_kubernetes_version_12210"), cluster.Version.GitVersion)

	// Upgrade k8s version
	upgradedK8s := functions.SetConfigTF(config.Rke2, config.RKE2K8sVersion1237, config.Nodes3_Etcd1_Cp1_Wkr1)
	assert.Equal(t, true, upgradedK8s)

	terraform.Apply(t, terraformOptions)

	// Wait for cluster
	functions.WaitForActiveCluster(t, client, clusterID)

	// Update cluster object
	cluster, err = client.Management.Cluster.ByID(clusterID)
	require.NoError(t, err)

	// Test cluster
	assert.Equal(t, clusterName, cluster.Name)
	assert.Equal(t, functions.OutputToInt64(terraform.Output(t, terraformOptions, "expected_node_count_3")), cluster.NodeCount)
	assert.Equal(t, terraform.Output(t, terraformOptions, "expected_provider"), cluster.Provider)
	assert.Equal(t, terraform.Output(t, terraformOptions, "expected_state_active"), cluster.State)
	assert.Equal(t, terraform.Output(t, terraformOptions, "expected_kubernetes_version_1237"), cluster.Version.GitVersion)

	// Scale to HA setup - 3 node pools: [3 etcd], [2 cp], [3 wkr]
	config.Build_Nodes8_HACluster()
	config2 := functions.SetConfigTF(config.Rke2, config.RKE2K8sVersion1237, config.Nodes8_HACluster)
	assert.Equal(t, true, config2)

	terraform.Apply(t, terraformOptions)

	// Wait for cluster
	functions.WaitForActiveCluster(t, client, clusterID)

	// Update cluster object
	cluster, err = client.Management.Cluster.ByID(clusterID)
	require.NoError(t, err)

	// Test cluster
	assert.Equal(t, clusterName, cluster.Name)
	assert.Equal(t, functions.OutputToInt64(terraform.Output(t, terraformOptions, "expected_node_count_8")), cluster.NodeCount)
	assert.Equal(t, terraform.Output(t, terraformOptions, "expected_provider"), cluster.Provider)
	assert.Equal(t, terraform.Output(t, terraformOptions, "expected_state_active"), cluster.State)
	assert.Equal(t, terraform.Output(t, terraformOptions, "expected_kubernetes_version_1237"), cluster.Version.GitVersion)

	// Scale Wkr pool to one - 3 node pools: [3 etcd], [2 cp], [1 wkr]
	config.Build_Nodes6_Etcd3_Cp2_Wkr1()
	config3 := functions.SetConfigTF(config.Rke2, config.RKE2K8sVersion1237, config.Nodes6_Etcd3_Cp2_Wkr1)
	assert.Equal(t, true, config3)

	terraform.Apply(t, terraformOptions)

	// Wait for cluster
	functions.WaitForActiveCluster(t, client, clusterID)

	// Update cluster object
	cluster, err = client.Management.Cluster.ByID(clusterID)
	require.NoError(t, err)

	// Test cluster
	assert.Equal(t, clusterName, cluster.Name)
	assert.Equal(t, functions.OutputToInt64(terraform.Output(t, terraformOptions, "expected_node_count_6")), cluster.NodeCount)
	assert.Equal(t, terraform.Output(t, terraformOptions, "expected_provider"), cluster.Provider)
	assert.Equal(t, terraform.Output(t, terraformOptions, "expected_state_active"), cluster.State)
	assert.Equal(t, terraform.Output(t, terraformOptions, "expected_kubernetes_version_1237"), cluster.Version.GitVersion)

}
