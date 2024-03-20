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

type SnapshotRestoreUpgradeStrategyTestSuite struct {
	suite.Suite
	session        *session.Session
	client         *rancher.Client
	clustersConfig *etcdsnapshot.Config
}

func (s *SnapshotRestoreUpgradeStrategyTestSuite) TearDownSuite() {
	s.session.Cleanup()
}

func (s *SnapshotRestoreUpgradeStrategyTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	s.clustersConfig = new(etcdsnapshot.Config)
	config.LoadConfig(etcdsnapshot.ConfigurationFileKey, s.clustersConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(s.T(), err)

	s.client = client
}

func (s *SnapshotRestoreUpgradeStrategyTestSuite) TestSnapshotRestoreUpgradeStrategy() {
	snapshotRestoreK8sVersion := &etcdsnapshot.Config{
		UpgradeKubernetesVersion:     "",
		SnapshotRestore:              "kubernetesVersion",
		ControlPlaneConcurrencyValue: "15%",
		ControlPlaneUnavailableValue: "3",
		WorkerConcurrencyValue:       "20%",
		WorkerUnavailableValue:       "15%",
		RecurringRestores:            1,
		ReplaceWorkerNode:            false,
	}

	snapshotRestoreAll := &etcdsnapshot.Config{
		UpgradeKubernetesVersion:     "",
		SnapshotRestore:              "all",
		ControlPlaneConcurrencyValue: "15%",
		ControlPlaneUnavailableValue: "3",
		WorkerConcurrencyValue:       "20%",
		WorkerUnavailableValue:       "15%",
		RecurringRestores:            1,
		ReplaceWorkerNode:            false,
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
			snapshotRestore(s.T(), s.client, s.client.RancherConfig.ClusterName, tt.etcdSnapshot)
		})
	}
}

func (s *SnapshotRestoreUpgradeStrategyTestSuite) TestSnapshotRestoreUpgradeStrategyDynamicInput() {
	if s.clustersConfig == nil {
		s.T().Skip()
	}

	snapshotRestore(s.T(), s.client, s.client.RancherConfig.ClusterName, s.clustersConfig)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSnapshotRestoreUpgradeStrategyTestSuite(t *testing.T) {
	suite.Run(t, new(SnapshotRestoreUpgradeStrategyTestSuite))
}
