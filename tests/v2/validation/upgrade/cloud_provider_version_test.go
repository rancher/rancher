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

type UpgradeCloudProviderSuite struct {
	suite.Suite
	session  *session.Session
	client   *rancher.Client
	clusters []string
}

func (u *UpgradeCloudProviderSuite) TearDownSuite() {
	u.session.Cleanup()
}

func (u *UpgradeCloudProviderSuite) SetupSuite() {
	testSession := session.NewSession()
	u.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(u.T(), err)

	u.client = client
}

func (u *UpgradeCloudProviderSuite) TestVsphere() {
	_, _, err := clusters.GetProvisioningClusterByName(u.client, u.client.RancherConfig.ClusterName, fleetNamespace)
	if err != nil {
		u.Run("RKE1", func() {
			upgradeVsphereCloudProviderCharts(u.T(), u.client, u.client.RancherConfig.ClusterName)
		})
	}
}

func TestCloudProviderVersionUpgradeSuite(t *testing.T) {
	suite.Run(t, new(UpgradeCloudProviderSuite))
}
