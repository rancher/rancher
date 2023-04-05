package rke1

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	nodestat "github.com/rancher/rancher/tests/framework/extensions/nodes"
	nodepools "github.com/rancher/rancher/tests/framework/extensions/rke1/nodepools"
	"github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestProvisioningRKE1Cluster(t *testing.T, client *rancher.Client, provider Provider, nodesAndRoles []nodepools.NodeRoles, kubeVersion, cni string, nodeTemplate *nodetemplates.NodeTemplate) (*management.Cluster, error) {
	clusterName := namegen.AppendRandomString(provider.Name)
	cluster := clusters.NewRKE1ClusterConfig(clusterName, cni, kubeVersion, client)
	clusterResp, err := clusters.CreateRKE1Cluster(client, cluster)
	require.NoError(t, err)

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

	podResults, podErrors := pods.StatusPods(client, clusterResp.ID)
	assert.NotEmpty(t, podResults)
	assert.Empty(t, podErrors)

	return clusterResp, nil
}
