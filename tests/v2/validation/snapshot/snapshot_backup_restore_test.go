//go:build (validation || infra.rke2k3s || cluster.any || stress) && !infra.any && !infra.aks && !infra.eks && !infra.gke && !infra.rke1 && !sanity && !extended

package snapshot

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/provisioninginput"

	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SnapshotRestoreTestSuite struct {
	suite.Suite
	session        *session.Session
	client         *rancher.Client
	ns             string
	clustersConfig *provisioninginput.Config
}

func (r *SnapshotRestoreTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *SnapshotRestoreTestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession

	r.ns = defaultNamespace

	r.clustersConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, r.clustersConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)

	r.client = client
}

func (r *SnapshotRestoreTestSuite) TestOnlySnapshotRestore() {
	r.Run("snapshot-restore", func() {
		subSession := r.session.NewSession()
		defer subSession.Cleanup()

		client, err := r.client.WithSession(subSession)
		require.NoError(r.T(), err)

		snapshotRestore(r.T(), client, r.client.RancherConfig.ClusterName, false, false)
	})
}

func (r *SnapshotRestoreTestSuite) TestSnapshotRestoreWithK8sUpgrade() {
	r.Run("snapshot-restore-with-k8s-upgrade", func() {
		subSession := r.session.NewSession()
		defer subSession.Cleanup()

		client, err := r.client.WithSession(subSession)
		require.NoError(r.T(), err)

		snapshotRestore(r.T(), client, r.client.RancherConfig.ClusterName, true, false)
	})
}

func (r *SnapshotRestoreTestSuite) TestSnapshotRestoreWithUpgradeStrategy() {
	r.Run("snapshot-restore-with-upgrade-strategy", func() {
		subSession := r.session.NewSession()
		defer subSession.Cleanup()

		client, err := r.client.WithSession(subSession)
		require.NoError(r.T(), err)

		snapshotRestore(r.T(), client, r.client.RancherConfig.ClusterName, true, true)
	})
}

func TestSnapshotRestore(t *testing.T) {
	suite.Run(t, new(SnapshotRestoreTestSuite))
}
