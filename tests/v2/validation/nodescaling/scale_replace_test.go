//go:build (validation || infra.rke2k3s || cluster.nodedriver || extended) && !infra.any && !infra.aks && !infra.eks && !infra.gke && !infra.rke1 && !cluster.any && !cluster.custom && !sanity && !stress

package nodescaling

import (
	"strings"
	"testing"

	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/machinepools"
	"github.com/rancher/rancher/tests/v2/actions/scalinginput"
	"github.com/rancher/shepherd/clients/rancher"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type NodeReplacingTestSuite struct {
	suite.Suite
	session *session.Session
	client  *rancher.Client
}

func (s *NodeReplacingTestSuite) TearDownSuite() {
	s.session.Cleanup()
}

func (s *NodeReplacingTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(s.T(), err)

	s.client = client
}

func (s *NodeReplacingTestSuite) TestReplacingNodes() {
	nodeRolesEtcd := machinepools.NodeRoles{
		Etcd: true,
	}

	nodeRolesControlPlane := machinepools.NodeRoles{
		ControlPlane: true,
	}

	nodeRolesWorker := machinepools.NodeRoles{
		Worker: true,
	}

	tests := []struct {
		name      string
		nodeRoles machinepools.NodeRoles
		client    *rancher.Client
	}{
		{"control plane nodes", nodeRolesControlPlane, s.client},
		{"etcd nodes", nodeRolesEtcd, s.client},
		{"worker nodes", nodeRolesWorker, s.client},
	}

	for _, tt := range tests {
		clusterID, err := clusters.GetV1ProvisioningClusterByName(s.client, s.client.RancherConfig.ClusterName)
		require.NoError(s.T(), err)

		cluster, err := tt.client.Steve.SteveType(ProvisioningSteveResourceType).ByID(clusterID)
		require.NoError(s.T(), err)

		updatedCluster := new(apisV1.Cluster)
		err = v1.ConvertToK8sType(cluster, &updatedCluster)
		require.NoError(s.T(), err)

		if strings.Contains(updatedCluster.Spec.KubernetesVersion, "rke2") {
			tt.name = "Replacing RKE2 " + tt.name
		} else {
			tt.name = "Replacing K3S " + tt.name
		}

		s.Run(tt.name, func() {
			err := scalinginput.ReplaceNodes(s.client, s.client.RancherConfig.ClusterName, tt.nodeRoles.Etcd, tt.nodeRoles.ControlPlane, tt.nodeRoles.Worker)
			require.NoError(s.T(), err)
		})
	}
}

func TestNodeReplacingTestSuite(t *testing.T) {
	suite.Run(t, new(NodeReplacingTestSuite))
}
