package rke2

import (
	"context"
	"fmt"
	"testing"

	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	nodestat "github.com/rancher/rancher/tests/framework/extensions/nodes"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	provisioning "github.com/rancher/rancher/tests/v2/validation/provisioning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

func CreateAndDeleteRKE2Cluster(t *testing.T, client *rancher.Client, provider Provider, nodesAndRoles []machinepools.NodeRoles, kubeVersion, cni string, externalNodeProvider *provisioning.ExternalNodeProvider) {
	cloudCredential, err := provider.CloudCredFunc(client)
	nodeNames := []string{}

	clusterName := namegen.AppendRandomString(provider.Name.String())
	generatedPoolName := fmt.Sprintf("nc-%s-pool1-", clusterName)
	machinePoolConfig := provider.MachinePoolFunc(generatedPoolName, namespace)

	machineConfigResp, err := client.Steve.SteveType(provider.MachineConfigPoolResourceSteveType).Create(machinePoolConfig)
	require.NoError(t, err)

	machinePools := machinepools.RKEMachinePoolSetup(nodesAndRoles, machineConfigResp)

	cluster := clusters.NewK3SRKE2ClusterConfig(clusterName, namespace, cni, cloudCredential.ID, kubeVersion, machinePools)

	clusterResp, err := clusters.CreateK3SRKE2Cluster(client, cluster)
	require.NoError(t, err)

	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	require.NoError(t, err)

	kubeProvisioningClient, err := adminClient.GetKubeAPIProvisioningClient()
	require.NoError(t, err)

	result, err := kubeProvisioningClient.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + clusterName,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	require.NoError(t, err)

	checkFunc := clusters.IsProvisioningClusterReady

	err = wait.WatchWait(result, checkFunc)
	assert.NoError(t, err)
	assert.Equal(t, clusterName, clusterResp.ObjectMeta.Name)
	assert.Equal(t, kubeVersion, cluster.Spec.KubernetesVersion)

	clusterIDName, err := clusters.GetClusterIDByName(client, clusterName)
	assert.NoError(t, err)

	err = nodestat.IsNodeReady(client, clusterIDName)
	require.NoError(t, err)

	machineResp, err := client.Steve.SteveType("cluster.x-k8s.io.machine").List(nil)
	require.NoError(t, err)
	for _, machine := range machineResp.Data {
		machineobj := &capi.Machine{}
		err = v1.ConvertToK8sType(machine.Spec, &machineobj.Spec)
		require.NoError(t, err)
		if machineobj.Spec.ClusterName == clusterName {
			nodeNames = append(nodeNames, machineobj.Spec.InfrastructureRef.Name)
		}
	}

	err = deleteCluster(t, client, clusterResp)
	assert.NoError(t, err)

}

func deleteCluster(t *testing.T, client *rancher.Client, cluster *v1.SteveAPIObject) error {
	err := client.Steve.SteveType(ProvisioningSteveResouceType).Delete(cluster)
	if err != nil {
		return err
	}
	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	require.NoError(t, err)

	provKubeClient, err := adminClient.GetKubeAPIProvisioningClient()
	if err != nil {
		return err
	}
	watchInterface, err := provKubeClient.Clusters(cluster.ObjectMeta.Namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + cluster.ObjectMeta.Name,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})

	if err != nil {
		return err
	}

	return wait.WatchWait(watchInterface, func(event watch.Event) (ready bool, err error) {
		cluster := event.Object.(*apisV1.Cluster)
		if event.Type == watch.Error {
			return false, fmt.Errorf("there was an error deleting cluster")
		} else if event.Type == watch.Deleted {
			return true, nil
		} else if cluster == nil {
			return true, nil
		}
		return false, nil
	})

}
