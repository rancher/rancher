//go:build (validation || extended || infra.any || cluster.any) && !sanity && !stress

package snapshot

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/etcdsnapshot"

	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SnapshotRestoreK8sUpgradeTestSuite struct {
	suite.Suite
	session        *session.Session
	client         *rancher.Client
	clustersConfig *etcdsnapshot.Config
}

func (s *SnapshotRestoreK8sUpgradeTestSuite) TearDownSuite() {
	s.session.Cleanup()
}

func (s *SnapshotRestoreK8sUpgradeTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	s.clustersConfig = new(etcdsnapshot.Config)
	config.LoadConfig(etcdsnapshot.ConfigurationFileKey, s.clustersConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(s.T(), err)

	s.client = client
}

func (s *SnapshotRestoreK8sUpgradeTestSuite) TestSnapshotRestoreK8sUpgrade() {
	snapshotRestoreK8sVersion := &etcdsnapshot.Config{
		UpgradeKubernetesVersion: s.clustersConfig.UpgradeKubernetesVersion,
		SnapshotRestore:          "kubernetesVersion",
	}

	snapshotRestoreAll := &etcdsnapshot.Config{
		UpgradeKubernetesVersion: s.clustersConfig.UpgradeKubernetesVersion,
		SnapshotRestore:          "all",
	}

	tests := []struct {
		name         string
		etcdSnapshot *etcdsnapshot.Config
		client       *rancher.Client
	}{
		{"Restore Kubernetes version and etcd", snapshotRestoreK8sVersion, s.client},
		{"Restore cluster config, Kubernetes version and etcd", snapshotRestoreAll, s.client},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			snapshotRestore(s.T(), s.client, s.client.RancherConfig.ClusterName, tt.etcdSnapshot, false)
		})
	}
}

func (s *SnapshotRestoreK8sUpgradeTestSuite) TestSnapshotRestoreK8sUpgradeDynamicInput() {
	snapshotRestore(s.T(), s.client, s.client.RancherConfig.ClusterName, s.clustersConfig, false)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSnapshotRestoreK8sUpgradeTestSuite(t *testing.T) {
	suite.Run(t, new(SnapshotRestoreK8sUpgradeTestSuite))
}
