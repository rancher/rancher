package provisioning

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	steveV1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/bundledclusters"
	"github.com/rancher/rancher/tests/framework/extensions/kubeconfig"
	nodestat "github.com/rancher/rancher/tests/framework/extensions/nodes"
	"github.com/rancher/rancher/tests/framework/extensions/provisioninginput"
	psadeploy "github.com/rancher/rancher/tests/framework/extensions/psact"
	"github.com/rancher/rancher/tests/framework/extensions/registries"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	wranglername "github.com/rancher/wrangler/pkg/name"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	logMessageKubernetesVersion = "Validating the current version is the upgraded one"
	hostnameLimit               = 63
)

// VerifyRKE1Cluster validates that the RKE1 cluster and its resources are in a good state, matching a given config.
func VerifyRKE1Cluster(t *testing.T, client *rancher.Client, clustersConfig *clusters.ClusterConfig, cluster *management.Cluster) {
	client, err := client.ReLogin()
	require.NoError(t, err)

	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	require.NoError(t, err)

	watchInterface, err := adminClient.GetManagementWatchInterface(management.ClusterType, metav1.ListOptions{
		FieldSelector:  "metadata.name=" + cluster.ID,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	require.NoError(t, err)

	checkFunc := clusters.IsHostedProvisioningClusterReady
	err = wait.WatchWait(watchInterface, checkFunc)
	require.NoError(t, err)

	assert.Equal(t, clustersConfig.KubernetesVersion, cluster.RancherKubernetesEngineConfig.Version)

	clusterToken, err := clusters.CheckServiceAccountTokenSecret(client, cluster.Name)
	require.NoError(t, err)
	assert.NotEmpty(t, clusterToken)

	err = nodestat.AllManagementNodeReady(client, cluster.ID)
	require.NoError(t, err)

	if clustersConfig.PSACT == string(provisioninginput.RancherPrivileged) || clustersConfig.PSACT == string(provisioninginput.RancherRestricted) || clustersConfig.PSACT == string(provisioninginput.RancherBaseline) {
		require.NotEmpty(t, cluster.DefaultPodSecurityAdmissionConfigurationTemplateName)

		err := psadeploy.CreateNginxDeployment(client, cluster.ID, clustersConfig.PSACT)
		require.NoError(t, err)
	}
	if clustersConfig.Registries != nil {
		if clustersConfig.Registries.RKE1Registries != nil {
			for _, registry := range clustersConfig.Registries.RKE1Registries {
				havePrefix, err := registries.CheckAllClusterPodsForRegistryPrefix(client, cluster.ID, registry.URL)
				require.NoError(t, err)
				assert.True(t, havePrefix)
			}
		}
	}
	if clustersConfig.Networking != nil {
		if clustersConfig.Networking.LocalClusterAuthEndpoint != nil {
			VerifyACE(t, adminClient, cluster)
		}
	}

	podResults, podErrors := pods.StatusPods(client, cluster.ID)
	assert.NotEmpty(t, podResults)
	assert.Empty(t, podErrors)
}

// VerifyCluster validates that a non-rke1 cluster and its resources are in a good state, matching a given config.
func VerifyCluster(t *testing.T, client *rancher.Client, cluster *steveV1.SteveAPIObject) {
	client, err := client.ReLogin()
	require.NoError(t, err)

	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	require.NoError(t, err)

	kubeProvisioningClient, err := adminClient.GetKubeAPIProvisioningClient()
	require.NoError(t, err)

	watchInterface, err := kubeProvisioningClient.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + cluster.Name,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	require.NoError(t, err)

	checkFunc := clusters.IsProvisioningClusterReady
	err = wait.WatchWait(watchInterface, checkFunc)
	require.NoError(t, err)

	clusterToken, err := clusters.CheckServiceAccountTokenSecret(client, cluster.Name)
	require.NoError(t, err)
	assert.NotEmpty(t, clusterToken)

	err = nodestat.AllMachineReady(client, cluster.ID)
	require.NoError(t, err)

	status := &provv1.ClusterStatus{}
	err = steveV1.ConvertToK8sType(cluster.Status, status)
	require.NoError(t, err)

	clusterSpec := &provv1.ClusterSpec{}
	err = steveV1.ConvertToK8sType(cluster.Spec, clusterSpec)
	require.NoError(t, err)

	configKubeVersion := clusterSpec.KubernetesVersion
	require.Equal(t, configKubeVersion, clusterSpec.KubernetesVersion)

	if clusterSpec.DefaultPodSecurityAdmissionConfigurationTemplateName == string(provisioninginput.RancherPrivileged) ||
		clusterSpec.DefaultPodSecurityAdmissionConfigurationTemplateName == string(provisioninginput.RancherRestricted) ||
		clusterSpec.DefaultPodSecurityAdmissionConfigurationTemplateName == string(provisioninginput.RancherBaseline) {

		require.NotEmpty(t, clusterSpec.DefaultPodSecurityAdmissionConfigurationTemplateName)

		err := psadeploy.CreateNginxDeployment(client, status.ClusterName, clusterSpec.DefaultPodSecurityAdmissionConfigurationTemplateName)
		require.NoError(t, err)
	}

	if clusterSpec.RKEConfig.Registries != nil {
		for registryName := range clusterSpec.RKEConfig.Registries.Configs {
			havePrefix, err := registries.CheckAllClusterPodsForRegistryPrefix(client, status.ClusterName, registryName)
			require.NoError(t, err)
			assert.True(t, havePrefix)
		}
	}

	if clusterSpec.LocalClusterAuthEndpoint.Enabled {
		mgmtClusterObject, err := adminClient.Management.Cluster.ByID(status.ClusterName)
		require.NoError(t, err)
		VerifyACE(t, adminClient, mgmtClusterObject)
	}

	podResults, podErrors := pods.StatusPods(client, status.ClusterName)
	assert.Empty(t, podErrors)
	assert.NotEmpty(t, podResults)
}

// CertRotationCompleteCheckFunc returns a watch check function that checks if the certificate rotation is complete
func CertRotationCompleteCheckFunc(generation int64) wait.WatchCheckFunc {
	return func(event watch.Event) (bool, error) {
		controlPlane := event.Object.(*rkev1.RKEControlPlane)
		return controlPlane.Status.CertificateRotationGeneration == generation, nil
	}
}

// VerifyACE validates that the ACE resources are healthy in a given cluster
func VerifyACE(t *testing.T, client *rancher.Client, cluster *management.Cluster) {
	client, err := client.ReLogin()
	require.NoError(t, err)

	kubeConfig, err := kubeconfig.GetKubeconfig(client, cluster.ID)
	require.NoError(t, err)

	original, err := client.SwitchContext(cluster.Name, kubeConfig)
	require.NoError(t, err)

	originalResp, err := original.Resource(corev1.SchemeGroupVersion.WithResource("pods")).Namespace("").List(context.TODO(), metav1.ListOptions{})
	require.NoError(t, err)
	for _, pod := range originalResp.Items {
		t.Logf("Pod %v", pod.GetName())
	}

	// each control plane has a context. For ACE, we should check these contexts
	contexts, err := kubeconfig.GetContexts(kubeConfig)
	require.NoError(t, err)
	var contextNames []string
	for context := range contexts {
		if strings.Contains(context, "pool") {
			contextNames = append(contextNames, context)
		}
	}

	for _, contextName := range contextNames {
		dynamic, err := client.SwitchContext(contextName, kubeConfig)
		assert.NoError(t, err)
		resp, err := dynamic.Resource(corev1.SchemeGroupVersion.WithResource("pods")).Namespace("").List(context.TODO(), metav1.ListOptions{})
		assert.NoError(t, err)
		t.Logf("Switched Context to %v", contextName)
		for _, pod := range resp.Items {
			t.Logf("Pod %v", pod.GetName())
		}
	}
}

// VerifyHostnameLength validates that the hostnames of the nodes in a cluster are of the correct length
func VerifyHostnameLength(t *testing.T, client *rancher.Client, clusterObject *steveV1.SteveAPIObject) {
	client, err := client.ReLogin()
	require.NoError(t, err)

	clusterSpec := &provv1.ClusterSpec{}
	err = steveV1.ConvertToK8sType(clusterObject.Spec, clusterSpec)
	require.NoError(t, err)

	for _, mp := range clusterSpec.RKEConfig.MachinePools {
		n := wranglername.SafeConcatName(clusterObject.Name, mp.Name)
		query, err := url.ParseQuery(fmt.Sprintf("labelSelector=%s=%s&fieldSelector=metadata.name=%s", capi.ClusterLabelName, clusterObject.Name, n))
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

			limit := hostnameLimit
			if mp.HostnameLengthLimit != 0 {
				limit = mp.HostnameLengthLimit
			} else if clusterSpec.RKEConfig.MachinePoolDefaults.HostnameLengthLimit != 0 {
				limit = clusterSpec.RKEConfig.MachinePoolDefaults.HostnameLengthLimit
			}

			assert.True(t, len(m.Status.NodeRef.Name) <= limit)
			if len(ustr.GetName()) < limit {
				assert.True(t, m.Status.NodeRef.Name == ustr.GetName())
			}
		}
		t.Logf("Verified hostname length for machine pool %s", mp.Name)
	}
}

// VerifyUpgrade validates that a cluster has been upgraded to a given version
func VerifyUpgrade(t *testing.T, updatedCluster *bundledclusters.BundledCluster, upgradedVersion string) {
	if updatedCluster.V3 != nil {
		assert.Equalf(t, upgradedVersion, updatedCluster.V3.RancherKubernetesEngineConfig.Version, "[%v]: %v", updatedCluster.Meta.Name, logMessageKubernetesVersion)
	} else {
		clusterSpec := &provv1.ClusterSpec{}
		err := steveV1.ConvertToK8sType(updatedCluster.V1.Spec, clusterSpec)
		require.NoError(t, err)
		assert.Equalf(t, upgradedVersion, clusterSpec.KubernetesVersion, "[%v]: %v", updatedCluster.Meta.Name, logMessageKubernetesVersion)
	}
}

// VerifySnapshots waits for a cluster's snapshots to be ready and validates that the correct number of snapshots have been taken
func VerifySnapshots(client *rancher.Client, localclusterID string, clusterName string, expectedSnapshotLength int) (string, error) {
	client, err := client.ReLogin()
	if err != nil {
		return "", err
	}
	var snapshotToBeRestored string

	err = kwait.Poll(5*time.Second, 5*time.Minute, func() (done bool, err error) {
		snapshotList, err := GetSnapshots(client, localclusterID, clusterName)
		if err != nil {
			return false, err
		}
		if len(snapshotList) == 0 {
			return false, fmt.Errorf("no snapshots found")
		}

		if len(snapshotList) == expectedSnapshotLength {
			snapshotToBeRestored = snapshotList[0]
			return true, nil
		}
		if len(snapshotList) > expectedSnapshotLength {
			return false, fmt.Errorf("more snapshots than expected")
		}

		return false, nil
	})
	return snapshotToBeRestored, err
}

// getSnapshots is a helper function to get the snapshots for a cluster
func GetSnapshots(client *rancher.Client, localclusterID string, clusterName string) ([]string, error) {
	steveclient, err := client.Steve.ProxyDownstream(localclusterID)
	if err != nil {
		return nil, err
	}
	snapshotSteveObjList, err := steveclient.SteveType("rke.cattle.io.etcdsnapshot").List(nil)
	if err != nil {
		return nil, err
	}
	snapshots := []string{}
	for _, snapshot := range snapshotSteveObjList.Data {
		if strings.Contains(snapshot.ObjectMeta.Name, clusterName) {
			snapshots = append(snapshots, snapshot.Name)
		}
	}
	return snapshots, nil

}
