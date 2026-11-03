package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
)

type K8sProxyTestSuite struct {
	suite.Suite
	client              *rancher.Client
	session             *session.Session
	downstreamClusterID string
}

func (s *K8sProxyTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	client, err := rancher.NewClient("", testSession)
	s.Require().NoError(err)
	s.client = client

	clusterID, err := s.findDownstreamClusterID()
	s.Require().NoError(err)
	s.downstreamClusterID = clusterID
}

func (s *K8sProxyTestSuite) TearDownSuite() {
	s.session.Cleanup()
}

// findDownstreamClusterID polls the management API until an active downstream cluster
// with a Ready condition is found, then returns its ID. Returns an error if no such
// cluster is found within the timeout.
func (s *K8sProxyTestSuite) findDownstreamClusterID() (string, error) {
	var clusterID string
	err := wait.PollUntilContextTimeout(s.T().Context(), 5*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		clusterList, err := s.client.Management.Cluster.ListAll(nil)
		if err != nil {
			return false, err
		}
		for _, cluster := range clusterList.Data {
			if cluster.ID == "local" || cluster.State != "active" {
				continue
			}
			for _, condition := range cluster.Conditions {
				if condition.Type == "Ready" && condition.Status == "True" {
					clusterID = cluster.ID
					return true, nil
				}
			}
		}
		return false, nil
	})
	if err != nil {
		return "", fmt.Errorf("no ready downstream cluster found within timeout: %w", err)
	}
	return clusterID, nil
}

func (s *K8sProxyTestSuite) httpClient() *http.Client {
	httpClient, err := rest.HTTPClientFor(s.client.WranglerContext.RESTConfig)
	s.Require().NoError(err)
	return httpClient
}

func (s *K8sProxyTestSuite) TestK8sProxyFetchesNamespacesFromLocalCluster() {
	url := fmt.Sprintf("https://%s/k8s/proxy/local/api/v1/namespaces", s.client.WranglerContext.RESTConfig.Host)

	httpClient := s.httpClient()
	resp, err := httpClient.Get(url)
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var payload map[string]any
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&payload))
	s.Require().Equal("NamespaceList", payload["kind"])
	_, ok := payload["items"]
	s.Require().True(ok)
}

func (s *K8sProxyTestSuite) TestK8sProxyFetchesNamespacesFromDownstreamCluster() {
	url := fmt.Sprintf("https://%s/k8s/proxy/%s/api/v1/namespaces", s.client.WranglerContext.RESTConfig.Host, s.downstreamClusterID)
	httpClient := s.httpClient()

	// Wrap in Eventually to handle transient proxy unavailability against a downstream cluster.
	var payload map[string]any
	s.Require().Eventually(func() bool {
		resp, err := httpClient.Get(url)
		if err != nil {
			return false
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return false
		}

		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return false
		}
		return true
	}, 2*time.Minute, 5*time.Second, "timed out waiting for downstream cluster proxy to return a successful response")

	s.Require().Equal("NamespaceList", payload["kind"])
	_, ok := payload["items"]
	s.Require().True(ok)
}

func (s *K8sProxyTestSuite) TestProxyK8sV1PathReturnsNotFound() {
	url := fmt.Sprintf("https://%s/k8s/proxy/%s/v1", s.client.WranglerContext.RESTConfig.Host, s.downstreamClusterID)
	httpClient := s.httpClient()

	resp, err := httpClient.Get(url)
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusNotFound, resp.StatusCode)
}

func TestK8sProxy(t *testing.T) {
	suite.Run(t, new(K8sProxyTestSuite))
}
