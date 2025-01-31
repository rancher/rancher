package provisioning

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/clusters"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"

	psadeploy "github.com/rancher/rancher/tests/v2/actions/psact"
	"github.com/rancher/rancher/tests/v2/actions/registries"
	"github.com/rancher/rancher/tests/v2/actions/reports"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	shepherdclusters "github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/clusters/bundledclusters"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/extensions/kubeconfig"
	nodestat "github.com/rancher/shepherd/extensions/nodes"
	"github.com/rancher/shepherd/extensions/sshkeys"
	"github.com/rancher/shepherd/extensions/workloads/pods"
	"github.com/rancher/shepherd/pkg/wait"
	wranglername "github.com/rancher/wrangler/pkg/name"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	local                       = "local"
	logMessageKubernetesVersion = "Validating the current version is the upgraded one"
	hostnameLimit               = 63
	etcdSnapshotAnnotation      = "etcdsnapshot.rke.io/storage"
	machineNameAnnotation       = "cluster.x-k8s.io/machine"
	machineSteveResourceType    = "cluster.x-k8s.io.machine"
	onDemandPrefix              = "on-demand-"
	s3                          = "s3"
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

	checkFunc := shepherdclusters.IsHostedProvisioningClusterReady
	err = wait.WatchWait(watchInterface, checkFunc)
	require.NoError(t, err)

	assert.Equal(t, clustersConfig.KubernetesVersion, cluster.RancherKubernetesEngineConfig.Version)

	clusterToken, err := clusters.CheckServiceAccountTokenSecret(client, cluster.Name)
	reports.TimeoutRKEReport(cluster, err)
	require.NoError(t, err)
	assert.NotEmpty(t, clusterToken)

	err = nodestat.AllManagementNodeReady(client, cluster.ID, defaults.ThirtyMinuteTimeout)
	reports.TimeoutRKEReport(cluster, err)
	require.NoError(t, err)

	if clustersConfig.PSACT == string(provisioninginput.RancherPrivileged) || clustersConfig.PSACT == string(provisioninginput.RancherRestricted) || clustersConfig.PSACT == string(provisioninginput.RancherBaseline) {
		require.NotEmpty(t, cluster.DefaultPodSecurityAdmissionConfigurationTemplateName)

		err := psadeploy.CreateNginxDeployment(client, cluster.ID, clustersConfig.PSACT)
		reports.TimeoutRKEReport(cluster, err)
		require.NoError(t, err)
	}
	if clustersConfig.Registries != nil {
		if clustersConfig.Registries.RKE1Registries != nil {
			for _, registry := range clustersConfig.Registries.RKE1Registries {
				havePrefix, err := registries.CheckAllClusterPodsForRegistryPrefix(client, cluster.ID, registry.URL)
				reports.TimeoutRKEReport(cluster, err)
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

	if clustersConfig.CloudProvider == "" {
		podErrors := pods.StatusPods(client, cluster.ID)
		assert.Empty(t, podErrors)
	}
}

// VerifyCluster validates that a non-rke1 cluster and its resources are in a good state, matching a given config.
func VerifyCluster(t *testing.T, client *rancher.Client, clustersConfig *clusters.ClusterConfig, cluster *steveV1.SteveAPIObject) {
	client, err := client.ReLogin()
	require.NoError(t, err)

	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	require.NoError(t, err)

	kubeProvisioningClient, err := adminClient.GetKubeAPIProvisioningClient()
	reports.TimeoutClusterReport(cluster, err)
	require.NoError(t, err)

	watchInterface, err := kubeProvisioningClient.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + cluster.Name,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	reports.TimeoutClusterReport(cluster, err)
	require.NoError(t, err)

	checkFunc := shepherdclusters.IsProvisioningClusterReady
	err = wait.WatchWait(watchInterface, checkFunc)
	reports.TimeoutClusterReport(cluster, err)
	require.NoError(t, err)

	clusterToken, err := clusters.CheckServiceAccountTokenSecret(client, cluster.Name)
	reports.TimeoutClusterReport(cluster, err)
	require.NoError(t, err)
	assert.NotEmpty(t, clusterToken)

	err = nodestat.AllMachineReady(client, cluster.ID, defaults.ThirtyMinuteTimeout)
	reports.TimeoutClusterReport(cluster, err)
	require.NoError(t, err)

	status := &provv1.ClusterStatus{}
	err = steveV1.ConvertToK8sType(cluster.Status, status)
	reports.TimeoutClusterReport(cluster, err)
	require.NoError(t, err)

	clusterSpec := &provv1.ClusterSpec{}
	err = steveV1.ConvertToK8sType(cluster.Spec, clusterSpec)
	reports.TimeoutClusterReport(cluster, err)
	require.NoError(t, err)

	configKubeVersion := clusterSpec.KubernetesVersion
	require.Equal(t, configKubeVersion, clusterSpec.KubernetesVersion)

	if clusterSpec.DefaultPodSecurityAdmissionConfigurationTemplateName == string(provisioninginput.RancherPrivileged) ||
		clusterSpec.DefaultPodSecurityAdmissionConfigurationTemplateName == string(provisioninginput.RancherRestricted) ||
		clusterSpec.DefaultPodSecurityAdmissionConfigurationTemplateName == string(provisioninginput.RancherBaseline) {

		require.NotEmpty(t, clusterSpec.DefaultPodSecurityAdmissionConfigurationTemplateName)

		err := psadeploy.CreateNginxDeployment(client, status.ClusterName, clusterSpec.DefaultPodSecurityAdmissionConfigurationTemplateName)
		reports.TimeoutClusterReport(cluster, err)
		require.NoError(t, err)
	}

	if clusterSpec.RKEConfig.Registries != nil {
		for registryName := range clusterSpec.RKEConfig.Registries.Configs {
			havePrefix, err := registries.CheckAllClusterPodsForRegistryPrefix(client, status.ClusterName, registryName)
			reports.TimeoutClusterReport(cluster, err)
			require.NoError(t, err)
			assert.True(t, havePrefix)
		}
	}

	if clusterSpec.LocalClusterAuthEndpoint.Enabled {
		mgmtClusterObject, err := adminClient.Management.Cluster.ByID(status.ClusterName)
		reports.TimeoutClusterReport(cluster, err)
		require.NoError(t, err)
		VerifyACE(t, adminClient, mgmtClusterObject)
	}

	podErrors := pods.StatusPods(client, status.ClusterName)
	assert.Empty(t, podErrors)

	if clustersConfig != nil {
		if clustersConfig.ClusterSSHTests != nil {
			VerifySSHTests(t, client, cluster, clustersConfig.ClusterSSHTests, status.ClusterName)
		}
	}
}

// VerifyHostedCluster validates that the hosted cluster and its resources are in a good state, matching a given config.
func VerifyHostedCluster(t *testing.T, client *rancher.Client, cluster *management.Cluster) {
	client, err := client.ReLogin()
	require.NoError(t, err)

	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	require.NoError(t, err)

	watchInterface, err := adminClient.GetManagementWatchInterface(management.ClusterType, metav1.ListOptions{
		FieldSelector:  "metadata.name=" + cluster.ID,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	reports.TimeoutRKEReport(cluster, err)
	require.NoError(t, err)

	checkFunc := shepherdclusters.IsHostedProvisioningClusterReady

	err = wait.WatchWait(watchInterface, checkFunc)
	reports.TimeoutRKEReport(cluster, err)
	require.NoError(t, err)

	clusterToken, err := clusters.CheckServiceAccountTokenSecret(client, cluster.Name)
	reports.TimeoutRKEReport(cluster, err)
	require.NoError(t, err)
	assert.NotEmpty(t, clusterToken)

	err = nodestat.AllManagementNodeReady(client, cluster.ID, defaults.ThirtyMinuteTimeout)
	reports.TimeoutRKEReport(cluster, err)
	require.NoError(t, err)

	podErrors := pods.StatusPods(client, cluster.ID)
	assert.Empty(t, podErrors)
}

// VerifyDeleteRKE1Cluster validates that a rke1 cluster and its resources are deleted.
func VerifyDeleteRKE1Cluster(t *testing.T, client *rancher.Client, clusterID string) {
	cluster, err := client.Management.Cluster.ByID(clusterID)
	require.NoError(t, err)

	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	require.NoError(t, err)

	watchInterface, err := adminClient.GetManagementWatchInterface(management.ClusterType, metav1.ListOptions{
		FieldSelector:  "metadata.name=" + clusterID,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	require.NoError(t, err)

	err = wait.WatchWait(watchInterface, func(event watch.Event) (ready bool, err error) {
		if event.Type == watch.Error {
			return false, fmt.Errorf("error: unable to delete cluster %s", cluster.Name)
		} else if event.Type == watch.Deleted {
			logrus.Infof("Cluster %s deleted!", cluster.Name)
			return true, nil
		}
		return false, nil
	})
	require.NoError(t, err)

	err = nodestat.AllNodeDeleted(client, clusterID)
	require.NoError(t, err)
}

// VerifyDeleteRKE2K3SCluster validates that a non-rke1 cluster and its resources are deleted.
func VerifyDeleteRKE2K3SCluster(t *testing.T, client *rancher.Client, clusterID string) {
	cluster, err := client.Steve.SteveType("provisioning.cattle.io.cluster").ByID(clusterID)
	require.NoError(t, err)

	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	require.NoError(t, err)

	provKubeClient, err := adminClient.GetKubeAPIProvisioningClient()
	require.NoError(t, err)

	watchInterface, err := provKubeClient.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + cluster.Name,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	require.NoError(t, err)

	err = wait.WatchWait(watchInterface, func(event watch.Event) (ready bool, err error) {
		cluster := event.Object.(*provv1.Cluster)
		if event.Type == watch.Error {
			return false, fmt.Errorf("error: unable to delete cluster %s", cluster.ObjectMeta.Name)
		} else if event.Type == watch.Deleted {
			logrus.Infof("Cluster %s deleted!", cluster.ObjectMeta.Name)
			return true, nil
		} else if cluster == nil {
			logrus.Info("Cluster deleted!")
			return true, nil
		}
		return false, nil
	})
	require.NoError(t, err)

	err = nodestat.AllNodeDeleted(client, clusterID)
	require.NoError(t, err)
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
		query, err := url.ParseQuery(fmt.Sprintf("labelSelector=%s=%s&fieldSelector=metadata.name=%s", capi.ClusterNameLabel, clusterObject.Name, n))
		require.NoError(t, err)

		machineDeploymentsResp, err := client.Steve.SteveType("cluster.x-k8s.io.machinedeployment").List(query)
		require.NoError(t, err)

		assert.True(t, len(machineDeploymentsResp.Data) == 1)

		md := &capi.MachineDeployment{}
		require.NoError(t, steveV1.ConvertToK8sType(machineDeploymentsResp.Data[0].JSONResp, md))

		query2, err := url.ParseQuery(fmt.Sprintf("labelSelector=%s=%s", capi.MachineDeploymentNameLabel, md.Name))
		require.NoError(t, err)

		machineResp, err := client.Steve.SteveType(machineSteveResourceType).List(query2)
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

// VerifySSHTests validates the ssh tests listed in the config on each node of the cluster
func VerifySSHTests(t *testing.T, client *rancher.Client, clusterObject *steveV1.SteveAPIObject, sshTests []provisioninginput.SSHTestCase, clusterID string) {
	client, err := client.ReLogin()
	require.NoError(t, err)

	steveClient, err := client.Steve.ProxyDownstream(clusterID)
	require.NoError(t, err)

	nodesSteveObjList, err := steveClient.SteveType("node").List(nil)
	require.NoError(t, err)

	sshUser, err := sshkeys.GetSSHUser(client, clusterObject)
	require.NoError(t, err)

	for _, tests := range sshTests {
		for _, machine := range nodesSteveObjList.Data {
			clusterNode, err := sshkeys.GetSSHNodeFromMachine(client, sshUser, &machine)
			require.NoError(t, err)

			machineName := machine.Annotations[machineNameAnnotation]
			err = CallSSHTestByName(tests, clusterNode, client, clusterID, machineName)
			require.NoError(t, err)

		}
	}
}
