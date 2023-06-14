//go:build (infra.rke1 || validation) && !infra.any && !infra.aks && !infra.eks && !infra.gke && !infra.rke2k3s && !stress && !sanity && !extended

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

type RKE1ClusterDeleteTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
}

func (c *RKE1ClusterDeleteTestSuite) TearDownSuite() {
	c.session.Cleanup()
}

func (c *RKE1ClusterDeleteTestSuite) SetupSuite() {
	testSession := session.NewSession()
	c.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(c.T(), err)

	c.client = client
}

func (c *RKE1ClusterDeleteTestSuite) TestDeletingRKE1Cluster() {
	clusterID, err := clusters.GetClusterIDByName(c.client, c.client.RancherConfig.ClusterName)
	require.NoError(c.T(), err)

	clusters.DeleteRKE1Cluster(c.client, clusterID)
	provisioning.VerifyDeleteRKE1Cluster(c.T(), c.client, clusterID)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestRKE1ClusterDeleteTestSuite(t *testing.T) {
	suite.Run(t, new(RKE1ClusterDeleteTestSuite))
}
