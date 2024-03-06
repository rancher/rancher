//go:build (validation || infra.rke1 || cluster.nodedriver || extended) && !infra.any && !infra.aks && !infra.eks && !infra.gke && !infra.rke2k3s && !cluster.any && !cluster.custom && !sanity && !stress

package nodescaling

import (
	"testing"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/provisioninginput"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type NodeScaleRKE1DownAndUp struct {
	suite.Suite
	session        *session.Session
	client         *rancher.Client
	ns             string
	clustersConfig *provisioninginput.Config
}

func (s *NodeScaleRKE1DownAndUp) TearDownSuite() {
	s.session.Cleanup()
}

func (s *NodeScaleRKE1DownAndUp) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	s.ns = provisioninginput.Namespace

	s.clustersConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, s.clustersConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(s.T(), err)

	s.client = client
}

func (s *NodeScaleRKE1DownAndUp) TestEtcdScaleDownAndUp() {
	s.Run("rke1-etcd-node-scale-down-and-up", func() {
		ReplaceRKE1Nodes(s.T(), s.client, s.client.RancherConfig.ClusterName, true, false, false)
	})
}
func (s *NodeScaleRKE1DownAndUp) TestWorkerScaleDownAndUp() {
	s.Run("rke1-worker-node-scale-down-and-up", func() {
		ReplaceRKE1Nodes(s.T(), s.client, s.client.RancherConfig.ClusterName, false, false, true)
	})
}
func (s *NodeScaleRKE1DownAndUp) TestControlPlaneScaleDownAndUp() {
	s.Run("rke1-controlplane-node-scale-down-and-up", func() {
		ReplaceRKE1Nodes(s.T(), s.client, s.client.RancherConfig.ClusterName, false, true, false)
	})
}

func TestRKE1NodeScaleDownAndUp(t *testing.T) {
	suite.Run(t, new(NodeScaleRKE1DownAndUp))
}
