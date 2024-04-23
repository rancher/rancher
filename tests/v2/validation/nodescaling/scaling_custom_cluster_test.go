//go:build (validation || infra.rke2k3s || cluster.custom || stress) && !infra.any && !infra.aks && !infra.eks && !infra.gke && !infra.rke1 && !cluster.any && !cluster.nodedriver && !sanity && !extended

package nodescaling

import (
	"strings"
	"testing"

	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/shepherd/clients/rancher"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/defaults/stevetypes"
	"github.com/rancher/shepherd/extensions/machinepools"
	"github.com/rancher/shepherd/extensions/scalinginput"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type CustomClusterNodeScalingTestSuite struct {
	suite.Suite
	client        *rancher.Client
	session       *session.Session
	scalingConfig *scalinginput.Config
}

func (s *CustomClusterNodeScalingTestSuite) TearDownSuite() {
	s.session.Cleanup()
}

func (s *CustomClusterNodeScalingTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	s.scalingConfig = new(scalinginput.Config)
	config.LoadConfig(scalinginput.ConfigurationFileKey, s.scalingConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(s.T(), err)

	s.client = client
}

func (s *CustomClusterNodeScalingTestSuite) TestScalingCustomClusterNodes() {
	nodeRolesEtcd := machinepools.NodeRoles{
		Etcd:     true,
		Quantity: 1,
	}

	nodeRolesControlPlane := machinepools.NodeRoles{
		ControlPlane: true,
		Quantity:     1,
	}

	nodeRolesEtcdControlPlane := machinepools.NodeRoles{
		Etcd:         true,
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
		{"control plane by 1", nodeRolesControlPlane, s.client},
		{"etcd by 1", nodeRolesEtcd, s.client},
		{"etcd and control plane by 1", nodeRolesEtcdControlPlane, s.client},
		{"worker by 1", nodeRolesWorker, s.client},
		{"worker by 2", nodeRolesTwoWorkers, s.client},
	}

	for _, tt := range tests {
		clusterID, err := clusters.GetV1ProvisioningClusterByName(s.client, s.client.RancherConfig.ClusterName)
		require.NoError(s.T(), err)

		cluster, err := tt.client.Steve.SteveType(stevetypes.Provisioning).ByID(clusterID)
		require.NoError(s.T(), err)

		updatedCluster := new(apisV1.Cluster)
		err = v1.ConvertToK8sType(cluster, &updatedCluster)
		require.NoError(s.T(), err)

		if strings.Contains(updatedCluster.Spec.KubernetesVersion, "rke2") {
			tt.name = "Scaling custom RKE2 " + tt.name
		} else {
			tt.name = "Scaling custom K3S " + tt.name
		}

		s.Run(tt.name, func() {
			scalingRKE2K3SCustomClusterPools(s.T(), s.client, clusterID, s.scalingConfig.NodeProvider, tt.nodeRoles)
		})
	}
}

func (s *CustomClusterNodeScalingTestSuite) TestScalingCustomClusterNodesDynamicInput() {
	if s.scalingConfig.MachinePools.NodeRoles == nil {
		s.T().Skip()
	}

	clusterID, err := clusters.GetV1ProvisioningClusterByName(s.client, s.client.RancherConfig.ClusterName)
	require.NoError(s.T(), err)

	scalingRKE2K3SCustomClusterPools(s.T(), s.client, clusterID, s.scalingConfig.NodeProvider, *s.scalingConfig.MachinePools.NodeRoles)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestCustomClusterNodeScalingTestSuite(t *testing.T) {
	suite.Run(t, new(CustomClusterNodeScalingTestSuite))
}
