package functions

import (
	"fmt"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	set "github.com/rancher/rancher/tests/terratest/functions/set"
	wait "github.com/rancher/rancher/tests/terratest/functions/wait"
	"github.com/rancher/rancher/tests/terratest/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func KubernetesUpgrade(t *testing.T, terraformOptions *terraform.Options, client *rancher.Client) error {
	var provider string
	var expectedUpgradedKubernetesVersion string

	terraformConfig := new(tests.TerraformConfig)
	config.LoadConfig("terraform", terraformConfig)

	clusterConfig := new(tests.TerratestConfig)
	config.LoadConfig("terratest", clusterConfig)

	module := terraformConfig.Module

	switch {
	case module == "aks":
		provider = "aks"
		expectedUpgradedKubernetesVersion = `v` + clusterConfig.UpgradedKubernetesVersion

	case module == "eks":
		provider = "eks"

	case module == "gke":
		provider = "gke"
		expectedUpgradedKubernetesVersion = `v` + clusterConfig.UpgradedKubernetesVersion

	case module == "ec2_rke1" || module == "linode_rke1":
		provider = "rke"
		expectedUpgradedKubernetesVersion = clusterConfig.UpgradedKubernetesVersion[:len(clusterConfig.UpgradedKubernetesVersion)-11]

	case module == "ec2_rke2" || module == "linode_rke2":
		provider = "rke2"
		expectedUpgradedKubernetesVersion = clusterConfig.UpgradedKubernetesVersion

	case module == "ec2_k3s" || module == "linode_k3s":
		provider = "k3s"
		expectedUpgradedKubernetesVersion = clusterConfig.UpgradedKubernetesVersion

	default:
		return fmt.Errorf("invalid module provided")
	}

	successful, err := set.SetConfigTF(clusterConfig.UpgradedKubernetesVersion, clusterConfig.Nodepools)
	require.NoError(t, err)
	assert.Equal(t, true, successful)

	terraform.Apply(t, terraformOptions)

	clusterID, err := clusters.GetClusterIDByName(client, terraformConfig.ClusterName)
	require.NoError(t, err)

	// Wait for kubernetes upgrade
	wait.WaitFor(t, client, clusterID, "kubernetes-upgrade")

	// Grab cluster object
	cluster, err := client.Management.Cluster.ByID(clusterID)
	require.NoError(t, err)

	// Test cluster
	assert.Equal(t, terraformConfig.ClusterName, cluster.Name)
	assert.Equal(t, provider, cluster.Provider)
	assert.Equal(t, "active", cluster.State)
	assert.Equal(t, expectedUpgradedKubernetesVersion, cluster.Version.GitVersion)

	return nil
}
