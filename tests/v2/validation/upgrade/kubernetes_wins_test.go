//go:build validation

package upgrade

import (
	"testing"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/v2/actions/clusters"
	"github.com/rancher/rancher/tests/v2/actions/upgradeinput"
	"github.com/rancher/shepherd/clients/rancher"
	extClusters "github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type UpgradeWindowsKubernetesTestSuite struct {
	suite.Suite
	session  *session.Session
	client   *rancher.Client
	clusters []upgradeinput.Cluster
}

func (u *UpgradeWindowsKubernetesTestSuite) TearDownSuite() {
	u.session.Cleanup()
}

func (u *UpgradeWindowsKubernetesTestSuite) SetupSuite() {
	testSession := session.NewSession()
	u.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(u.T(), err)

	u.client = client

	clusters, err := upgradeinput.LoadUpgradeKubernetesConfig(client)
	require.NoError(u.T(), err)

	u.clusters = clusters
}

func (u *UpgradeWindowsKubernetesTestSuite) TestUpgradeWindowsKubernetes() {
	tests := []struct {
		name         string
		client       *rancher.Client
		nodeSelector map[string]string
	}{
		{"Upgrading Windows ", u.client, map[string]string{"kubernetes.io/os": "windows"}},
	}

	for _, tt := range tests {
		for _, cluster := range u.clusters {
			updatedClusterID, err := extClusters.GetClusterIDByName(tt.client, cluster.Name)
			require.NoError(u.T(), err)

			nodes, err := tt.client.Management.Node.ListAll(&types.ListOpts{
				Filters: map[string]interface{}{
					"clusterId": updatedClusterID,
				},
			})

			for _, node := range nodes.Data {
				if tt.nodeSelector["kubernetes.io/os"] == "windows" {
					node.Labels["kubernetes.io/os"] = "windows"
				}
			}

			testConfig := clusters.ConvertConfigToClusterConfig(&cluster.ProvisioningInput)
			upgradeDownstreamCluster(&u.Suite, tt.name, tt.client, cluster.Name, testConfig, cluster, tt.nodeSelector, windowsContainerImage)
		}
	}
}

func TestWindowsKubernetesUpgradeTestSuite(t *testing.T) {
	suite.Run(t, new(UpgradeWindowsKubernetesTestSuite))
}
