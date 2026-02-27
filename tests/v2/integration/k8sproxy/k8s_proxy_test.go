package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	provisioningv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2prov/clients"
	v2provcluster "github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

type K8sProxyTestSuite struct {
	suite.Suite
	client              *rancher.Client
	v2provClient        *clients.Clients
	session             *session.Session
	downstreamClusterID string
}

func (s *K8sProxyTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	client, err := rancher.NewClient("", testSession)
	s.Require().NoError(err)
	s.client = client

	v2provClient, err := clients.New()
	s.Require().NoError(err)
	s.v2provClient = v2provClient

	downstreamCluster, err := v2provcluster.New(v2provClient, &provisioningv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "k8s-proxy-downstream-",
		},
		Spec: provisioningv1.ClusterSpec{
			KubernetesVersion: defaults.SomeK8sVersion,
			RKEConfig: &provisioningv1.RKEConfig{
				MachinePools: []provisioningv1.RKEMachinePool{{
					EtcdRole:         true,
					ControlPlaneRole: true,
					WorkerRole:       true,
					Quantity:         &defaults.One,
				}},
			},
		},
	})
	s.Require().NoError(err)

	downstreamCluster, err = v2provcluster.WaitForCreate(v2provClient, downstreamCluster)
	s.Require().NoError(err)

	s.downstreamClusterID = downstreamCluster.Status.ClusterName
	s.Require().NotEmpty(s.downstreamClusterID)
}

func (s *K8sProxyTestSuite) TearDownSuite() {
	if s.v2provClient != nil {
		s.v2provClient.Close()
	}
	s.session.Cleanup()
}

func (s *K8sProxyTestSuite) TestK8sProxyLocalNamespaces() {
	responseCode, body := s.doProxyRequest("local", "/api/v1/namespaces/default")
	s.Require().Equal(http.StatusOK, responseCode)

	var ns *corev1.Namespace
	err := json.Unmarshal(body, ns)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), ns.Name, "default")
}

func (s *K8sProxyTestSuite) TestK8sProxyDownstreamNamespaces() {
	responseCode, body := s.doProxyRequest(s.downstreamClusterID, "/api/v1/namespaces/default")
	s.Require().Equal(http.StatusOK, responseCode)

	var ns *corev1.Namespace
	err := json.Unmarshal(body, ns)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), ns.Name, "default")
}

func (s *K8sProxyTestSuite) TestK8sProxyDownstreamV1NotFound() {
	responseCode, _ := s.doProxyRequest(s.downstreamClusterID, "/v1")
	s.Require().Equal(http.StatusNotFound, responseCode)
}

func (s *K8sProxyTestSuite) doProxyRequest(clusterID, pathSuffix string) (int, []byte) {
	httpClient, err := rest.HTTPClientFor(s.client.WranglerContext.RESTConfig)
	s.Require().NoError(err)

	url := fmt.Sprintf("https://%s/k8s/proxy/%s%s", s.client.RancherConfig.Host, clusterID, pathSuffix)
	request, err := http.NewRequest(http.MethodGet, url, nil)
	s.Require().NoError(err)
	request.Header.Set("Authorization", "Bearer "+s.client.RancherConfig.AdminToken)

	response, err := httpClient.Do(request)
	s.Require().NoError(err)
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	s.Require().NoError(err)

	return response.StatusCode, body
}

func TestK8sProxySuite(t *testing.T) {
	suite.Run(t, new(K8sProxyTestSuite))
}
