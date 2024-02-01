//go:build (validation || extended || infra.any || cluster.any) && !sanity && !stress

package snapshot

import (
	"testing"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/etcdsnapshot"

	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/session"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SnapshotRestoreTestSuite struct {
	suite.Suite
	session        *session.Session
	client         *rancher.Client
	clustersConfig *etcdsnapshot.Config
}

func (s *SnapshotRestoreTestSuite) TearDownSuite() {
	s.session.Cleanup()
}

func (s *SnapshotRestoreTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	s.clustersConfig = new(etcdsnapshot.Config)
	config.LoadConfig(etcdsnapshot.ConfigurationFileKey, s.clustersConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(s.T(), err)

	s.client = client
}

func (s *SnapshotRestoreTestSuite) TestSnapshotRestoreETCDOnly() {
	snapshotRestoreNone := &etcdsnapshot.Config{
		UpgradeKubernetesVersion: "",
		SnapshotRestore:          "none",
		RecurringRestores:        1,
	}

	tests := []struct {
		name         string
		etcdSnapshot *etcdsnapshot.Config
		client       *rancher.Client
	}{
		{"Restore etcd only", snapshotRestoreNone, s.client},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			snapshotRestore(s.T(), s.client, s.client.RancherConfig.ClusterName, tt.etcdSnapshot)
		})
	}
}

func (s *SnapshotRestoreTestSuite) TestSnapshotRestoreETCDOnlyDynamicInput() {
	snapshotRestore(s.T(), s.client, s.client.RancherConfig.ClusterName, s.clustersConfig)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSnapshotRestoreTestSuite(t *testing.T) {
	suite.Run(t, new(SnapshotRestoreTestSuite))
}
