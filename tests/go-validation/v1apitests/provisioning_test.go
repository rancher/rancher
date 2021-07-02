package v1_api_tests

import (
	"fmt"
	"testing"

	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/tests/go-validation/rancherrest"
	"github.com/rancher/rancher/tests/go-validation/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	digitalOceanCloudCredentialName = "docloudcredential"
	namespace                       = "fleet-default"
	defaultRandStringLength         = 5
	baseDOClusterName               = "docluster"
)

func TestProvisioning_DigitalOcean(t *testing.T) {
	t.Log("Create Cloud Credential")
	client, err := rancherrest.NewClient()
	require.NoError(t, err)

	cloudCredentialName := digitalOceanCloudCredentialName + utils.RandStringBytes(defaultRandStringLength)
	doCloudCred := rancherrest.NewDigitalOceanCloudCredential(cloudCredentialName, "", namespace)

	_, err = client.CreateCloudCredential(doCloudCred)
	require.NoError(t, err)
	t.Log("Cloud Credential was created successfully")

	//randominize name
	clusterName := baseDOClusterName + utils.RandStringBytes(defaultRandStringLength)

	generatedPoolName := fmt.Sprintf("nc-%s-pool1", clusterName)

	machinePoolConfig := rancherrest.NewMachinePoolConfig(generatedPoolName, rancherrest.DOKind, namespace, rancherrest.DOPoolType, "ubuntu-20-04-x64", "nyc3", "s-2vcpu-4gb")

	t.Logf("Creating DO machine pool config %s", generatedPoolName)
	podConfigClient := client.NewPodConfigClient(rancherrest.DOResourceConfig)

	machineConfigResult, err := podConfigClient.CreateMachineConfigPool(machinePoolConfig)
	require.NoError(t, err)

	t.Logf("Successfully created DO machine pool %s", generatedPoolName)

	provisioningClient, err := rancherrest.NewProvisioningClient()
	require.NoError(t, err)

	machinePool := rancherrest.MachinePoolSetup(true, true, true, "pool1", 1, machineConfigResult)
	machinePools := []apisV1.RKEMachinePool{
		machinePool,
	}
	clusterConfig := rancherrest.NewClusterConfig(clusterName, namespace, "calico", machineConfigResult.GetName(), cloudCredentialName, machinePools)

	cluster := provisioningClient.NewCluster(namespace)

	t.Logf("Creating Cluster %s", clusterName)
	v1Cluster, err := cluster.CreateCluster(clusterConfig)
	require.NoError(t, err)

	assert.Equal(t, v1Cluster.Name, clusterName)

	t.Logf("Created Cluster %s", v1Cluster.ClusterName)

	t.Logf("Checking status of cluster %s", v1Cluster.ClusterName)
	//check cluster status
	ready, err := cluster.CheckClusterStatus(clusterName)
	assert.NoError(t, err)
	assert.True(t, ready)

	if utils.RancherCleanup() {
		err = ClearDigitalOceanCluster(client, doCloudCred, cluster, clusterName)
		require.NoError(t, err)
	}

}

func ClearDigitalOceanCluster(client *rancherrest.Client, cloudCredConfig *v1.Secret, cluster *rancherrest.Cluster, clusterName string) error {
	err := client.DeleteCloudCredential(cloudCredConfig)
	if err != nil {
		return err
	}

	err = cluster.DeleteCluster(clusterName)
	if err != nil {
		return err
	}
	return nil
}
