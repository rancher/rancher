package rke2

import (
	"context"
	"fmt"
	"strings"
	"testing"

	apiv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/defaults"
	hardening "github.com/rancher/rancher/tests/framework/extensions/hardening/rke2"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	nodestat "github.com/rancher/rancher/tests/framework/extensions/nodes"
	"github.com/rancher/rancher/tests/framework/extensions/pipeline"
	psadeploy "github.com/rancher/rancher/tests/framework/extensions/psact"
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

func TestProvisioningRKE2CustomCluster(t *testing.T, client *rancher.Client, externalNodeProvider provisioning.ExternalNodeProvider, nodesAndRoles []machinepools.NodeRoles, psact, kubeVersion, cni string, hardened bool, advancedOptions provisioning.AdvancedOptions) {
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
		if nodes.Windows {
			finalRoleCommand += " --windows"
		}
		quantityPerPool = append(quantityPerPool, nodes.Quantity)
		rolesPerPool = append(rolesPerPool, finalRoleCommand)
		for i := int32(0); i < nodes.Quantity; i++ {
			rolesPerNode = append(rolesPerNode, finalRoleCommand)
		}
	}
	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	require.NoError(t, err)

	nodes, err := externalNodeProvider.NodeCreationFunc(client, rolesPerPool, quantityPerPool)
	require.NoError(t, err)

	clusterName := namegen.AppendRandomString(externalNodeProvider.Name)

	cluster := clusters.NewK3SRKE2ClusterConfig(clusterName, namespace, cni, "", kubeVersion, psact, nil, advancedOptions)

	clusterResp, err := clusters.CreateK3SRKE2Cluster(client, cluster)
	require.NoError(t, err)

	if client.Flags.GetValue(environmentflag.UpdateClusterName) {
		pipeline.UpdateConfigClusterName(clusterName)
	}

	client, err = client.ReLogin()
	require.NoError(t, err)
	customCluster, err := client.Steve.SteveType(clusters.ProvisioningSteveResourceType).ByID(clusterResp.ID)
	require.NoError(t, err)

	clusterStatus := &apiv1.ClusterStatus{}
	err = v1.ConvertToK8sType(customCluster.Status, clusterStatus)
	require.NoError(t, err)

	token, err := tokenregistration.GetRegistrationToken(client, clusterStatus.ClusterName)
	require.NoError(t, err)

	clusterIDName, err := clusters.GetClusterIDByName(adminClient, clusterName)
	assert.NoError(t, err)

	kubeProvisioningClient, err := adminClient.GetKubeAPIProvisioningClient()
	require.NoError(t, err)
	result, err := kubeProvisioningClient.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + clusterName,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	require.NoError(t, err)
	checkFunc := clusters.IsProvisioningClusterReady

	for key, node := range nodes {
		t.Logf("Execute Registration Command for node %s", node.NodeID)
		var command string
		if strings.Contains(rolesPerNode[key], "windows") {
			err = wait.WatchWait(result, checkFunc)
			assert.NoError(t, err)
			assert.Equal(t, clusterName, clusterResp.ObjectMeta.Name)
			t.Logf("Windows pool detected, using powershell.exe")
			command = fmt.Sprintf("powershell.exe %s -Address %s", token.InsecureWindowsNodeCommand, node.PublicIPAddress)
		} else {
			t.Logf("Linux pool detected, using bash")
			command = fmt.Sprintf("%s %s --address %s", token.InsecureNodeCommand, rolesPerNode[key], node.PublicIPAddress)
		}
		t.Logf("Command: %s", command)
		output, err := node.ExecuteCommand(command)
		require.NoError(t, err)

		t.Logf(output)
	}
	result, err = kubeProvisioningClient.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + clusterName,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	require.NoError(t, err)
	err = wait.WatchWait(result, checkFunc)
	assert.NoError(t, err)
	assert.Equal(t, clusterName, clusterResp.ObjectMeta.Name)

	err = nodestat.IsNodeReady(client, clusterIDName)
	require.NoError(t, err)

	clusterToken, err := clusters.CheckServiceAccountTokenSecret(client, clusterName)
	require.NoError(t, err)
	assert.NotEmpty(t, clusterToken)

	if hardened && kubeVersion <= string(provisioning.HardenedKubeVersion) {
		err = hardening.HardeningNodes(client, hardened, nodes, rolesPerNode)
		require.NoError(t, err)

		hardenCluster := clusters.HardenK3SRKE2ClusterConfig(clusterName, namespace, "", "", kubeVersion, psact, nil, provisioning.AdvancedOptions{})

		hardenClusterResp, err := clusters.UpdateK3SRKE2Cluster(client, clusterResp, hardenCluster)
		require.NoError(t, err)
		assert.Equal(t, clusterName, hardenClusterResp.ObjectMeta.Name)

		err = hardening.PostHardeningConfig(client, hardened, nodes, rolesPerNode)
		require.NoError(t, err)
	}

	if psact == string(provisioning.RancherPrivileged) || psact == string(provisioning.RancherRestricted) {
		err = psadeploy.CheckPSACT(client, clusterName)
		require.NoError(t, err)

		_, err = psadeploy.CreateNginxDeployment(client, clusterIDName, psact)
		require.NoError(t, err)
	}

	podResults, podErrors := pods.StatusPods(client, clusterIDName)
	assert.NotEmpty(t, podResults)
	assert.Empty(t, podErrors)
}
