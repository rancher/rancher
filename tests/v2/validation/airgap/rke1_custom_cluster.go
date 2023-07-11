package airgap

import (
	"fmt"
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/corral"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/bundledclusters"
	"github.com/rancher/rancher/tests/framework/extensions/defaults"
	nodestat "github.com/rancher/rancher/tests/framework/extensions/nodes"
	"github.com/rancher/rancher/tests/framework/extensions/tokenregistration"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/v2/validation/provisioning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	rke1AirgapCustomCluster = "rke1airgapcustomcluster"
	rke1NodeCorralName      = "rke1registerNode"
)

func testProvisioningRKE1CustomCluster(t *testing.T, client *rancher.Client, nodesAndRoles map[int]string, corralImage, cni, kubeVersion, registryFQDN string, cleanup bool, advancedOptions provisioning.AdvancedOptions) string {
	clusterName := namegen.AppendRandomString(rke1AirgapCustomCluster)

	cluster := clusters.NewRKE1ClusterConfig(clusterName, cni, kubeVersion, "", client, advancedOptions)
	clusterResp, err := clusters.CreateRKE1Cluster(client, cluster)
	require.NoError(t, err)

	client, err = client.ReLogin()
	require.NoError(t, err)

	customCluster, err := client.Management.Cluster.ByID(clusterResp.ID)
	require.NoError(t, err)

	token, err := tokenregistration.GetRegistrationToken(client, customCluster.ID)
	require.NoError(t, err)

	t.Logf("Register Custom Cluster Through Corral")
	for numNodes, roles := range nodesAndRoles {
		err = corral.UpdateCorralConfig("node_count", fmt.Sprint(numNodes))
		require.NoError(t, err)

		command := fmt.Sprintf("%s %s", token.NodeCommand, roles)
		t.Logf("registration command is %s", command)
		err = corral.UpdateCorralConfig("registration_command", command)
		require.NoError(t, err)
		corralName := namegen.AppendRandomString(rke1NodeCorralName)

		_, err = corral.CreateCorral(client.Session, corralName, corralImage, true, cleanup)
		require.NoError(t, err)
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

	podResults, podErrors := pods.StatusPods(client, clusterResp.ID)
	assert.NotEmpty(t, podResults)
	assert.Empty(t, podErrors)

	return clusterName
}

func validateRKE1KubernetesUpgrade(t *testing.T, updatedCluster *bundledclusters.BundledCluster, upgradedVersion string) {
	assert.Equalf(t, upgradedVersion, updatedCluster.V3.RancherKubernetesEngineConfig.Version, "[%v]: %v", updatedCluster.Meta.Name, logMessageKubernetesVersion)
}
