package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/api/scheme"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/cloudcredentials"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	provisioning "github.com/rancher/rancher/tests/v2/validation/provisioning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	namespace = "fleet-default"
)

type RKE2EtcdSnapshotRestoreTestSuite struct {
	suite.Suite
	session            *session.Session
	client             *rancher.Client
	clusterName        string
	namespace          string
	kubernetesVersions []string
	cnis               []string
	providers          []string
	nodesAndRoles      []machinepools.NodeRoles
}

func (p *RKE2EtcdSnapshotRestoreTestSuite) TearDownSuite() {
	p.session.Cleanup()
}

var EtcdSnapshotGroupVersionResource = schema.GroupVersionResource{
	Group:    "rke.cattle.io",
	Version:  "v1",
	Resource: "etcdsnapshots",
}

func (r *RKE2EtcdSnapshotRestoreTestSuite) SetupSuite() {
	testSession := session.NewSession(r.T())
	r.session = testSession

	clustersConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, clustersConfig)

	r.kubernetesVersions = clustersConfig.KubernetesVersions
	r.cnis = clustersConfig.CNIs
	r.providers = clustersConfig.Providers
	r.nodesAndRoles = clustersConfig.NodesAndRoles

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)

	r.client = client

	r.clusterName = r.client.RancherConfig.ClusterName
	r.namespace = r.client.RancherConfig.ClusterName
}

func (r *RKE2EtcdSnapshotRestoreTestSuite) TestEtcdSnapshotRestoreFreshCluster(provider Provider, kubeVersion string, cni string, nodesAndRoles []machinepools.NodeRoles, credential *cloudcredentials.CloudCredential) {
	name := fmt.Sprintf("Provider_%s/Kubernetes_Version_%s/Nodes_%v", provider.Name, kubeVersion, nodesAndRoles)
	r.Run(name, func() {
		testSession := session.NewSession(r.T())
		defer testSession.Cleanup()

		testSessionClient, err := r.client.WithSession(testSession)
		require.NoError(r.T(), err)

		clusterName := provisioning.AppendRandomString(fmt.Sprintf("%s-%s", r.clusterName, provider.Name))
		generatedPoolName := fmt.Sprintf("nc-%s-pool1-", clusterName)
		machinePoolConfig := provider.MachinePoolFunc(generatedPoolName, namespace)

		machineConfigResp, err := machinepools.CreateMachineConfig(provider.MachineConfig, machinePoolConfig, testSessionClient)
		require.NoError(r.T(), err)

		machinePools := machinepools.RKEMachinePoolSetup(nodesAndRoles, machineConfigResp)

		cluster := clusters.NewRKE2ClusterConfig(clusterName, namespace, cni, credential.ID, kubeVersion, machinePools)

		clusterResp, err := clusters.CreateRKE2Cluster(testSessionClient, cluster)
		require.NoError(r.T(), err)

		kubeProvisioningClient, err := r.client.GetKubeAPIProvisioningClient()
		require.NoError(r.T(), err)

		result, err := kubeProvisioningClient.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
			FieldSelector:  "metadata.name=" + clusterName,
			TimeoutSeconds: &defaults.WatchTimeoutSeconds,
		})
		require.NoError(r.T(), err)

		checkFunc := clusters.IsProvisioningClusterReady
		err = wait.WatchWait(result, checkFunc)
		assert.NoError(r.T(), err)
		assert.Equal(r.T(), clusterName, clusterResp.ObjectMeta.Name)

		newClusterID, err := clusters.GetClusterIDByName(r.client, clusterName)

		require.NoError(r.T(), r.createSnapshot(clusterName, 1))
		snapshotName := r.GetSnapshot(r.client, newClusterID, clusterName, "local", namespace, metav1.ListOptions{})
		time.Sleep(60 * time.Second)
		require.NoError(r.T(), r.restoreSnapshot(clusterName, snapshotName, 1, "all"))

	})
}

func (r *RKE2EtcdSnapshotRestoreTestSuite) createSnapshot(clustername string, generation int) error {
	kubeProvisioningClient, err := r.client.GetKubeAPIProvisioningClient()
	require.NoError(r.T(), err)

	cluster, err := kubeProvisioningClient.Clusters(namespace).Get(context.TODO(), clustername, metav1.GetOptions{})
	if err != nil {
		return err
	}

	cluster.Spec.RKEConfig.ETCDSnapshotCreate = &rkev1.ETCDSnapshotCreate{
		Generation: generation,
	}

	cluster, err = kubeProvisioningClient.Clusters(namespace).Update(context.TODO(), cluster, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	result, err := kubeProvisioningClient.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + cluster.ObjectMeta.Name,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	require.NoError(r.T(), err)

	checkFunc := clusters.IsProvisioningClusterReady

	err = wait.WatchWait(result, checkFunc)
	assert.NoError(r.T(), err)

	return nil
}

func (r *RKE2EtcdSnapshotRestoreTestSuite) restoreSnapshot(clustername string, name string, generation int, restoreconfig string) error {
	kubeProvisioningClient, err := r.client.GetKubeAPIProvisioningClient()
	require.NoError(r.T(), err)

	cluster, err := kubeProvisioningClient.Clusters(namespace).Get(context.TODO(), clustername, metav1.GetOptions{})
	if err != nil {
		return err
	}
	cluster.Spec.RKEConfig.ETCDSnapshotRestore = &rkev1.ETCDSnapshotRestore{
		Name:             name,
		Generation:       generation,
		RestoreRKEConfig: "all",
	}

	cluster, err = kubeProvisioningClient.Clusters(namespace).Update(context.TODO(), cluster, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	results, err := kubeProvisioningClient.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + cluster.ObjectMeta.Name,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	require.NoError(r.T(), err)

	checkFuncs := clusters.IsProvisioningClusterReady

	err = wait.WatchWait(results, checkFuncs)
	assert.NoError(r.T(), err)

	return nil
}

func (r *RKE2EtcdSnapshotRestoreTestSuite) GetSnapshot(client *rancher.Client, newClusterID string, clusterName string, clusterID string, namespace string, getOpts metav1.ListOptions) string {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return ""
	}
	etcdResource := dynamicClient.Resource(EtcdSnapshotGroupVersionResource).Namespace("fleet-default")
	unstructuredResp, err := etcdResource.List(context.TODO(), getOpts)
	if err != nil {
		return ""
	}

	snapshots := &rkev1.ETCDSnapshotList{}
	err = scheme.Scheme.Convert(unstructuredResp, snapshots, unstructuredResp.GroupVersionKind())
	if err != nil {
		return ""
	}

	for _, EtcdSnapshot := range snapshots.Items {
		if EtcdSnapshot.Labels["rke.cattle.io/cluster-name"] == clusterName {
			return EtcdSnapshot.Name
		}
	}
	return ""
}

func (r *RKE2EtcdSnapshotRestoreTestSuite) TestEtcdSnapshotRestore() {
	for _, providerName := range r.providers {
		subSession := r.session.NewSession()

		provider := CreateProvider(providerName)

		client, err := r.client.WithSession(subSession)
		require.NoError(r.T(), err)

		cloudCredential, err := provider.CloudCredFunc(client)
		require.NoError(r.T(), err)

		for _, kubernetesVersion := range r.kubernetesVersions {
			for _, cni := range r.cnis {
				r.TestEtcdSnapshotRestoreFreshCluster(provider, kubernetesVersion, cni, r.nodesAndRoles, cloudCredential)
			}
		}

		subSession.Cleanup()
	}
}

func TestEtcdSnapshotRestore(t *testing.T) {
	suite.Run(t, new(RKE2EtcdSnapshotRestoreTestSuite))
}
