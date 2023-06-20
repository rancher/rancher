package airgap

import (
	"context"
	"fmt"
	"testing"

	apiv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/corral"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
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
	rke2k3sAirgapCustomCluster = "rke2k3sairgapcustomcluster"
	rke2k3sNodeCorralName      = "rke2k3sregisterNode"
)

func testProvisionAirgapRKE2K3SCustomCluster(t *testing.T, client *rancher.Client, nodesAndRoles map[int]string, corralImage, cni, kubeVersion string, cleanup bool, advancedOptions provisioning.AdvancedOptions) string {
	namespace := "fleet-default"

	clusterName := namegen.AppendRandomString(rke2k3sAirgapCustomCluster)

	cluster := clusters.NewK3SRKE2ClusterConfig(clusterName, namespace, cni, "", kubeVersion, "", nil, advancedOptions)

	clusterResp, err := clusters.CreateK3SRKE2Cluster(client, cluster)
	require.NoError(t, err)

	client, err = client.ReLogin()
	require.NoError(t, err)
	customCluster, err := client.Steve.SteveType(clusters.ProvisioningSteveResourceType).ByID(clusterResp.ID)
	require.NoError(t, err)

	clusterStatus := &apiv1.ClusterStatus{}
	err = v1.ConvertToK8sType(customCluster.Status, clusterStatus)
	require.NoError(t, err)

	token, err := tokenregistration.GetRegistrationToken(client, clusterStatus.ClusterName)
	require.NoError(t, err)

	t.Logf("Register Custom Cluster Through Corral")
	for numNodes, roles := range nodesAndRoles {
		err = corral.UpdateCorralConfig("node_count", fmt.Sprint(numNodes))
		require.NoError(t, err)

		command := fmt.Sprintf("%s %s", token.InsecureNodeCommand, roles)
		t.Logf("registration command is %s", command)
		err = corral.UpdateCorralConfig("registration_command", command)
		require.NoError(t, err)

		corralName := namegen.AppendRandomString(rke2k3sNodeCorralName)
		_, err = corral.CreateCorral(client.Session, corralName, corralImage, true, cleanup)
		require.NoError(t, err)
	}

	kubeProvisioningClient, err := client.GetKubeAPIProvisioningClient()
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

	clusterToken, err := clusters.CheckServiceAccountTokenSecret(client, clusterName)
	require.NoError(t, err)
	assert.NotEmpty(t, clusterToken)

	podResults, podErrors := pods.StatusPods(client, clusterIDName)
	assert.NotEmpty(t, podResults)
	assert.Empty(t, podErrors)

	return clusterName
}

func validateRKE2K3SKubernetesUpgrade(t *testing.T, updatedCluster *bundledclusters.BundledCluster, upgradedVersion string) {
	clusterSpec := &apiv1.ClusterSpec{}
	err := v1.ConvertToK8sType(updatedCluster.V1.Spec, clusterSpec)
	require.NoError(t, err)

	assert.Equalf(t, upgradedVersion, clusterSpec.KubernetesVersion, "[%v]: %v", updatedCluster.Meta.Name, logMessageKubernetesVersion)
}
