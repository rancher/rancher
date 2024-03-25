//go:build validation || extended

package upgrade

import (
	"testing"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	providerLabel = "provider.cattle.io"
)

type MigrateCloudProviderSuite struct {
	suite.Suite
	session  *session.Session
	client   *rancher.Client
	clusters []string
}

func (u *MigrateCloudProviderSuite) TearDownSuite() {
	u.session.Cleanup()
}

func (u *MigrateCloudProviderSuite) SetupSuite() {
	testSession := session.NewSession()
	u.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(u.T(), err)

	u.client = client
}

func (u *MigrateCloudProviderSuite) TestAWS() {
	_, steveClusterObject, err := clusters.GetProvisioningClusterByName(u.client, u.client.RancherConfig.ClusterName, fleetNamespace)
	if err != nil {
		u.Run("RKE1", func() {
			rke1AWSCloudProviderMigration(u.T(), u.client, u.client.RancherConfig.ClusterName)
		})
	} else {
		u.Run("RKE2", func() {
			rke2AWSCloudProviderMigration(u.T(), u.client, steveClusterObject)
		})
	}
}

func TestCloudProviderMigrationTestSuite(t *testing.T) {
	suite.Run(t, new(MigrateCloudProviderSuite))
}
