//go:build validation

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

type SnapshotAdditionalTestsTestSuite struct {
	suite.Suite
	session        *session.Session
	client         *rancher.Client
	clustersConfig *etcdsnapshot.Config
}

func (s *SnapshotAdditionalTestsTestSuite) TearDownSuite() {
	s.session.Cleanup()
}

func (s *SnapshotAdditionalTestsTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	s.clustersConfig = new(etcdsnapshot.Config)
	config.LoadConfig(etcdsnapshot.ConfigurationFileKey, s.clustersConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(s.T(), err)

	s.client = client
}

func (s *SnapshotAdditionalTestsTestSuite) TestSnapshotReplaceWorkerNode() {
	snapshotRestoreAll := &etcdsnapshot.Config{
		UpgradeKubernetesVersion: "",
		SnapshotRestore:          "all",
		RecurringRestores:        1,
		ReplaceWorkerNode:        true,
	}

	snapshotRestoreK8sVersion := &etcdsnapshot.Config{
		UpgradeKubernetesVersion: "",
		SnapshotRestore:          "kubernetesVersion",
		RecurringRestores:        1,
		ReplaceWorkerNode:        true,
	}

	snapshotRestoreNone := &etcdsnapshot.Config{
		UpgradeKubernetesVersion: "",
		SnapshotRestore:          "none",
		RecurringRestores:        1,
		ReplaceWorkerNode:        true,
	}

	tests := []struct {
		name         string
		etcdSnapshot *etcdsnapshot.Config
		client       *rancher.Client
	}{
		{"Replace worker nodes and restore cluster config, Kubernetes version and etcd", snapshotRestoreAll, s.client},
		{"Replace worker nodes and restore Kubernetes version and etcd", snapshotRestoreK8sVersion, s.client},
		{"Replace worker nodes and restore etcd only", snapshotRestoreNone, s.client},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			snapshotRestore(s.T(), s.client, s.client.RancherConfig.ClusterName, tt.etcdSnapshot)
		})
	}
}

func (s *SnapshotAdditionalTestsTestSuite) TestSnapshotRecurringRestores() {
	snapshotRestoreFiveTimes := &etcdsnapshot.Config{
		UpgradeKubernetesVersion: "",
		SnapshotRestore:          "all",
		RecurringRestores:        5,
		ReplaceWorkerNode:        false,
	}

	tests := []struct {
		name         string
		etcdSnapshot *etcdsnapshot.Config
		client       *rancher.Client
	}{
		{"Restore snapshot 5 times", snapshotRestoreFiveTimes, s.client},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			snapshotRestore(s.T(), s.client, s.client.RancherConfig.ClusterName, tt.etcdSnapshot)
		})
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSnapshotAdditionalTestsTestSuite(t *testing.T) {
	suite.Run(t, new(SnapshotAdditionalTestsTestSuite))
}
