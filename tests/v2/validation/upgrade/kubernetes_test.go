//go:build validation

package upgrade

import (
	"strings"
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/bundledclusters"
	"github.com/rancher/rancher/tests/framework/extensions/defaults"
	nodestat "github.com/rancher/rancher/tests/framework/extensions/nodes"
	"github.com/rancher/rancher/tests/framework/extensions/provisioninginput"
	psadeploy "github.com/rancher/rancher/tests/framework/extensions/psact"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	rke1KubeVersionCheck = "rancher"
	rke2KubeVersionCheck = "rke2"
	k3sKubeVersionCheck  = "k3s"
)

type UpgradeKubernetesTestSuite struct {
	suite.Suite
	session  *session.Session
	client   *rancher.Client
	clusters []Clusters
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

	clusters, err := loadUpgradeKubernetesConfig(client)
	require.NoError(u.T(), err)

	require.NotEmptyf(u.T(), clusters, "couldn't generate the config for the upgrade test")
	u.clusters = clusters
}

func (u *UpgradeKubernetesTestSuite) TestUpgradeKubernetes() {
	for _, cluster := range u.clusters {
		cluster := cluster
		u.Run(cluster.Name, func() {
			if cluster.isUpgradeDisabled {
				u.T().Skipf("Kubernetes upgrade is disabled for [%v]", cluster.Name)
			}

			u.testUpgradeSingleCluster(cluster.Name, cluster.VersionToUpgrade, cluster.PSACT, cluster.isLatestVersion)
		})
	}
}

func TestKubernetesUpgradeTestSuite(t *testing.T) {
	suite.Run(t, new(UpgradeKubernetesTestSuite))
}

func (u *UpgradeKubernetesTestSuite) testUpgradeSingleCluster(clusterName, versionToUpgrade, psact string, isLatestVersion bool) {
	subSession := u.session.NewSession()
	defer subSession.Cleanup()

	client, err := u.client.WithSession(subSession)
	require.NoError(u.T(), err)

	clusterMeta, err := clusters.NewClusterMeta(client, clusterName)
	require.NoError(u.T(), err)
	require.NotNilf(u.T(), clusterMeta, "Couldn't get the cluster meta")
	u.T().Logf("[%v]: Provider is: %v, Hosted: %v, Imported: %v ", clusterName, clusterMeta.Provider, clusterMeta.IsHosted, clusterMeta.IsImported)

	initCluster, err := bundledclusters.NewWithClusterMeta(clusterMeta)
	require.NoError(u.T(), err)

	cluster, err := initCluster.Get(client)
	require.NoError(u.T(), err)

	versions, err := cluster.ListAvailableVersions(client)
	require.NoError(u.T(), err)
	u.T().Logf("[%v]: Available versions for the cluster: %v", clusterName, versions)

	version := getVersion(u.T(), clusterName, versions, isLatestVersion, versionToUpgrade)
	require.NotNilf(u.T(), version, "Couldn't get the version")
	u.T().Logf("[%v]: Selected version: %v", clusterName, *version)

	updatedCluster, err := cluster.UpdateKubernetesVersion(client, version)
	require.NoError(u.T(), err)

	u.T().Logf("[%v]: Validating sent update request for kubernetes version of the cluster", clusterName)
	validateKubernetesVersions(u.T(), client, updatedCluster, version, isCheckingCurrentCluster)

	err = clusters.WaitClusterToBeUpgraded(client, clusterMeta.ID)
	require.NoError(u.T(), err)
	u.T().Logf("[%v]: Waiting cluster to be upgraded and ready", clusterName)

	u.T().Logf("[%v]: Validating updated cluster's kubernetes version", clusterName)
	validateKubernetesVersions(u.T(), client, updatedCluster, version, !isCheckingCurrentCluster)

	if clusterMeta.IsHosted {
		updatedCluster.UpdateNodepoolKubernetesVersions(client, version)

		u.T().Logf("[%v]: Validating sent update request for nodepools kubernetes versions of the cluster", clusterName)
		validateNodepoolVersions(u.T(), client, updatedCluster, version, isCheckingCurrentCluster)

		err = clusters.WaitClusterToBeUpgraded(client, clusterMeta.ID)
		require.NoError(u.T(), err)

		u.T().Logf("[%v]: Validating updated cluster's nodepools kubernetes versions", clusterName)
		validateNodepoolVersions(u.T(), client, updatedCluster, version, !isCheckingCurrentCluster)
	}
	if strings.Contains(versionToUpgrade, rke1KubeVersionCheck) {
		err = nodestat.AllManagementNodeReady(client, clusterMeta.ID, defaults.ThirtyMinuteTimeout)
	} else if strings.Contains(versionToUpgrade, rke2KubeVersionCheck) || strings.Contains(versionToUpgrade, k3sKubeVersionCheck) {
		err = nodestat.AllMachineReady(client, clusterMeta.ID, defaults.TenMinuteTimeout)
	}
	require.NoError(u.T(), err)

	clusterToken, err := clusters.CheckServiceAccountTokenSecret(client, clusterName)
	require.NoError(u.T(), err)
	assert.NotEmpty(u.T(), clusterToken)

	if psact == string(provisioninginput.RancherPrivileged) || psact == string(provisioninginput.RancherRestricted) || psact == string(provisioninginput.RancherBaseline) {
		err := psadeploy.CreateNginxDeployment(client, clusterMeta.ID, psact)
		require.NoError(u.T(), err)
	}

	podErrors := pods.StatusPods(client, clusterMeta.ID)
	assert.Empty(u.T(), podErrors)
}
