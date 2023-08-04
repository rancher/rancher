package rke1

import (
	"fmt"
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/defaults"
	nodestat "github.com/rancher/rancher/tests/framework/extensions/nodes"
	"github.com/rancher/rancher/tests/framework/extensions/pipeline"
	psadeploy "github.com/rancher/rancher/tests/framework/extensions/psact"
	matrix "github.com/rancher/rancher/tests/framework/extensions/rke1/componentchecks"
	nodepools "github.com/rancher/rancher/tests/framework/extensions/rke1/nodepools"
	"github.com/rancher/rancher/tests/framework/extensions/tokenregistration"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	"github.com/rancher/rancher/tests/framework/pkg/environmentflag"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/v2/validation/provisioning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestProvisioningRKE1CustomCluster(t *testing.T, client *rancher.Client, externalNodeProvider provisioning.ExternalNodeProvider, nodesAndRoles []nodepools.NodeRoles, psact, kubeVersion, cni string, advancedOptions provisioning.AdvancedOptions) {
	rolesPerNode := []string{}
	quantityPerPool := []int32{}
	rolesPerPool := []string{}
	for _, nodes := range nodesAndRoles {
		var finalRoleCommand string
		if nodes.ControlPlane {
			finalRoleCommand += " --controlplane"
		}
		if nodes.Etcd {
			finalRoleCommand += " --etcd"
		}
		if nodes.Worker {
			finalRoleCommand += " --worker"
		}

		quantityPerPool = append(quantityPerPool, int32(nodes.Quantity))
		rolesPerPool = append(rolesPerPool, finalRoleCommand)
		for i := int64(0); i < nodes.Quantity; i++ {
			rolesPerNode = append(rolesPerNode, finalRoleCommand)
		}
	}

	nodes, err := externalNodeProvider.NodeCreationFunc(client, rolesPerPool, quantityPerPool)
	require.NoError(t, err)

	clusterName := namegen.AppendRandomString(externalNodeProvider.Name)

	cluster := clusters.NewRKE1ClusterConfig(clusterName, cni, kubeVersion, psact, client, advancedOptions)
	clusterResp, err := clusters.CreateRKE1Cluster(client, cluster)
	require.NoError(t, err)

	if client.Flags.GetValue(environmentflag.UpdateClusterName) {
		pipeline.UpdateConfigClusterName(clusterName)
	}

	client, err = client.ReLogin()
	require.NoError(t, err)

	customCluster, err := client.Management.Cluster.ByID(clusterResp.ID)
	require.NoError(t, err)

	token, err := tokenregistration.GetRegistrationToken(client, customCluster.ID)
	require.NoError(t, err)

	for key, node := range nodes {
		t.Logf("Execute Registration Command for node %s", node.NodeID)
		command := fmt.Sprintf("%s %s --address %s --internal-address %s",
			token.NodeCommand, rolesPerNode[key], node.PublicIPAddress, node.PrivateIPAddress)

		output, err := node.ExecuteCommand(command)
		require.NoError(t, err)
		t.Logf(output)
	}

	opts := metav1.ListOptions{
		FieldSelector:  "metadata.name=" + clusterResp.ID,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	}

	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	require.NoError(t, err)
	watchInterface, err := adminClient.GetManagementWatchInterface(management.ClusterType, opts)
	require.NoError(t, err)

	checkFunc := clusters.IsHostedProvisioningClusterReady

	err = wait.WatchWait(watchInterface, checkFunc)
	require.NoError(t, err)
	assert.Equal(t, clusterName, clusterResp.Name)
	assert.Equal(t, kubeVersion, clusterResp.RancherKubernetesEngineConfig.Version)

	err = nodestat.IsNodeReady(client, clusterResp.ID)
	require.NoError(t, err)

	clusterToken, err := clusters.CheckServiceAccountTokenSecret(client, clusterName)
	require.NoError(t, err)
	assert.NotEmpty(t, clusterToken)

	etcdVersion, err := matrix.CheckETCDVersion(client, nodes, clusterResp.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, etcdVersion)

	if psact == string(provisioning.RancherPrivileged) || psact == string(provisioning.RancherRestricted) {
		err = psadeploy.CheckPSACT(client, clusterName)
		require.NoError(t, err)

		_, err = psadeploy.CreateNginxDeployment(client, clusterName, psact)
		require.NoError(t, err)
	}

	podResults, podErrors := pods.StatusPods(client, clusterResp.ID)
	assert.NotEmpty(t, podResults)
	assert.Empty(t, podErrors)
}
