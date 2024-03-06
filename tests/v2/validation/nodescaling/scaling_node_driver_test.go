//go:build (validation || infra.rke2k3s || cluster.nodedriver || extended) && !infra.any && !infra.aks && !infra.eks && !infra.gke && !infra.rke1 && !cluster.any && !cluster.custom && !sanity && !stress

package nodescaling

import (
	"testing"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/machinepools"
	"github.com/rancher/shepherd/extensions/scalinginput"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type NodeScalingTestSuite struct {
	suite.Suite
	client        *rancher.Client
	session       *session.Session
	scalingConfig *scalinginput.Config
}

func (s *NodeScalingTestSuite) TearDownSuite() {
	s.session.Cleanup()
}

func (s *NodeScalingTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	s.scalingConfig = new(scalinginput.Config)
	config.LoadConfig(scalinginput.ConfigurationFileKey, s.scalingConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(s.T(), err)

	s.client = client
}

func (s *NodeScalingTestSuite) TestScalingNodePools() {
	nodeRolesEtcd := machinepools.NodeRoles{
		Etcd:     true,
		Quantity: 1,
	}

	nodeRolesControlPlane := machinepools.NodeRoles{
		ControlPlane: true,
		Quantity:     1,
	}

	nodeRolesWorker := machinepools.NodeRoles{
		Worker:   true,
		Quantity: 1,
	}

	nodeRolesTwoWorkers := machinepools.NodeRoles{
		Worker:   true,
		Quantity: 2,
	}

	tests := []struct {
		name      string
		nodeRoles machinepools.NodeRoles
		client    *rancher.Client
	}{
		{"Scaling control plane by 1", nodeRolesControlPlane, s.client},
		{"Scaling etcd by 1", nodeRolesEtcd, s.client},
		{"Scaling worker by 1", nodeRolesWorker, s.client},
		{"Scaling worker by 2", nodeRolesTwoWorkers, s.client},
	}

	for _, tt := range tests {
		clusterID, err := clusters.GetV1ProvisioningClusterByName(s.client, s.client.RancherConfig.ClusterName)
		require.NoError(s.T(), err)

		s.Run(tt.name, func() {
			scalingRKE2K3SNodePools(s.T(), s.client, clusterID, tt.nodeRoles)
		})
	}
}

func (s *NodeScalingTestSuite) TestScalingNodePoolsDynamicInput() {
	if s.scalingConfig.MachinePools == nil {
		s.T().Skip()
	}

	clusterID, err := clusters.GetV1ProvisioningClusterByName(s.client, s.client.RancherConfig.ClusterName)
	require.NoError(s.T(), err)

	scalingRKE2K3SNodePools(s.T(), s.client, clusterID, *s.scalingConfig.MachinePools.NodeRoles)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestNodeScalingTestSuite(t *testing.T) {
	suite.Run(t, new(NodeScalingTestSuite))
}
