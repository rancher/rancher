//go:build (infra.rke2k3s || validation) && !infra.any && !infra.aks && !infra.eks && !infra.gke && !infra.rke1 && !stress && !sanity && !extended

package deleting

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/provisioning"
	"github.com/rancher/rancher/tests/framework/pkg/session"
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
	clusterID, err := clusters.GetV1ProvisioningClusterByName(c.client, c.client.RancherConfig.ClusterName)
	require.NoError(c.T(), err)

	clusters.DeleteK3SRKE2Cluster(c.client, clusterID)
	provisioning.VerifyDeleteRKE2K3SCluster(c.T(), c.client, clusterID)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestClusterDeleteTestSuite(t *testing.T) {
	suite.Run(t, new(ClusterDeleteTestSuite))
}
