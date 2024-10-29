//go:build (validation || infra.rke1 || cluster.nodedriver || extended) && !infra.any && !infra.aks && !infra.eks && !infra.gke && !infra.rke2k3s && !cluster.any && !cluster.custom && !sanity && !stress

package nodescaling

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	nodepools "github.com/rancher/rancher/tests/v2/actions/rke1/nodepools"
	"github.com/rancher/rancher/tests/v2/actions/scalinginput"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type RKE1NodeReplacingTestSuite struct {
	suite.Suite
	session        *session.Session
	client         *rancher.Client
	ns             string
	clustersConfig *provisioninginput.Config
}

func (s *RKE1NodeReplacingTestSuite) TearDownSuite() {
	s.session.Cleanup()
}

func (s *RKE1NodeReplacingTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	s.ns = provisioninginput.Namespace

	s.clustersConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, s.clustersConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(s.T(), err)

	s.client = client
}

func (s *RKE1NodeReplacingTestSuite) TestReplacingRKE1Nodes() {
	nodeRolesEtcd := nodepools.NodeRoles{
		Etcd:         true,
		ControlPlane: false,
		Worker:       false,
	}

	nodeRolesControlPlane := nodepools.NodeRoles{
		Etcd:         false,
		ControlPlane: true,
		Worker:       false,
	}

	nodeRolesWorker := nodepools.NodeRoles{
		Etcd:         false,
		ControlPlane: false,
		Worker:       true,
	}

	tests := []struct {
		name      string
		nodeRoles nodepools.NodeRoles
		client    *rancher.Client
	}{
		{"Replacing control plane nodes", nodeRolesControlPlane, s.client},
		{"Replacing etcd nodes", nodeRolesEtcd, s.client},
		{"Replacing worker nodes", nodeRolesWorker, s.client},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			err := scalinginput.ReplaceRKE1Nodes(s.client, s.client.RancherConfig.ClusterName, tt.nodeRoles.Etcd, tt.nodeRoles.ControlPlane, tt.nodeRoles.Worker)
			require.NoError(s.T(), err)
		})
	}
}

func TestRKE1NodeReplacingTestSuite(t *testing.T) {
	suite.Run(t, new(RKE1NodeReplacingTestSuite))
}
