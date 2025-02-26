//go:build validation

package upgrade

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/upgradeinput"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/rancher/norman/types"
	"github.com/rancher/shepherd/extensions/clusters"
)

var verifyIngress = true

type UpgradeWorkloadTestSuite struct {
	suite.Suite
	session  *session.Session
	client   *rancher.Client
	clusters []upgradeinput.Cluster
}

func (u *UpgradeWorkloadTestSuite) TearDownSuite() {
	u.session.Cleanup()
}

func (u *UpgradeWorkloadTestSuite) SetupSuite() {
	testSession := session.NewSession()
	u.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(u.T(), err)

	u.client = client

	clusters, err := upgradeinput.LoadUpgradeKubernetesConfig(client)
	require.NoError(u.T(), err)

	u.clusters = clusters
}

func (u *UpgradeWorkloadTestSuite) TestWorkloadPreUpgrade() {
	var nodeSelector = make(map[string]string)
	for _, cluster := range u.clusters {
		cluster := cluster
		testName := "Pre Upgrade checks for the cluster " + cluster.Name
		u.Run(testName, func() {
			cluster.FeaturesToTest.Ingress = &verifyIngress
			clusterID, err := clusters.GetClusterIDByName(u.client, cluster.Name)
			require.NoError(u.T(), err, "Error getting cluster ID")
			nodes, err := u.client.Management.Node.ListAll(&types.ListOpts{
				Filters: map[string]interface{}{
					"clusterId": clusterID,
				},
			})
			require.NoError(u.T(), err)

			image := containerImage
			for _, node := range nodes.Data {
				if node.Labels["kubernetes.io/os"] == "windows" {
					image = windowsContainerImage
					nodeSelector["kubernetes.io/os"] = "windows"
					break
				}
			}
			createPreUpgradeWorkloads(u.T(), u.client, cluster.Name, cluster.FeaturesToTest, nodeSelector, image)
		})
	}
}

func (u *UpgradeWorkloadTestSuite) TestWorkloadPostUpgrade() {
	for _, cluster := range u.clusters {
		cluster := cluster
		testName := "Post Upgrade checks for the cluster " + cluster.Name
		u.Run(testName, func() {
			cluster.FeaturesToTest.Ingress = &verifyIngress
			createPostUpgradeWorkloads(u.T(), u.client, cluster.Name, cluster.FeaturesToTest)
		})
	}
}

func TestWorkloadUpgradeTestSuite(t *testing.T) {
	suite.Run(t, new(UpgradeWorkloadTestSuite))
}
