package rke1

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/defaults"
	nodestat "github.com/rancher/rancher/tests/framework/extensions/nodes"
	"github.com/rancher/rancher/tests/framework/extensions/pipeline"
	psadeploy "github.com/rancher/rancher/tests/framework/extensions/psact"
	nodepools "github.com/rancher/rancher/tests/framework/extensions/rke1/nodepools"
	"github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	"github.com/rancher/rancher/tests/framework/pkg/environmentflag"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/v2/validation/provisioning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestProvisioningRKE1Cluster(t *testing.T, client *rancher.Client, provider Provider, nodesAndRoles []nodepools.NodeRoles, psact string, kubeVersion, cni string, nodeTemplate *nodetemplates.NodeTemplate, advancedOptions provisioning.AdvancedOptions) (*management.Cluster, error) {
	clusterName := namegen.AppendRandomString(provider.Name.String())
	cluster := clusters.NewRKE1ClusterConfig(clusterName, cni, kubeVersion, psact, client, advancedOptions)
	clusterResp, err := clusters.CreateRKE1Cluster(client, cluster)
	require.NoError(t, err)

	if client.Flags.GetValue(environmentflag.UpdateClusterName) {
		pipeline.UpdateConfigClusterName(clusterName)
	}

	nodePool, err := nodepools.NodePoolSetup(client, nodesAndRoles, clusterResp.ID, nodeTemplate.ID)
	require.NoError(t, err)

	nodePoolName := nodePool.Name

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
	assert.Equal(t, nodePoolName, nodePool.Name)
	assert.Equal(t, kubeVersion, clusterResp.RancherKubernetesEngineConfig.Version)

	err = nodestat.IsNodeReady(client, clusterResp.ID)
	require.NoError(t, err)

	clusterToken, err := clusters.CheckServiceAccountTokenSecret(client, clusterName)
	require.NoError(t, err)
	assert.NotEmpty(t, clusterToken)

	if psact == string(provisioning.RancherPrivileged) || psact == string(provisioning.RancherRestricted) {
		err = psadeploy.CheckPSACT(client, clusterName)
		require.NoError(t, err)

		_, err = psadeploy.CreateNginxDeployment(client, clusterResp.ID, psact)
		require.NoError(t, err)
	}

	podResults, podErrors := pods.StatusPods(client, clusterResp.ID)
	assert.NotEmpty(t, podResults)
	assert.Empty(t, podErrors)

	return clusterResp, nil
}
