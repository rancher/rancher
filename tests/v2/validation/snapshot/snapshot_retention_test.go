//go:build (validation || extended || infra.any || cluster.any) && !sanity && !stress

package snapshot

import (
	"fmt"
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/etcdsnapshot"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/defaults/stevetypes"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// For SnapshotInterval this will be hours for rke1 and minutes for rke2
type SnapshotRetentionConfig struct {
	ClusterName       string `json:"clusterName" yaml:"clusterName"`
	SnapshotInterval  int    `json:"snapshotInterval" yaml:"snapshotInterval"`
	SnapshotRetention int    `json:"snapshotRetention" yaml:"snapshotRetention"`
}

type SnapshotRetentionTestSuite struct {
	suite.Suite
	session        *session.Session
	client         *rancher.Client
	snapshotConfig *SnapshotRetentionConfig
	provider       string
}

func (s *SnapshotRetentionTestSuite) TearDownSuite() {
	s.session.Cleanup()
}

func (s *SnapshotRetentionTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	s.snapshotConfig = new(SnapshotRetentionConfig)
	config.LoadConfig("retentionTest", s.snapshotConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(s.T(), err)

	s.client = client

	v1ClusterID, err := clusters.GetV1ProvisioningClusterByName(client, s.snapshotConfig.ClusterName)
	var v3ClusterID string
	if v1ClusterID == "" {
		v3ClusterID, err = clusters.GetClusterIDByName(client, s.snapshotConfig.ClusterName)
		require.NoError(s.T(), err)
		v1ClusterID = "fleet-default/" + v3ClusterID
	}

	require.NoError(s.T(), err)
	fleetCluster, err := s.client.Steve.SteveType(stevetypes.FleetCluster).ByID(v1ClusterID)
	require.NoError(s.T(), err)

	s.provider = fleetCluster.ObjectMeta.Labels["provider.cattle.io"]

	if s.provider == "rke" {
		clusterObject, err := s.client.Management.Cluster.ByID(v3ClusterID)
		require.NoError(s.T(), err)

		updatedClusterObject := clusterObject
		updatedClusterObject.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.Retention = int64(s.snapshotConfig.SnapshotRetention)
		updatedClusterObject.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.IntervalHours = int64(s.snapshotConfig.SnapshotInterval)

		_, err = s.client.Management.Cluster.Update(clusterObject, updatedClusterObject)
		require.NoError(s.T(), err)
	} else {
		if s.snapshotConfig.SnapshotInterval < 5 {
			logrus.Info("Snapshot cron schedules below 2 minutes can cause unexpected behaviors in rancher")
		}

		clusterObject, clusterResponse, err := clusters.GetProvisioningClusterByName(s.client, s.snapshotConfig.ClusterName, "fleet-default")
		require.NoError(s.T(), err)

		clusterObject.Spec.RKEConfig.ETCD.SnapshotRetention = s.snapshotConfig.SnapshotRetention
		cronSchedule := fmt.Sprintf("%s%v%s", "*/", s.snapshotConfig.SnapshotInterval, " * * * *")
		clusterObject.Spec.RKEConfig.ETCD.SnapshotScheduleCron = cronSchedule
		_, err = s.client.Steve.SteveType(stevetypes.Provisioning).Update(clusterResponse, clusterObject)
		require.NoError(s.T(), err)
	}
}

func (s *SnapshotRetentionTestSuite) TestAutomaticSnapshotRetention() {
	tests := []struct {
		testName                 string
		client                   *rancher.Client
		clusterName              string
		retentionLimit           int
		intervalBetweenSnapshots int
	}{
		{"Retention limit test", s.client, s.snapshotConfig.ClusterName, 2, 1},
	}

	for _, tt := range tests {
		s.Run(tt.testName, func() {
			config := s.snapshotConfig
			err := etcdsnapshot.CreateSnapshotsUntilRetentionLimit(s.client, config.ClusterName, config.SnapshotRetention, config.SnapshotInterval)
			require.NoError(s.T(), err)
		})
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSnapshotRetentionTestSuite(t *testing.T) {
	suite.Run(t, new(SnapshotRetentionTestSuite))
}
