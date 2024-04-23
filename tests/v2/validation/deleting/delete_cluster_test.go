//go:build (infra.rke2k3s || validation) && !infra.any && !infra.aks && !infra.eks && !infra.gke && !infra.rke1 && !stress && !sanity && !extended

package deleting

import (
	"strings"
	"testing"

	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/shepherd/clients/rancher"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/defaults/stevetypes"
	"github.com/rancher/shepherd/extensions/provisioning"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ClusterDeleteTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
}

func (c *ClusterDeleteTestSuite) TearDownSuite() {
	c.session.Cleanup()
}

func (c *ClusterDeleteTestSuite) SetupSuite() {
	testSession := session.NewSession()
	c.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(c.T(), err)

	c.client = client
}

func (c *ClusterDeleteTestSuite) TestDeletingCluster() {
	tests := []struct {
		name   string
		client *rancher.Client
	}{
		{"cluster", c.client},
	}

	for _, tt := range tests {
		clusterID, err := clusters.GetV1ProvisioningClusterByName(c.client, c.client.RancherConfig.ClusterName)
		require.NoError(c.T(), err)

		cluster, err := tt.client.Steve.SteveType(stevetypes.Provisioning).ByID(clusterID)
		require.NoError(c.T(), err)

		updatedCluster := new(apisV1.Cluster)
		err = v1.ConvertToK8sType(cluster, &updatedCluster)
		require.NoError(c.T(), err)

		if strings.Contains(updatedCluster.Spec.KubernetesVersion, "rke2") {
			tt.name = "Deleting RKE2 " + tt.name
		} else {
			tt.name = "Deleting K3S " + tt.name
		}

		c.Run(tt.name, func() {
			clusters.DeleteK3SRKE2Cluster(tt.client, clusterID)
			provisioning.VerifyDeleteRKE2K3SCluster(c.T(), tt.client, clusterID)
		})
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestClusterDeleteTestSuite(t *testing.T) {
	suite.Run(t, new(ClusterDeleteTestSuite))
}
