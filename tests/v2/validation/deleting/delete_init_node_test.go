package deleting

import (
	"testing"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type DeleteInitNodeTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
}

func (d *DeleteInitNodeTestSuite) TearDownSuite() {
	d.session.Cleanup()
}

func (d *DeleteInitNodeTestSuite) SetupSuite() {
	testSession := session.NewSession()
	d.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(d.T(), err)

	d.client = client
}

func (d *DeleteInitNodeTestSuite) TestDeleteInitNode() {
	clusterID, err := clusters.GetClusterIDByName(d.client, d.client.RancherConfig.ClusterName)
	require.NoError(d.T(), err)

	deleteInitNode(d.T(), d.client, clusterID)
} 

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestDeleteInitNodeTestSuite(t *testing.T) {
	suite.Run(t, new(DeleteInitNodeTestSuite))
}
