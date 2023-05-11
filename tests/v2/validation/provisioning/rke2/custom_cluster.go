package rke2

import (
	"context"
	"fmt"
	"testing"

	apiv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/defaults"
	hardening "github.com/rancher/rancher/tests/framework/extensions/hardening/rke2"
	nodestat "github.com/rancher/rancher/tests/framework/extensions/nodes"
	"github.com/rancher/rancher/tests/framework/extensions/pipeline"
	"github.com/rancher/rancher/tests/framework/extensions/tokenregistration"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	"github.com/rancher/rancher/tests/framework/pkg/environmentflag"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	provisioning "github.com/rancher/rancher/tests/v2/validation/provisioning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	namespace = "fleet-default"
)

func TestProvisioningRKE2CustomCluster(t *testing.T, client *rancher.Client, externalNodeProvider provisioning.ExternalNodeProvider, nodesAndRoles []string, kubeVersion, cni string, hardened bool, nodeCountWin int, hasWindows bool) {
	numNodesLin := len(nodesAndRoles)

	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	require.NoError(t, err)

	linuxNodes, winNodes, err := externalNodeProvider.NodeCreationFunc(client, numNodesLin, nodeCountWin, hasWindows)
	require.NoError(t, err)

	clusterName := namegen.AppendRandomString(externalNodeProvider.Name)

	cluster := clusters.NewK3SRKE2ClusterConfig(clusterName, namespace, cni, "", kubeVersion, nil)

	clusterResp, err := clusters.CreateK3SRKE2Cluster(client, cluster)
	require.NoError(t, err)

	if client.Flags.GetValue(environmentflag.UpdateClusterName) {
		pipeline.UpdateConfigClusterName(clusterName)
	}

	client, err = client.ReLogin()
	require.NoError(t, err)
	customCluster, err := client.Steve.SteveType(clusters.ProvisioningSteveResouceType).ByID(clusterResp.ID)
	require.NoError(t, err)

	clusterStatus := &apiv1.ClusterStatus{}
	err = v1.ConvertToK8sType(customCluster.Status, clusterStatus)
	require.NoError(t, err)

	token, err := tokenregistration.GetRegistrationToken(client, clusterStatus.ClusterName)
	require.NoError(t, err)

	for key, linuxNode := range linuxNodes {
		t.Logf("Execute Registration Command for node %s", linuxNode.NodeID)
		command := fmt.Sprintf("%s %s", token.InsecureNodeCommand, nodesAndRoles[key])

		output, err := linuxNode.ExecuteCommand(command)
		require.NoError(t, err)

		t.Logf(output)
	}

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

	if hasWindows {
		for _, winNode := range winNodes {
			t.Logf("Execute Registration Command for node %s", winNode.NodeID)

			output, err := winNode.ExecuteCommand("powershell.exe" + token.InsecureWindowsNodeCommand)
			require.NoError(t, err)

			t.Logf(string(output[:]))
		}

		kubeWinProvisioningClient, err := adminClient.GetKubeAPIProvisioningClient()
		require.NoError(t, err)

		result, err := kubeWinProvisioningClient.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
			FieldSelector: "metadata.name=" + clusterName,

			TimeoutSeconds: &defaults.WatchTimeoutSeconds,
		})
		require.NoError(t, err)

		checkFunc := clusters.IsProvisioningClusterReady
		err = wait.WatchWait(result, checkFunc)
		assert.NoError(t, err)
		assert.Equal(t, clusterName, clusterResp.ObjectMeta.Name)
	}

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

	if hardened && kubeVersion <= string(provisioning.HardenedKubeVersion) {
		err = hardening.HardeningNodes(client, hardened, linuxNodes, nodesAndRoles)
		require.NoError(t, err)

		hardenCluster := clusters.HardenK3SRKE2ClusterConfig(clusterName, namespace, "", "", kubeVersion, nil)

		hardenClusterResp, err := clusters.UpdateK3SRKE2Cluster(client, clusterResp, hardenCluster)
		require.NoError(t, err)
		assert.Equal(t, clusterName, hardenClusterResp.ObjectMeta.Name)

		err = hardening.PostHardeningConfig(client, hardened, linuxNodes, nodesAndRoles)
		require.NoError(t, err)
	}
}
