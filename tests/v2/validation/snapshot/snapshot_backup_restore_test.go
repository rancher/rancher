package snapshot

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/kubernetesversions"
	"github.com/rancher/rancher/tests/framework/extensions/provisioninginput"

	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type RKE2SnapshotRestoreTestSuite struct {
	suite.Suite
	session        *session.Session
	client         *rancher.Client
	ns             string
	clustersConfig *provisioninginput.Config
}

func (r *RKE2SnapshotRestoreTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *RKE2SnapshotRestoreTestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession

	r.ns = defaultNamespace

	r.clustersConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, r.clustersConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)

	r.client = client

}

func (r *RKE2SnapshotRestoreTestSuite) TestOnlySnapshotRestore() {
	r.Run("rke2-snapshot-restore", func() {
		SnapshotRestore(r.T(), r.client, r.client.RancherConfig.ClusterName, "", false)
	})
}

func (r *RKE2SnapshotRestoreTestSuite) TestSnapshotRestoreWithK8sUpgrade() {
	availableVersions, err := kubernetesversions.Default(r.client, clusters.RKE2ClusterType.String(), nil)
	require.NoError(r.T(), err)
	upgrade := availableVersions[0]
	if len(r.clustersConfig.RKE2KubernetesVersions) == 2 {
		upgrade = r.clustersConfig.RKE2KubernetesVersions[1]
	}
	r.Run("rke2-snapshot-restore-with-k8s-upgrade", func() {
		SnapshotRestore(r.T(), r.client, r.client.RancherConfig.ClusterName, upgrade, false)
	})
}

func (r *RKE2SnapshotRestoreTestSuite) TestSnapshotRestoreWithUpgradeStrategy() {
	availableVersions, err := kubernetesversions.Default(r.client, clusters.RKE2ClusterType.String(), nil)
	require.NoError(r.T(), err)
	upgrade := availableVersions[0]
	if len(r.clustersConfig.RKE2KubernetesVersions) == 2 {
		upgrade = r.clustersConfig.RKE2KubernetesVersions[1]
	}
	r.Run("rke2-snapshot-restore-with-upgrade-strategy", func() {
		SnapshotRestore(r.T(), r.client, r.client.RancherConfig.ClusterName, upgrade, true)
	})
}

func TestRKE2SnapshotRestore(t *testing.T) {
	suite.Run(t, new(RKE2SnapshotRestoreTestSuite))
}
