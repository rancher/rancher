package rke2

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	steveV1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	nodestat "github.com/rancher/rancher/tests/framework/extensions/nodes"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	provisioning "github.com/rancher/rancher/tests/v2/validation/provisioning"
	name2 "github.com/rancher/wrangler/pkg/name"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

func TestHostnameTruncation(t *testing.T, client *rancher.Client, provider Provider, machinePools []provv1.RKEMachinePool, defaultHostnameLengthLimit int, kubeVersion, cni string, advancedOptions provisioning.AdvancedOptions) {
	cloudCredential, err := provider.CloudCredFunc(client)
	require.NoError(t, err)

	generatedPoolName := "nc-"
	machinePoolConfig := provider.MachinePoolFunc(generatedPoolName, namespace)

	machineConfigResp, err := client.Steve.SteveType(provider.MachineConfigPoolResourceSteveType).Create(machinePoolConfig)
	require.NoError(t, err)

	for i := range machinePools {
		machinePools[i].NodeConfig = &corev1.ObjectReference{
			Kind: machineConfigResp.Kind,
			Name: machineConfigResp.Name,
		}
		machinePools[i].EtcdRole = true
		machinePools[i].ControlPlaneRole = true
		machinePools[i].WorkerRole = true
	}

	cluster := clusters.NewK3SRKE2ClusterConfig("", namespace, cni, cloudCredential.ID, kubeVersion, "", machinePools, advancedOptions)

	cluster.GenerateName = "t-"
	cluster.Spec.RKEConfig.MachinePoolDefaults.HostnameLengthLimit = defaultHostnameLengthLimit
	clusterResp, err := clusters.CreateK3SRKE2Cluster(client, cluster)
	require.NoError(t, err)

	require.NoError(t, steveV1.ConvertToK8sType(clusterResp.JSONResp, cluster))
	clusterName := cluster.Name
	assert.True(t, len(clusterName) == 7)

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

	for _, mp := range cluster.Spec.RKEConfig.MachinePools {
		n := name2.SafeConcatName(cluster.Name, mp.Name)
		query, err := url.ParseQuery(fmt.Sprintf("labelSelector=%s=%s&fieldSelector=metadata.name=%s", capi.ClusterLabelName, clusterName, n))
		require.NoError(t, err)

		machineDeploymentsResp, err := client.Steve.SteveType("cluster.x-k8s.io.machinedeployment").List(query)
		require.NoError(t, err)

		assert.True(t, len(machineDeploymentsResp.Data) == 1)

		md := &capi.MachineDeployment{}
		require.NoError(t, steveV1.ConvertToK8sType(machineDeploymentsResp.Data[0].JSONResp, md))

		query2, err := url.ParseQuery(fmt.Sprintf("labelSelector=%s=%s", capi.MachineDeploymentLabelName, md.Name))
		require.NoError(t, err)

		machineResp, err := client.Steve.SteveType("cluster.x-k8s.io.machine").List(query2)
		require.NoError(t, err)

		assert.True(t, len(machineResp.Data) > 0)

		for i := range machineResp.Data {
			m := capi.Machine{}
			require.NoError(t, steveV1.ConvertToK8sType(machineResp.Data[i].JSONResp, &m))

			assert.NotNil(t, m.Status.NodeRef)

			dynamic, err := client.GetRancherDynamicClient()
			require.NoError(t, err)

			gv, err := schema.ParseGroupVersion(m.Spec.InfrastructureRef.APIVersion)
			require.NoError(t, err)

			gvr := schema.GroupVersionResource{
				Group:    gv.Group,
				Version:  gv.Version,
				Resource: strings.ToLower(m.Spec.InfrastructureRef.Kind) + "s",
			}

			ustr, err := dynamic.Resource(gvr).Namespace(m.Namespace).Get(context.TODO(), m.Spec.InfrastructureRef.Name, metav1.GetOptions{})
			require.NoError(t, err)

			limit := 63
			if mp.HostnameLengthLimit != 0 {
				limit = mp.HostnameLengthLimit
			} else if cluster.Spec.RKEConfig.MachinePoolDefaults.HostnameLengthLimit != 0 {
				limit = cluster.Spec.RKEConfig.MachinePoolDefaults.HostnameLengthLimit
			}

			assert.True(t, len(m.Status.NodeRef.Name) <= limit)
			if len(ustr.GetName()) < limit {
				assert.True(t, m.Status.NodeRef.Name == ustr.GetName())
			}
		}
	}
}
