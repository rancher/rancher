package functions

import (
	"fmt"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	wait "github.com/rancher/rancher/tests/terratest/functions/wait"
	"github.com/rancher/rancher/tests/v2/validation/terratest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

func Provision(t *testing.T, terraformOptions *terraform.Options) (*rancher.Client, error) {
	var provider string
	var expectedKubernetesVersion string

	terraformConfig := new(terratest.TerraformConfig)
	config.LoadConfig("terraform", terraformConfig)

	clusterConfig := new(terratest.TerratestConfig)
	config.LoadConfig("terratest", clusterConfig)

	module := terraformConfig.Module

	switch {
	case module == "aks":
		provider = "aks"
		expectedKubernetesVersion = `v` + clusterConfig.KubernetesVersion

	case module == "eks":
		provider = "eks"
		expectedKubernetesVersion = clusterConfig.KubernetesVersion

	case module == "gke":
		provider = "gke"
		expectedKubernetesVersion = `v` + clusterConfig.KubernetesVersion

	case module == "ec2_rke1" || module == "linode_rke1":
		provider = "rke"
		expectedKubernetesVersion = clusterConfig.KubernetesVersion[:len(clusterConfig.KubernetesVersion)-11]

	case module == "ec2_rke2" || module == "linode_rke2":
		provider = "rke2"
		expectedKubernetesVersion = clusterConfig.KubernetesVersion

	case module == "ec2_k3s" || module == "linode_k3s":
		provider = "k3s"
		expectedKubernetesVersion = clusterConfig.KubernetesVersion

	default:
		return nil, fmt.Errorf("invalid module provided")
	}

	terraform.InitAndApply(t, terraformOptions)

	testSession := session.NewSession()

	client, err := rancher.NewClient("", testSession)
	require.NoError(t, err)

	clusterID, err := clusters.GetClusterIDByName(client, terraformConfig.ClusterName)
	require.NoError(t, err)

	if module == "ec2_rke1" || module == "linode_rke1" {
		wait.WaitFor(t, client, clusterID, "provisioning")
	}

	wait_err := kwait.Poll(100*time.Millisecond, 30*time.Minute, func() (done bool, err error) {
		cluster, err := client.Management.Cluster.ByID(clusterID)
		require.NoError(t, err)

		if err != nil {
			return false, err
		}

		if cluster.Name == terraformConfig.ClusterName && cluster.Provider == provider && cluster.State == "active" && cluster.NodeCount == clusterConfig.NodeCount {
			return true, nil
		}

		return false, nil
	})
	require.NoError(t, wait_err)

	cluster, err := client.Management.Cluster.ByID(clusterID)
	require.NoError(t, err)

	assert.Equal(t, terraformConfig.ClusterName, cluster.Name)
	assert.Equal(t, provider, cluster.Provider)
	assert.Equal(t, "active", cluster.State)
	assert.Equal(t, clusterConfig.NodeCount, cluster.NodeCount)
	if module != "eks" {
		assert.Equal(t, expectedKubernetesVersion, cluster.Version.GitVersion)
	}
	if module == "eks" {
		assert.Equal(t, expectedKubernetesVersion, cluster.Version.GitVersion[1:5])
	}

	return client, nil
}