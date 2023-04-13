package rke2

import (
	"context"
	"fmt"
	"testing"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	nodestat "github.com/rancher/rancher/tests/framework/extensions/nodes"
	"github.com/rancher/rancher/tests/framework/extensions/pipeline"
	"github.com/rancher/rancher/tests/framework/extensions/sshkeys"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	"github.com/rancher/rancher/tests/framework/pkg/environmentflag"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/nodes"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	provisioning "github.com/rancher/rancher/tests/v2/validation/provisioning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestProvisioningRKE2Cluster(t *testing.T, client *rancher.Client, provider Provider, nodesAndRoles []machinepools.NodeRoles, kubeVersion, cni string) {
	cloudCredential, err := provider.CloudCredFunc(client)
	require.NoError(t, err)

	clusterName := namegen.AppendRandomString(provider.Name.String())
	generatedPoolName := fmt.Sprintf("nc-%s-pool1-", clusterName)
	machinePoolConfig := provider.MachinePoolFunc(generatedPoolName, namespace)

	machineConfigResp, err := client.Steve.SteveType(provider.MachineConfigPoolResourceSteveType).Create(machinePoolConfig)
	require.NoError(t, err)

	machinePools := machinepools.RKEMachinePoolSetup(nodesAndRoles, machineConfigResp)

	cluster := clusters.NewK3SRKE2ClusterConfig(clusterName, namespace, cni, cloudCredential.ID, kubeVersion, machinePools)
	clusterResp, err := clusters.CreateK3SRKE2Cluster(client, cluster)
	require.NoError(t, err)

	if client.Flags.GetValue(environmentflag.UpdateClusterName) {
		pipeline.UpdateConfigClusterName(clusterName)
	}

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

	clusterIDName, err := clusters.GetClusterIDByName(adminClient, clusterName)
	assert.NoError(t, err)

	err = nodestat.IsNodeReady(client, clusterIDName)
	require.NoError(t, err)

	clusterToken, err := clusters.CheckServiceAccountTokenSecret(client, clusterName)
	require.NoError(t, err)
	assert.NotEmpty(t, clusterToken)

	podResults, podErrors := pods.StatusPods(client, clusterIDName)
	assert.NotEmpty(t, podResults)
	assert.Empty(t, podErrors)

	nodesList, err := client.Management.Node.ListAll(&types.ListOpts{
		Filters: map[string]interface{}{
			"clusterId": clusterIDName,
		},
	})
	require.NoError(&testing.T{}, err)
	assert.NotEmpty(&testing.T{}, nodesList)

	for _, rancherNode := range nodesList.Data {
		sshkey, err := sshkeys.DownloadSSHKeys(client, rancherNode.Name)
		require.NoError(&testing.T{}, err)
		assert.NotEmpty(&testing.T{}, sshkey)
		ec2Node := &nodes.Node{
			NodeID:          rancherNode.Name,
			PublicIPAddress: rancherNode.ExternalIPAddress,
			SSHUser:         rancherNode.SshUser,
			SSHKey:          sshkey,
		}

		output, err := provisioning.CheckCPU(ec2Node)
		require.NoError(&testing.T{}, err)
		assert.NotEmpty(&testing.T{}, output)

	}
}
