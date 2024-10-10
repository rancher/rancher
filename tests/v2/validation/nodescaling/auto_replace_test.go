//go:build (validation || extended) && !infra.any && !infra.aks && !infra.eks && !infra.gke && !infra.rke2k3s && !cluster.any && !cluster.custom && !cluster.nodedriver && !sanity && !stress

package nodescaling

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/scalinginput"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/rancherversion"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	fleetNamespace        = "fleet-default"
	deletingState         = "deleting"
	machineNameAnnotation = "cluster.x-k8s.io/machine"
)

type AutoReplaceSuite struct {
	suite.Suite
	client        *rancher.Client
	session       *session.Session
	rancherConfig *rancherversion.Config
}

func (s *AutoReplaceSuite) TearDownSuite() {
	s.session.Cleanup()
}

func (s *AutoReplaceSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	rancherConfig := new(rancherversion.Config)
	config.LoadConfig(rancherversion.ConfigurationFileKey, rancherConfig)
	s.rancherConfig = rancherConfig

	client, err := rancher.NewClient("", testSession)
	require.NoError(s.T(), err)

	s.client = client
}

func (s *AutoReplaceSuite) TestEtcdAutoReplaceRKE2K3S() {
	err := scalinginput.AutoReplaceFirstNodeWithRole(s.client, s.client.RancherConfig.ClusterName, "etcd")
	require.NoError(s.T(), err)
}

func (s *AutoReplaceSuite) TestControlPlaneAutoReplaceRKE2K3S() {
	err := scalinginput.AutoReplaceFirstNodeWithRole(s.client, s.client.RancherConfig.ClusterName, "control-plane")
	require.NoError(s.T(), err)
}

func (s *AutoReplaceSuite) TestWorkerAutoReplaceRKE2K3S() {
	err := scalinginput.AutoReplaceFirstNodeWithRole(s.client, s.client.RancherConfig.ClusterName, "worker")
	require.NoError(s.T(), err)
}

func TestAutoReplaceSuite(t *testing.T) {
	suite.Run(t, new(AutoReplaceSuite))
}
