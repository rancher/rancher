package provisioning

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	kubeconfig "github.com/rancher/rancher/tests/framework/extensions/kubeconfig"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
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
	kubernetesVersions       []string
	cnis                     []string
	providers                []string
	localClusterAuthEndpoint rkev1.LocalClusterAuthEndpoint
}

func (r *RKE2ACETestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *RKE2ACETestSuite) SetupSuite() {
	testSession := session.NewSession(r.T())
	r.session = testSession

	clustersConfig := new(Config)
	config.LoadConfig(ConfigurationFileKey, clustersConfig)

	r.kubernetesVersions = clustersConfig.KubernetesVersions
	r.cnis = clustersConfig.CNIs
	r.providers = clustersConfig.Providers
	r.localClusterAuthEndpoint = clustersConfig.LocalClusterAuthEndpoint

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)

	r.client = client
}

func (r *RKE2ACETestSuite) Provisioning_RKE2ClusterACE(provider Provider) {

	subSession := r.session.NewSession()
	defer subSession.Cleanup()

	client, err := r.client.WithSession(subSession)
	require.NoError(r.T(), err)

	cloudCredential, err := provider.CloudCredFunc(client)
	require.NoError(r.T(), err)

	nodeRoles0 := []map[string]bool{
		{
			"controlplane": true,
			"etcd":         false,
			"worker":       false,
		},
		{
			"controlplane": false,
			"etcd":         true,
			"worker":       false,
		},
		{
			"controlplane": false,
			"etcd":         false,
			"worker":       true,
		},
		{
			"controlplane": false,
			"etcd":         false,
			"worker":       true,
		},
		{
			"controlplane": false,
			"etcd":         false,
			"worker":       true,
		},
	}

	nodeRoles1 := []map[string]bool{
		{
			"controlplane": true,
			"etcd":         false,
			"worker":       false,
		},
		{
			"controlplane": false,
			"etcd":         true,
			"worker":       false,
		},
		{
			"controlplane": true,
			"etcd":         false,
			"worker":       false,
		},
		{
			"controlplane": false,
			"etcd":         false,
			"worker":       true,
		},
		{
			"controlplane": false,
			"etcd":         false,
			"worker":       true,
		},
	}

	tests := []struct {
		name      string
		nodeRoles []map[string]bool
		client    *rancher.Client
	}{
		{"Single Control Plane", nodeRoles0, r.client},
		{"Multiple Control Planes", nodeRoles1, r.client},
	}

	var name string
	for _, tt := range tests {
		for _, kubeVersion := range r.kubernetesVersions {
			name = tt.name + " Kubernetes version: " + kubeVersion
			for _, cni := range r.cnis {
				name += " cni: " + cni
				r.Run(name, func() {
					testSession := session.NewSession(r.T())
					defer testSession.Cleanup()

					testSessionClient, err := tt.client.WithSession(testSession)
					require.NoError(r.T(), err)

					clusterName := AppendRandomString(provider.Name)
					generatedPoolName := fmt.Sprintf("nc-%s-pool1-", clusterName)
					machinePoolConfig := provider.MachinePoolFunc(generatedPoolName, namespace)

					machineConfigResp, err := machinepools.CreateMachineConfig(provider.MachineConfig, machinePoolConfig, testSessionClient)
					require.NoError(r.T(), err)

					machinePools := machinepools.RKEMachinePoolSetup(tt.nodeRoles, machineConfigResp)

					cluster := clusters.NewRKE2ClusterConfig(clusterName, namespace, cni, cloudCredential.ID, kubeVersion, machinePools)

					cluster.Spec.LocalClusterAuthEndpoint = r.localClusterAuthEndpoint

					clusterResp, err := clusters.CreateRKE2Cluster(testSessionClient, cluster)
					require.NoError(r.T(), err)

					result, err := testSessionClient.Provisioning.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
						FieldSelector:  "metadata.name=" + clusterName,
						TimeoutSeconds: &defaults.WatchTimeoutSeconds,
					})
					require.NoError(r.T(), err)

					checkFunc := clusters.IsProvisioningClusterReady

					err = wait.WatchWait(result, checkFunc)
					assert.NoError(r.T(), err)
					assert.Equal(r.T(), clusterName, clusterResp.Name)

					clusterInfo, err := testSessionClient.Provisioning.Clusters(namespace).Get(context.TODO(), clusterResp.Name, metav1.GetOptions{})
					require.NoError(r.T(), err)

					clusterID := clusterInfo.Status.ClusterName

					kubeConfig, err := kubeconfig.GetKubeconfig(r.client, clusterID)
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
					for key, _ := range contexts {
						if strings.Contains(key, "pool") {
							contextNames = append(contextNames, key)
						}
					}

					for _, contextName := range contextNames {

						time.Sleep(30 * time.Second)

						dynamic, err := r.client.SwitchContext(contextName, kubeConfig)
						require.NoError(r.T(), err)

						resp, err := dynamic.Resource(corev1.SchemeGroupVersion.WithResource("pods")).Namespace("").List(context.TODO(), metav1.ListOptions{})
						require.NoError(r.T(), err)

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

func (r *RKE2ACETestSuite) TestProvisioningACE() {
	for provider_name := range r.providers {
		provider_struct := CreateProvider(r.providers[provider_name])
		r.Provisioning_RKE2ClusterACE(provider_struct)
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestRKE2ACETestSuite(t *testing.T) {
	suite.Run(t, new(RKE2ACETestSuite))
}
