//go:build (validation || infra.eks || extended) && !infra.any && !infra.aks && !infra.gke && !infra.rke2k3s && !infra.rke1 && !cluster.any && !cluster.custom && !cluster.nodedriver && !sanity && !stress

package nodescaling

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/scalinginput"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/clusters/eks"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/session"
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
		{"Scaling EKS node group by 1", scaleOneNode, s.client},
		{"Scaling EKS node group by 2", scaleTwoNodes, s.client},
	}

	for _, tt := range tests {
		clusterID, err := clusters.GetClusterIDByName(s.client, s.client.RancherConfig.ClusterName)
		require.NoError(s.T(), err)

		s.Run(tt.name, func() {
			scalingEKSNodePools(s.T(), s.client, clusterID, &tt.eksNodes)
		})
	}
}

func (s *EKSNodeScalingTestSuite) TestScalingEKSNodePoolsDynamicInput() {
	if s.scalingConfig.EKSNodePool == nil {
		s.T().Skip()
	}

	clusterID, err := clusters.GetClusterIDByName(s.client, s.client.RancherConfig.ClusterName)
	require.NoError(s.T(), err)

	scalingEKSNodePools(s.T(), s.client, clusterID, s.scalingConfig.EKSNodePool)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestEKSNodeScalingTestSuite(t *testing.T) {
	t.Skip("This test has been deprecated; check https://github.com/rancher/hosted-providers-e2e for updated tests")
	suite.Run(t, new(EKSNodeScalingTestSuite))
}
