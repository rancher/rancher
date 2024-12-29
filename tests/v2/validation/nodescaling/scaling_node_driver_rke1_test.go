//go:build (validation || infra.rke1 || cluster.nodedriver || extended) && !infra.any && !infra.aks && !infra.eks && !infra.gke && !infra.rke2k3s && !cluster.any && !cluster.custom && !sanity && !stress

package nodescaling

import (
	"testing"

	nodepools "github.com/rancher/rancher/tests/v2/actions/rke1/nodepools"
	"github.com/rancher/rancher/tests/v2/actions/scalinginput"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type RKE1NodeScalingTestSuite struct {
	suite.Suite
	client        *rancher.Client
	session       *session.Session
	scalingConfig *scalinginput.Config
}

func (s *RKE1NodeScalingTestSuite) TearDownSuite() {
	s.session.Cleanup()
}

func (s *RKE1NodeScalingTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	s.scalingConfig = new(scalinginput.Config)
	config.LoadConfig(scalinginput.ConfigurationFileKey, s.scalingConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(s.T(), err)

	s.client = client
}

func (s *RKE1NodeScalingTestSuite) TestScalingRKE1NodePools() {
	nodeRolesEtcd := nodepools.NodeRoles{
		Etcd:     true,
		Quantity: 1,
	}

	nodeRolesControlPlane := nodepools.NodeRoles{
		ControlPlane: true,
		Quantity:     1,
	}

	nodeRolesWorker := nodepools.NodeRoles{
		Worker:   true,
		Quantity: 1,
	}

	tests := []struct {
		name      string
		nodeRoles nodepools.NodeRoles
		client    *rancher.Client
	}{
		{"Scaling control plane by 1", nodeRolesControlPlane, s.client},
		{"Scaling etcd node by 1", nodeRolesEtcd, s.client},
		{"Scaling worker by 1", nodeRolesWorker, s.client},
	}

	for _, tt := range tests {
		clusterID, err := clusters.GetClusterIDByName(s.client, s.client.RancherConfig.ClusterName)
		require.NoError(s.T(), err)

		s.Run(tt.name, func() {
			scalingRKE1NodePools(s.T(), s.client, clusterID, tt.nodeRoles)
		})
	}
}

func (s *RKE1NodeScalingTestSuite) TestScalingRKE1NodePoolsDynamicInput() {
	if s.scalingConfig.NodePools.NodeRoles == nil {
		s.T().Skip()
	}

	clusterID, err := clusters.GetClusterIDByName(s.client, s.client.RancherConfig.ClusterName)
	require.NoError(s.T(), err)

	scalingRKE1NodePools(s.T(), s.client, clusterID, *s.scalingConfig.NodePools.NodeRoles)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestRKE1NodeScalingTestSuite(t *testing.T) {
	suite.Run(t, new(RKE1NodeScalingTestSuite))
}
