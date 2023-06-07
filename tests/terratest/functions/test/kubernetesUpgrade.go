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
	"github.com/rancher/rancher/tests/v2/validation/terratest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func KubernetesUpgrade(t *testing.T, terraformOptions *terraform.Options, client *rancher.Client) error {
	var provider string
	var expectedUpgradedKubernetesVersion string

	terraformConfig := new(terratest.TerraformConfig)
	config.LoadConfig("terraform", terraformConfig)

	clusterConfig := new(terratest.TerratestConfig)
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
		t.Logf("Invalid module provided. Valid modules are: aks, eks, gke, ec2_rke1, linode_rke1, ec2_rke2, linode_rke2, ec2_k3s, linode_k3s")
		return fmt.Errorf("invalid module provided")
	}

	result, err := set.SetConfigTF(t, clusterConfig.UpgradedKubernetesVersion, clusterConfig.Nodepools)
	require.NoError(t, err)
	assert.Equal(t, true, result)

	terraform.Apply(t, terraformOptions)

	clusterID, err := clusters.GetClusterIDByName(client, terraformConfig.ClusterName)
	require.NoError(t, err)

	wait.WaitFor(t, client, clusterID, "kubernetes-upgrade")

	cluster, err := client.Management.Cluster.ByID(clusterID)
	require.NoError(t, err)

	assert.Equal(t, terraformConfig.ClusterName, cluster.Name)
	assert.Equal(t, provider, cluster.Provider)
	assert.Equal(t, "active", cluster.State)
	assert.Equal(t, expectedUpgradedKubernetesVersion, cluster.Version.GitVersion)

	return nil
}
