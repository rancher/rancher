//go:build (validation || extended || infra.any || cluster.any) && !sanity && !stress

package snapshot

import (
	"strings"
	"testing"

	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/shepherd/clients/rancher"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/etcdsnapshot"

	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/session"

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
		UpgradeKubernetesVersion: "",
		SnapshotRestore:          "kubernetesVersion",
		RecurringRestores:        1,
	}

	tests := []struct {
		name         string
		etcdSnapshot *etcdsnapshot.Config
		client       *rancher.Client
	}{
		{"Restore Kubernetes version and etcd", snapshotRestoreK8sVersion, s.client},
	}

	for _, tt := range tests {
		clusterObject, _, _ := clusters.GetProvisioningClusterByName(tt.client, s.client.RancherConfig.ClusterName, namespace)
		if clusterObject == nil {
			clusterID, err := clusters.GetClusterIDByName(s.client, s.client.RancherConfig.ClusterName)
			require.NoError(s.T(), err)

			clusterResp, err := tt.client.Management.Cluster.ByID(clusterID)
			require.NoError(s.T(), err)

			if clusterResp.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.S3BackupConfig != nil {
				tt.name = "RKE1 S3 " + tt.name
			} else {
				tt.name = "RKE1 Local " + tt.name
			}
		} else {
			clusterID, err := clusters.GetV1ProvisioningClusterByName(s.client, s.client.RancherConfig.ClusterName)
			require.NoError(s.T(), err)

			cluster, err := tt.client.Steve.SteveType(clusters.ProvisioningSteveResourceType).ByID(clusterID)
			require.NoError(s.T(), err)

			updatedCluster := new(apisV1.Cluster)
			err = v1.ConvertToK8sType(cluster, &updatedCluster)
			require.NoError(s.T(), err)

			if updatedCluster.Spec.RKEConfig.ETCD.S3 != nil {
				tt.name = "S3 " + tt.name
			} else {
				tt.name = "Local " + tt.name
			}

			if strings.Contains(updatedCluster.Spec.KubernetesVersion, "rke2") {
				tt.name = "RKE2 " + tt.name
			} else if strings.Contains(updatedCluster.Spec.KubernetesVersion, "k3s") {
				tt.name = "K3S " + tt.name
			}
		}

		s.Run(tt.name, func() {
			snapshotRestore(s.T(), s.client, s.client.RancherConfig.ClusterName, tt.etcdSnapshot)
		})
	}
}

func (s *SnapshotRestoreK8sUpgradeTestSuite) TestSnapshotRestoreK8sUpgradeDynamicInput() {
	if s.clustersConfig == nil {
		s.T().Skip()
	}

	snapshotRestore(s.T(), s.client, s.client.RancherConfig.ClusterName, s.clustersConfig)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSnapshotRestoreK8sUpgradeTestSuite(t *testing.T) {
	suite.Run(t, new(SnapshotRestoreK8sUpgradeTestSuite))
}
