//go:build validation

package upgrade

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/clusters"
	"github.com/rancher/rancher/tests/v2/actions/upgradeinput"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type UpgradeKubernetesTestSuite struct {
	suite.Suite
	session  *session.Session
	client   *rancher.Client
	clusters []upgradeinput.Cluster
}

func (u *UpgradeKubernetesTestSuite) TearDownSuite() {
	u.session.Cleanup()
}

func (u *UpgradeKubernetesTestSuite) SetupSuite() {
	testSession := session.NewSession()
	u.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(u.T(), err)

	u.client = client

	clusters, err := upgradeinput.LoadUpgradeKubernetesConfig(client)
	require.NoError(u.T(), err)

	u.clusters = clusters
}

func (u *UpgradeKubernetesTestSuite) TestUpgradeKubernetes() {
	tests := []struct {
		name   string
		client *rancher.Client
	}{
		{"Upgrading ", u.client},
	}

	for _, tt := range tests {
		for _, cluster := range u.clusters {
			testConfig := clusters.ConvertConfigToClusterConfig(&cluster.ProvisioningInput)

			if cluster.Name == local {
				upgradeLocalCluster(&u.Suite, tt.name, tt.client, testConfig, cluster, containerImage)
			} else {
				upgradeDownstreamCluster(&u.Suite, tt.name, tt.client, cluster.Name, testConfig, cluster, nil, containerImage)
			}
		}
	}
}

func TestKubernetesUpgradeTestSuite(t *testing.T) {
	suite.Run(t, new(UpgradeKubernetesTestSuite))
}
