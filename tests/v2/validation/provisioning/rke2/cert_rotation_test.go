package rke2

import (
	"context"
	"fmt"
	"testing"

	apiv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/cloudcredentials"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/defaults"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	"github.com/rancher/rancher/tests/framework/extensions/pipeline"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/environmentflag"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/v2/validation/provisioning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type V2ProvCertRotationTestSuite struct {
	suite.Suite
	session         *session.Session
	client          *rancher.Client
	config          *provisioning.Config
	clusterName     string
	namespace       string
	advancedOptions provisioning.AdvancedOptions
}

func (r *V2ProvCertRotationTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *V2ProvCertRotationTestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession

	r.config = new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, r.config)

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)

	r.client = client

	r.clusterName = r.client.RancherConfig.ClusterName
	r.namespace = r.client.RancherConfig.ClusterName
	r.advancedOptions = r.config.AdvancedOptions
}

func (r *V2ProvCertRotationTestSuite) testCertRotation(provider Provider, kubeVersion string, nodesAndRoles []machinepools.NodeRoles, credential *cloudcredentials.CloudCredential) {
	name := fmt.Sprintf("Provider_%s/Kubernetes_Version_%s/Nodes_%v", provider.Name, kubeVersion, nodesAndRoles)
	r.Run(name, func() {
		r.Run("initial", func() {
			testSession := session.NewSession()
			defer testSession.Cleanup()

			testSessionClient, err := r.client.WithSession(testSession)
			require.NoError(r.T(), err)

			clusterName := namegen.AppendRandomString(fmt.Sprintf("%s-%s", r.clusterName, provider.Name))
			generatedPoolName := fmt.Sprintf("nc-%s-pool1-", clusterName)
			machinePoolConfig := provider.MachinePoolFunc(generatedPoolName, namespace)

			machineConfigResp, err := testSessionClient.Steve.SteveType(provider.MachineConfigPoolResourceSteveType).Create(machinePoolConfig)
			require.NoError(r.T(), err)

			machinePools := machinepools.RKEMachinePoolSetup(nodesAndRoles, machineConfigResp)

			cluster := clusters.NewK3SRKE2ClusterConfig(clusterName, namespace, "calico", credential.ID, kubeVersion, "", machinePools, r.advancedOptions)
			clusterResp, err := clusters.CreateK3SRKE2Cluster(testSessionClient, cluster)
			require.NoError(r.T(), err)

			if r.client.Flags.GetValue(environmentflag.UpdateClusterName) {
				pipeline.UpdateConfigClusterName(clusterName)
			}

			kubeProvisioningClient, err := r.client.GetKubeAPIProvisioningClient()
			require.NoError(r.T(), err)

			result, err := kubeProvisioningClient.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
				FieldSelector:  "metadata.name=" + cluster.ObjectMeta.Name,
				TimeoutSeconds: &defaults.WatchTimeoutSeconds,
			})
			require.NoError(r.T(), err)
			checkFunc := clusters.IsProvisioningClusterReady

			err = wait.WatchWait(result, checkFunc)
			require.NoError(r.T(), err)
			assert.Equal(r.T(), clusterName, clusterResp.ObjectMeta.Name)

			steveCluster, err := r.client.Steve.SteveType(clusters.ProvisioningSteveResourceType).ByID(clusterResp.ID)
			require.NoError(r.T(), err)
			require.NotNil(r.T(), steveCluster.Status)

			// rotate certs
			require.NoError(r.T(), r.rotateCerts(clusterResp.ID, 1))
			// rotate certs again
			require.NoError(r.T(), r.rotateCerts(clusterResp.ID, 2))
		})
	})
}

func (r *V2ProvCertRotationTestSuite) TestCertRotation() {
	for _, providerName := range r.config.Providers {
		subSession := r.session.NewSession()

		provider := CreateProvider(providerName)

		client, err := r.client.WithSession(subSession)
		require.NoError(r.T(), err)

		cloudCredential, err := provider.CloudCredFunc(client)
		require.NoError(r.T(), err)

		for _, kubernetesVersion := range r.config.RKE2KubernetesVersions {
			r.testCertRotation(provider, kubernetesVersion, r.config.NodesAndRoles, cloudCredential)
		}

		subSession.Cleanup()
	}
}

func (r *V2ProvCertRotationTestSuite) rotateCerts(id string, generation int64) error {
	kubeProvisioningClient, err := r.client.GetKubeAPIProvisioningClient()
	require.NoError(r.T(), err)

	cluster, err := r.client.Steve.SteveType(clusters.ProvisioningSteveResourceType).ByID(id)
	if err != nil {
		return err
	}

	clusterSpec := &apiv1.ClusterSpec{}
	err = v1.ConvertToK8sType(cluster.Spec, clusterSpec)
	require.NoError(r.T(), err)

	updatedCluster := *cluster

	clusterSpec.RKEConfig.RotateCertificates = &rkev1.RotateCertificates{
		Generation: generation,
	}

	updatedCluster.Spec = *clusterSpec

	cluster, err = r.client.Steve.SteveType(clusters.ProvisioningSteveResourceType).Update(cluster, updatedCluster)
	if err != nil {
		return err
	}

	kubeRKEClient, err := r.client.GetKubeAPIRKEClient()
	require.NoError(r.T(), err)

	result, err := kubeRKEClient.RKEControlPlanes(namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + cluster.ObjectMeta.Name,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	require.NoError(r.T(), err)

	checkFunc := CertRotationComplete(generation)

	err = wait.WatchWait(result, checkFunc)
	if err != nil {
		return err
	}

	clusterWait, err := kubeProvisioningClient.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + cluster.ObjectMeta.Name,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	if err != nil {
		return err
	}

	clusterCheckFunc := clusters.IsProvisioningClusterReady

	err = wait.WatchWait(clusterWait, clusterCheckFunc)
	if err != nil {
		return err
	}

	return nil
}

func CertRotationComplete(generation int64) wait.WatchCheckFunc {
	return func(event watch.Event) (bool, error) {
		controlPlane := event.Object.(*rkev1.RKEControlPlane)
		return controlPlane.Status.CertificateRotationGeneration == generation, nil
	}
}

func TestCertRotation(t *testing.T) {
	suite.Run(t, new(V2ProvCertRotationTestSuite))
}
