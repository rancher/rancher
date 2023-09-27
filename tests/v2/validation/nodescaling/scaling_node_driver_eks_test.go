package nodescaling

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/eks"
	"github.com/rancher/rancher/tests/framework/extensions/scalinginput"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type EKSNodeScalingTestSuite struct {
	suite.Suite
	client        *rancher.Client
	session       *session.Session
	scalingConfig *scalinginput.Config
}

func (s *EKSNodeScalingTestSuite) TearDownSuite() {
	s.session.Cleanup()
}

func (s *EKSNodeScalingTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	s.scalingConfig = new(scalinginput.Config)
	config.LoadConfig(scalinginput.ConfigurationFileKey, s.scalingConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(s.T(), err)

	s.client = client
}

func (s *EKSNodeScalingTestSuite) TestScalingEKSNodePools() {
	scaleOneNode := eks.NodeGroupConfig{
		DesiredSize: &oneNode,
	}

	scaleTwoNodes := eks.NodeGroupConfig{
		DesiredSize: &twoNodes,
	}

	tests := []struct {
		name     string
		eksNodes eks.NodeGroupConfig
		client   *rancher.Client
	}{
		{"Scaling node group by 1", scaleOneNode, s.client},
		{"Scaling node group by 2", scaleTwoNodes, s.client},
	}

	for _, tt := range tests {
		clusterID, err := clusters.GetClusterIDByName(s.client, s.client.RancherConfig.ClusterName)
		require.NoError(s.T(), err)

		s.Run(tt.name, func() {
			ScalingEKSNodePools(s.T(), s.client, clusterID, &tt.eksNodes)
		})
	}
}

func (s *EKSNodeScalingTestSuite) TestScalingEKSNodePoolsDynamicInput() {
	if s.scalingConfig.EKSNodePool == nil {
		s.T().Skip()
	}

	clusterID, err := clusters.GetClusterIDByName(s.client, s.client.RancherConfig.ClusterName)
	require.NoError(s.T(), err)

	ScalingEKSNodePools(s.T(), s.client, clusterID, s.scalingConfig.EKSNodePool)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestEKSNodeScalingTestSuite(t *testing.T) {
	suite.Run(t, new(EKSNodeScalingTestSuite))
}
