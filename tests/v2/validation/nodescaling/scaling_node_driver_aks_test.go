//go:build (validation || infra.aks || extended) && !infra.any && !infra.eks && !infra.gke && !infra.rke2k3s && !infra.rke1 && !cluster.any && !cluster.custom && !cluster.nodedriver && !sanity && !stress

package nodescaling

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/scalinginput"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/clusters/aks"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type AKSNodeScalingTestSuite struct {
	suite.Suite
	client        *rancher.Client
	session       *session.Session
	scalingConfig *scalinginput.Config
}

func (s *AKSNodeScalingTestSuite) TearDownSuite() {
	s.session.Cleanup()
}

func (s *AKSNodeScalingTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	s.scalingConfig = new(scalinginput.Config)
	config.LoadConfig(scalinginput.ConfigurationFileKey, s.scalingConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(s.T(), err)

	s.client = client
}

func (s *AKSNodeScalingTestSuite) TestScalingAKSNodePools() {
	scaleOneNode := aks.NodePool{
		NodeCount: &oneNode,
	}

	scaleTwoNodes := aks.NodePool{
		NodeCount: &twoNodes,
	}

	tests := []struct {
		name     string
		aksNodes aks.NodePool
		client   *rancher.Client
	}{
		{"Scaling AKS agentpool by 1", scaleOneNode, s.client},
		{"Scaling AKS agentpool by 2", scaleTwoNodes, s.client},
	}

	for _, tt := range tests {
		clusterID, err := clusters.GetClusterIDByName(s.client, s.client.RancherConfig.ClusterName)
		require.NoError(s.T(), err)

		s.Run(tt.name, func() {
			scalingAKSNodePools(s.T(), s.client, clusterID, &tt.aksNodes)
		})
	}
}

func (s *AKSNodeScalingTestSuite) TestScalingAKSNodePoolsDynamicInput() {
	if s.scalingConfig.AKSNodePool == nil {
		s.T().Skip()
	}

	clusterID, err := clusters.GetClusterIDByName(s.client, s.client.RancherConfig.ClusterName)
	require.NoError(s.T(), err)

	scalingAKSNodePools(s.T(), s.client, clusterID, s.scalingConfig.AKSNodePool)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestAKSNodeScalingTestSuite(t *testing.T) {
	t.Skip("This test has been deprecated; check https://github.com/rancher/hosted-providers-e2e for updated tests")
	suite.Run(t, new(AKSNodeScalingTestSuite))
}
