package rke2

import (
	"context"
	"fmt"
	"strings"
	"testing"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/kubeconfig"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	nodestat "github.com/rancher/rancher/tests/framework/extensions/nodes"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	password "github.com/rancher/rancher/tests/framework/extensions/users/passwordgenerator"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	provisioning "github.com/rancher/rancher/tests/v2/validation/provisioning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RKE2ACETestSuite struct {
	suite.Suite
	client                   *rancher.Client
	session                  *session.Session
	standardUserClient       *rancher.Client
	kubernetesVersions       []string
	cnis                     []string
	providers                []string
	psact                    string
	advancedOptions          provisioning.AdvancedOptions
	localClusterAuthEndpoint rkev1.LocalClusterAuthEndpoint
}

func (r *RKE2ACETestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *RKE2ACETestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession

	clustersConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, clustersConfig)

	r.kubernetesVersions = clustersConfig.RKE2KubernetesVersions
	r.cnis = clustersConfig.CNIs
	r.providers = clustersConfig.Providers
	r.psact = clustersConfig.PSACT
	r.advancedOptions = clustersConfig.AdvancedOptions
	r.localClusterAuthEndpoint = clustersConfig.LocalClusterAuthEndpoint

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)

	r.client = client

	enabled := true
	var testuser = namegen.AppendRandomString("testuser-")
	var testpassword = password.GenerateUserPassword("testpass-")
	user := &management.User{
		Username: testuser,
		Password: testpassword,
		Name:     testuser,
		Enabled:  &enabled,
	}

	newUser, err := users.CreateUserWithRole(client, user, "user")
	require.NoError(r.T(), err)

	newUser.Password = user.Password

	standardUserClient, err := client.AsUser(newUser)
	require.NoError(r.T(), err)

	r.standardUserClient = standardUserClient
}

func (r *RKE2ACETestSuite) TestProvisioningRKE2ClusterACE() {
	nodeRoles0 := []machinepools.NodeRoles{
		{
			ControlPlane: true,
			Etcd:         false,
			Worker:       false,
			Quantity:     3,
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
		psact     string
	}{
		{"Multiple Control Planes - Admin", nodeRoles0, r.client, r.psact},
		{"Multiple Control Planes - Standard", nodeRoles0, r.standardUserClient, r.psact},
	}

	var name string
	for _, tt := range tests {
		subSession := r.session.NewSession()
		defer subSession.Cleanup()

		client, err := tt.client.WithSession(subSession)
		require.NoError(r.T(), err)

		for _, providerName := range r.providers {
			provider := CreateProvider(providerName)
			providerName := " Node Provider: " + provider.Name.String()
			for _, kubeVersion := range r.kubernetesVersions {
				name = tt.name + providerName + " Kubernetes version: " + kubeVersion
				for _, cni := range r.cnis {
					name += " cni: " + cni
					r.Run(name, func() {
						cloudCredential, err := provider.CloudCredFunc(client)
						require.NoError(r.T(), err)

						clusterName := namegen.AppendRandomString(provider.Name.String())
						generatedPoolName := fmt.Sprintf("nc-%s-pool1-", clusterName)
						machinePoolConfig := provider.MachinePoolFunc(generatedPoolName, namespace)

						machineConfigResp, err := client.Steve.SteveType(provider.MachineConfigPoolResourceSteveType).Create(machinePoolConfig)
						require.NoError(r.T(), err)

						machinePools := machinepools.RKEMachinePoolSetup(nodeRoles0, machineConfigResp)

						cluster := clusters.NewK3SRKE2ClusterConfig(clusterName, namespace, cni, cloudCredential.ID, kubeVersion, tt.psact, machinePools, r.advancedOptions)

						cluster.Spec.LocalClusterAuthEndpoint = r.localClusterAuthEndpoint

						clusterResp, err := clusters.CreateK3SRKE2Cluster(client, cluster)
						require.NoError(r.T(), err)

						adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
						require.NoError(r.T(), err)
						kubeProvisioningClient, err := adminClient.GetKubeAPIProvisioningClient()
						require.NoError(r.T(), err)

						result, err := kubeProvisioningClient.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
							FieldSelector:  "metadata.name=" + clusterName,
							TimeoutSeconds: &defaults.WatchTimeoutSeconds,
						})
						require.NoError(r.T(), err)

						checkFunc := clusters.IsProvisioningClusterReady

						err = wait.WatchWait(result, checkFunc)
						require.NoError(r.T(), err)
						assert.Equal(r.T(), clusterName, clusterResp.ObjectMeta.Name)
						assert.Equal(r.T(), kubeVersion, cluster.Spec.KubernetesVersion)

						clusterIDName, err := clusters.GetClusterIDByName(adminClient, clusterName)
						assert.NoError(r.T(), err)

						err = nodestat.IsNodeReady(client, clusterIDName)
						require.NoError(r.T(), err)

						kubeConfig, err := kubeconfig.GetKubeconfig(client, clusterIDName)
						require.NoError(r.T(), err)

						original, err := r.client.SwitchContext(clusterResp.Name, kubeConfig)
						require.NoError(r.T(), err)

						originalResp, err := original.Resource(corev1.SchemeGroupVersion.WithResource("pods")).Namespace("").List(context.TODO(), metav1.ListOptions{})
						require.NoError(r.T(), err)
						for _, pod := range originalResp.Items {
							r.T().Logf("Pod %v", pod.GetName())
						}

						contexts, err := kubeconfig.GetContexts(kubeConfig)
						require.NoError(r.T(), err)
						var contextNames []string
						for context := range contexts {
							if strings.Contains(context, "pool") {
								contextNames = append(contextNames, context)
							}
						}

						for _, contextName := range contextNames {
							dynamic, err := r.client.SwitchContext(contextName, kubeConfig)
							assert.NoError(r.T(), err)
							resp, err := dynamic.Resource(corev1.SchemeGroupVersion.WithResource("pods")).Namespace("").List(context.TODO(), metav1.ListOptions{})
							assert.NoError(r.T(), err)
							r.T().Logf("Switched Context to %v", contextName)
							for _, pod := range resp.Items {
								r.T().Logf("Pod %v", pod.GetName())
							}
						}

					})
				}
			}
		}
	}
}

func TestRKE2ACETestSuite(t *testing.T) {
	suite.Run(t, new(RKE2ACETestSuite))
}
