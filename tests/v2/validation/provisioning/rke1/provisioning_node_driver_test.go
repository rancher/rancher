package provisioning

import (
	"fmt"
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	rkeconfig "github.com/rancher/rancher/tests/v2/validation/provisioning/rke2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RKE1NodeDriverProvisioningTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	kubernetesVersions []string
	cnis               []string
	providers          []string
}

func (r *RKE1NodeDriverProvisioningTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *RKE1NodeDriverProvisioningTestSuite) SetupSuite() {
	testSession := session.NewSession(r.T())
	r.session = testSession

	clustersConfig := new(rkeconfig.Config)
	config.LoadConfig(rkeconfig.ConfigurationFileKey, clustersConfig)

	r.kubernetesVersions = clustersConfig.KubernetesVersions
	r.cnis = clustersConfig.CNIs
	r.providers = clustersConfig.Providers

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)

	r.client = client

	enabled := true
	var testuser = rkeconfig.AppendRandomString("testuser-")
	user := &management.User{
		Username: testuser,
		Password: "rancherrancher123!",
		Name:     testuser,
		Enabled:  &enabled,
	}

	newUser, err := users.CreateUserWithRole(client, user, "clusters-create", "user")
	require.NoError(r.T(), err)

	newUser.Password = user.Password
}

func (r *RKE1NodeDriverProvisioningTestSuite) ProvisioningRKE1Cluster(provider Provider) {
	providerName := " Node Provider: " + provider.Name
	tests := []struct {
		name   string
		client *rancher.Client
	}{
		{"Admin User", r.client},
	}

	var name string
	for _, tt := range tests {
		subSession := r.session.NewSession()
		defer subSession.Cleanup()

		client, err := tt.client.WithSession(subSession)
		require.NoError(r.T(), err)

		for _, kubeVersion := range r.kubernetesVersions {
			name = tt.name + providerName + " Kubernetes version: " + kubeVersion
			for _, cni := range r.cnis {
				name += " cni: " + cni
				r.Run(name, func() {
					cluster := &management.Cluster{
						DockerRootDir:           "/var/lib/docker",
						EnableClusterAlerting:   false,
						EnableClusterMonitoring: false,
						LocalClusterAuthEndpoint: &management.LocalClusterAuthEndpoint{
							Enabled: true,
						},
						Name:     rkeconfig.AppendRandomString("rke1"),
						Provider: provider.Name,
						RancherKubernetesEngineConfig: &management.RancherKubernetesEngineConfig{
							DNS: &management.DNSConfig{
								Provider: "coredns",
								Options: map[string]string{
									"stubDomains": "cluster.local",
								},
							},
							Ingress: &management.IngressConfig{
								Provider: "nginx",
							},
							Monitoring: &management.MonitoringConfig{
								Provider: "metrics-server",
							},
							Network: &management.NetworkConfig{
								MTU:     0,
								Options: map[string]string{},
							},
						},
					}

					clusterResp, err := client.Management.Cluster.Create(cluster)
					require.NoError(r.T(), err)

					nodeTemplateResp, err := provider.NodeTemplateFunc(client)

					nodePool := &management.NodePool{
						ClusterID:               clusterResp.ID,
						ControlPlane:            true,
						DeleteNotReadyAfterSecs: 0,
						Etcd:                    true,
						HostnamePrefix:          rkeconfig.AppendRandomString("rke1"),
						NodeTemplateID:          nodeTemplateResp.ID,
						Quantity:                1,
						Worker:                  true,
					}

					nodePoolResp, err := client.Management.NodePool.Create(nodePool)
					require.NoError(r.T(), err)
					fmt.Printf(nodePoolResp.ClusterID)

					clusterName := clusterResp.Name

					opts := metav1.ListOptions{
						FieldSelector:  "metadata.name=" + clusterResp.ID,
						TimeoutSeconds: &defaults.WatchTimeoutSeconds,
					}
					watchInterface, err := r.client.GetManagementWatchInterface(management.ClusterType, opts)
					require.NoError(r.T(), err)

					checkFunc := clusters.IsHostedProvisioningClusterReady

					err = wait.WatchWait(watchInterface, checkFunc)
					require.NoError(r.T(), err)
					assert.Equal(r.T(), clusterName, clusterResp.Name)
				})
			}
		}
	}
}

func (r *RKE1NodeDriverProvisioningTestSuite) TestProvisioning() {
	for _, providerName := range r.providers {
		provider := CreateProvider(providerName)
		r.ProvisioningRKE1Cluster(provider)
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestRKE1ProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(RKE1NodeDriverProvisioningTestSuite))
}
