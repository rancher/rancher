package k3s

import (
	"context"
	"fmt"
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	password "github.com/rancher/rancher/tests/framework/extensions/users/passwordgenerator"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	provisioning "github.com/rancher/rancher/tests/v2/validation/provisioning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	namespace = "fleet-default"
)

type K3SNodeDriverProvisioningTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	standardUserClient *rancher.Client
	kubernetesVersions []string
	providers          []string
}

func (k *K3SNodeDriverProvisioningTestSuite) TearDownSuite() {
	k.session.Cleanup()
}

func (k *K3SNodeDriverProvisioningTestSuite) SetupSuite() {
	testSession := session.NewSession(k.T())
	k.session = testSession

	clustersConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, clustersConfig)

	k.kubernetesVersions = clustersConfig.KubernetesVersions
	k.providers = clustersConfig.Providers

	client, err := rancher.NewClient("", testSession)
	require.NoError(k.T(), err)

	k.client = client

	enabled := true
	var testuser = provisioning.AppendRandomString("testuser-")
	var testpassword = password.GenerateUserPassword("testpass-")
	user := &management.User{
		Username: testuser,
		Password: testpassword,
		Name:     testuser,
		Enabled:  &enabled,
	}

	newUser, err := users.CreateUserWithRole(client, user, "user")
	require.NoError(k.T(), err)

	newUser.Password = user.Password

	standardUserClient, err := client.AsUser(newUser)
	require.NoError(k.T(), err)

	k.standardUserClient = standardUserClient
}

func (k *K3SNodeDriverProvisioningTestSuite) ProvisioningK3SCluster(provider Provider) {
	providerName := " Node Provider: " + provider.Name
	nodeRoles0 := []machinepools.NodeRoles{
		{
			ControlPlane: true,
			Etcd:         true,
			Worker:       true,
			Quantity:     1,
		},
	}

	nodeRoles1 := []machinepools.NodeRoles{
		{
			ControlPlane: true,
			Etcd:         false,
			Worker:       false,
			Quantity:     1,
		},
		{
			ControlPlane: false,
			Etcd:         true,
			Worker:       false,
			Quantity:     1,
		},
		{
			ControlPlane: false,
			Etcd:         false,
			Worker:       true,
			Quantity:     1,
		},
	}

	tests := []struct {
		name      string
		nodeRoles []machinepools.NodeRoles
		client    *rancher.Client
	}{
		{"1 Node all roles Admin User", nodeRoles0, k.client},
		{"1 Node all roles Standard User", nodeRoles0, k.standardUserClient},
		{"3 nodes - 1 role per node Admin User", nodeRoles1, k.client},
		{"3 nodes - 1 role per node Standard User", nodeRoles1, k.standardUserClient},
	}

	var name string
	for _, tt := range tests {
		subSession := k.session.NewSession()
		defer subSession.Cleanup()

		client, err := tt.client.WithSession(subSession)
		require.NoError(k.T(), err)

		cloudCredential, err := provider.CloudCredFunc(client)
		require.NoError(k.T(), err)
		for _, kubeVersion := range k.kubernetesVersions {
			name = tt.name + providerName + " Kubernetes version: " + kubeVersion
			k.Run(name, func() {
				testSession := session.NewSession(k.T())
				defer testSession.Cleanup()

				testSessionClient, err := tt.client.WithSession(testSession)
				require.NoError(k.T(), err)

				clusterName := provisioning.AppendRandomString(provider.Name)
				generatedPoolName := fmt.Sprintf("nc-%s-pool1-", clusterName)
				machinePoolConfig := provider.MachinePoolFunc(generatedPoolName, namespace)

				machineConfigResp, err := testSessionClient.Steve.SteveType(provider.MachineConfigPoolResourceSteveType).Create(machinePoolConfig)
				require.NoError(k.T(), err)

				machinePools := machinepools.RKEMachinePoolSetup(tt.nodeRoles, machineConfigResp)

				cluster := clusters.NewK3SRKE2ClusterConfig(clusterName, namespace, "", cloudCredential.ID, kubeVersion, machinePools)

				clusterResp, err := clusters.CreateK3SRKE2Cluster(testSessionClient, cluster)
				require.NoError(k.T(), err)

				kubeProvisioningClient, err := k.client.GetKubeAPIProvisioningClient()
				require.NoError(k.T(), err)

				result, err := kubeProvisioningClient.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
					FieldSelector:  "metadata.name=" + clusterName,
					TimeoutSeconds: &defaults.WatchTimeoutSeconds,
				})
				require.NoError(k.T(), err)

				checkFunc := clusters.IsProvisioningClusterReady

				err = wait.WatchWait(result, checkFunc)
				assert.NoError(k.T(), err)
				assert.Equal(k.T(), clusterName, clusterResp.ObjectMeta.Name)

				clusterToken, err := clusters.CheckServiceAccountTokenSecret(client, clusterName)
				require.NoError(k.T(), err)
				assert.NotEmpty(k.T(), clusterToken)
			})
		}
	}
}

func (k *K3SNodeDriverProvisioningTestSuite) ProvisioningK3SClusterDynamicInput(provider Provider, nodesAndRoles []machinepools.NodeRoles) {
	providerName := " Node Provider: " + provider.Name
	tests := []struct {
		name   string
		client *rancher.Client
	}{
		{"Admin User", k.client},
		{"Standard User", k.standardUserClient},
	}

	var name string
	for _, tt := range tests {
		subSession := k.session.NewSession()
		defer subSession.Cleanup()

		client, err := tt.client.WithSession(subSession)
		require.NoError(k.T(), err)

		cloudCredential, err := provider.CloudCredFunc(client)
		require.NoError(k.T(), err)

		for _, kubeVersion := range k.kubernetesVersions {
			name = tt.name + providerName + " Kubernetes version: " + kubeVersion
			k.Run(name, func() {
				testSession := session.NewSession(k.T())
				defer testSession.Cleanup()

				testSessionClient, err := tt.client.WithSession(testSession)
				require.NoError(k.T(), err)

				clusterName := provisioning.AppendRandomString(provider.Name)
				generatedPoolName := fmt.Sprintf("nc-%s-pool1-", clusterName)
				machinePoolConfig := provider.MachinePoolFunc(generatedPoolName, namespace)

				machineConfigResp, err := testSessionClient.Steve.SteveType(provider.MachineConfigPoolResourceSteveType).Create(machinePoolConfig)
				require.NoError(k.T(), err)

				machinePools := machinepools.RKEMachinePoolSetup(nodesAndRoles, machineConfigResp)

				cluster := clusters.NewK3SRKE2ClusterConfig(clusterName, namespace, "", cloudCredential.ID, kubeVersion, machinePools)

				clusterResp, err := clusters.CreateK3SRKE2Cluster(testSessionClient, cluster)
				require.NoError(k.T(), err)

				kubeProvisioningClient, err := k.client.GetKubeAPIProvisioningClient()
				require.NoError(k.T(), err)

				result, err := kubeProvisioningClient.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
					FieldSelector:  "metadata.name=" + clusterName,
					TimeoutSeconds: &defaults.WatchTimeoutSeconds,
				})
				require.NoError(k.T(), err)

				checkFunc := clusters.IsProvisioningClusterReady

				err = wait.WatchWait(result, checkFunc)
				assert.NoError(k.T(), err)
				assert.Equal(k.T(), clusterName, clusterResp.ObjectMeta.Name)

				clusterToken, err := clusters.CheckServiceAccountTokenSecret(client, clusterName)
				require.NoError(k.T(), err)
				assert.NotEmpty(k.T(), clusterToken)
			})
		}
	}
}

func (k *K3SNodeDriverProvisioningTestSuite) TestProvisioning() {
	for _, providerName := range k.providers {
		provider := CreateProvider(providerName)
		k.ProvisioningK3SCluster(provider)
	}
}

func (k *K3SNodeDriverProvisioningTestSuite) TestProvisioningDynamicInput() {
	clustersConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, clustersConfig)
	nodesAndRoles := clustersConfig.NodesAndRoles

	if len(nodesAndRoles) == 0 {
		k.T().Skip()
	}

	for _, providerName := range k.providers {
		provider := CreateProvider(providerName)
		k.ProvisioningK3SClusterDynamicInput(provider, nodesAndRoles)
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestK3SProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(K3SNodeDriverProvisioningTestSuite))
}
