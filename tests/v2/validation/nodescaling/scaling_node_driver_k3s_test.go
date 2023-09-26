package nodescaling

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	"github.com/rancher/rancher/tests/framework/extensions/scalinginput"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type K3SNodeScalingTestSuite struct {
	suite.Suite
	client        *rancher.Client
	session       *session.Session
	scalingConfig *scalinginput.Config
}

func (s *K3SNodeScalingTestSuite) TearDownSuite() {
	s.session.Cleanup()
}

func (s *K3SNodeScalingTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	s.scalingConfig = new(scalinginput.Config)
	config.LoadConfig(scalinginput.ConfigurationFileKey, s.scalingConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(s.T(), err)

	s.client = client
}

func (s *K3SNodeScalingTestSuite) TestScalingK3SNodePools() {
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
		{"Scaling control plane machine pool by 1", nodeRolesEtcd, s.client},
		{"Scaling etcd node machine pool by 1", nodeRolesControlPlane, s.client},
		{"Scaling worker node machine pool by 1", nodeRolesWorker, s.client},
		{"Scaling worker node machine pool by 2", nodeRolesTwoWorkers, s.client},
	}

	for _, tt := range tests {
		clusterID, err := clusters.GetV1ProvisioningClusterByName(s.client, s.client.RancherConfig.ClusterName)
		require.NoError(s.T(), err)

		s.Run(tt.name, func() {
			ScalingRKE2K3SNodePools(s.T(), s.client, clusterID, tt.nodeRoles)
		})
	}
}

func (s *K3SNodeScalingTestSuite) TestScalingK3SNodePoolsDynamicInput() {
	if s.scalingConfig.NodesAndRoles == nil {
		s.T().Skip()
	}

	clusterID, err := clusters.GetV1ProvisioningClusterByName(s.client, s.client.RancherConfig.ClusterName)
	require.NoError(s.T(), err)

	ScalingRKE2K3SNodePools(s.T(), s.client, clusterID, *s.scalingConfig.NodesAndRoles)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestK3SNodeScalingTestSuite(t *testing.T) {
	suite.Run(t, new(K3SNodeScalingTestSuite))
}
