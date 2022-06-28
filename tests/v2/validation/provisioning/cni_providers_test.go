package provisioning

import (
	"context"
	"fmt"
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	"github.com/rancher/rancher/tests/framework/extensions/workloads"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RKE2CNIProvisioningTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	standardUserClient *rancher.Client
	kubernetesVersions []string
	cnis               []string
	providers          []string
}

func (r *RKE2CNIProvisioningTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *RKE2CNIProvisioningTestSuite) SetupSuite() {
	testSession := session.NewSession(r.T())
	r.session = testSession

	clustersConfig := new(Config)
	config.LoadConfig(ConfigurationFileKey, clustersConfig)

	r.kubernetesVersions = clustersConfig.KubernetesVersions
	r.cnis = []string{"canal", "cilium", "calico", "multus,canal", "multus,cilium", "multus,calico"}
	r.providers = clustersConfig.Providers

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)

	r.client = client

	enabled := true
	var testuser = AppendRandomString("testuser-")
	user := &management.User{
		Username: testuser,
		Password: "rancherrancher123!",
		Name:     testuser,
		Enabled:  &enabled,
	}

	newUser, err := users.CreateUserWithRole(client, user, "clusters-create", "user")
	require.NoError(r.T(), err)

	newUser.Password = user.Password

	standardUserClient, err := client.AsUser(newUser)
	require.NoError(r.T(), err)

	r.standardUserClient = standardUserClient
}

func (r *RKE2CNIProvisioningTestSuite) ProvisioningRKE2Cluster(provider Provider) {
	providerName := " Node Provider: " + provider.Name

	nodeRoles1 := []machinepools.NodeRoles{
		{
			ControlPlane: true,
			Etcd:         false,
			Worker:       false,
			Quantity:     2,
		},
		{
			ControlPlane: false,
			Etcd:         true,
			Worker:       false,
			Quantity:     3,
		},
		{
			ControlPlane: false,
			Etcd:         false,
			Worker:       true,
			Quantity:     3,
		},
	}

	tests := []struct {
		name      string
		nodeRoles []machinepools.NodeRoles
		client    *rancher.Client
	}{
		{"3 etcd 2 cp 3 worker nodes Admin User", nodeRoles1, r.client},
		{"3 etcd 2 cp 3 worker nodes Standard User", nodeRoles1, r.standardUserClient},
	}

	var name string
	for _, tt := range tests {
		subSession := r.session.NewSession()
		defer subSession.Cleanup()

		client, err := tt.client.WithSession(subSession)
		require.NoError(r.T(), err)

		cloudCredential, err := provider.CloudCredFunc(client)
		require.NoError(r.T(), err)
		for _, kubeVersion := range r.kubernetesVersions {
			name = tt.name + providerName + " Kubernetes version: " + kubeVersion
			for _, cni := range r.cnis {
				name += " cni: " + cni
				r.Run(name, func() {
					testSession := session.NewSession(r.T())

					testSessionClient, err := tt.client.WithSession(testSession)
					require.NoError(r.T(), err)

					clusterName := AppendRandomString(provider.Name)
					generatedPoolName := fmt.Sprintf("nc-%s-pool1-", clusterName)
					machinePoolConfig := provider.MachinePoolFunc(generatedPoolName, namespace)

					machineConfigResp, err := machinepools.CreateMachineConfig(provider.MachineConfig, machinePoolConfig, testSessionClient)
					require.NoError(r.T(), err)

					machinePools := machinepools.RKEMachinePoolSetup(tt.nodeRoles, machineConfigResp)

					cluster := clusters.NewRKE2ClusterConfig(clusterName, namespace, cni, cloudCredential.ID, kubeVersion, machinePools)

					clusterResp, err := clusters.CreateRKE2Cluster(testSessionClient, cluster)
					require.NoError(r.T(), err)

					result, err := r.client.Provisioning.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
						FieldSelector:  "metadata.name=" + clusterName,
						TimeoutSeconds: &defaults.WatchTimeoutSeconds,
					})
					require.NoError(r.T(), err)

					checkFunc := clusters.IsProvisioningClusterReady

					err = wait.WatchWait(result, checkFunc)
					assert.NoError(r.T(), err)
					assert.Equal(r.T(), clusterName, clusterResp.Name)

					clusterID, err := clusters.GetClusterIDByName(r.client, clusterName)
					assert.NoError(r.T(), err)
					podResults, podErrors := workloads.StatusPods(client, clusterID, metav1.ListOptions{})
					fmt.Print(podResults, podErrors)
				})
			}
		}
	}
}

func (r *RKE2CNIProvisioningTestSuite) TestProvisioning() {
	for _, providerName := range r.providers {
		provider := CreateProvider(providerName)
		r.ProvisioningRKE2Cluster(provider)
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestCNIProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(RKE2CNIProvisioningTestSuite))
}
