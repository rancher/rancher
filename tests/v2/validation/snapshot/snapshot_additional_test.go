//go:build validation

package snapshot

import (
	"strings"
	"testing"

	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/etcdsnapshot"
	"github.com/rancher/shepherd/clients/rancher"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"

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

func (s *SnapshotAdditionalTestsTestSuite) TestSnapshotReplaceNodes() {
	controlPlaneSnapshotRestore := &etcdsnapshot.Config{
		UpgradeKubernetesVersion: "",
		SnapshotRestore:          "none",
		RecurringRestores:        1,
		ReplaceRoles: &etcdsnapshot.ReplaceRoles{
			ControlPlane: true,
		},
	}

	etcdSnapshotRestore := &etcdsnapshot.Config{
		UpgradeKubernetesVersion: "",
		SnapshotRestore:          "none",
		RecurringRestores:        1,
		ReplaceRoles: &etcdsnapshot.ReplaceRoles{
			Etcd: true,
		},
	}

	workerSnapshotRestore := &etcdsnapshot.Config{
		UpgradeKubernetesVersion: "",
		SnapshotRestore:          "none",
		RecurringRestores:        1,
		ReplaceRoles: &etcdsnapshot.ReplaceRoles{
			Worker: true,
		},
	}

	tests := []struct {
		name         string
		etcdSnapshot *etcdsnapshot.Config
		client       *rancher.Client
	}{
		{"Replace control plane nodes", controlPlaneSnapshotRestore, s.client},
		{"Replace etcd nodes", etcdSnapshotRestore, s.client},
		{"Replace worker nodes", workerSnapshotRestore, s.client},
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

		if strings.Contains(tt.name, "S3") {
			s.Run(tt.name, func() {
				etcdsnapshot.SnapshotRestore(s.T(), s.client, s.client.RancherConfig.ClusterName, tt.etcdSnapshot, containerImage)
			})
		} else {
			s.T().Skip("Skipping test; only S3 enabled clusters are enabled for this test")
		}
	}
}

func (s *SnapshotAdditionalTestsTestSuite) TestSnapshotRecurringRestores() {
	snapshotRestoreFiveTimes := &etcdsnapshot.Config{
		UpgradeKubernetesVersion: "",
		SnapshotRestore:          "none",
		RecurringRestores:        5,
	}

	tests := []struct {
		name         string
		etcdSnapshot *etcdsnapshot.Config
		client       *rancher.Client
	}{
		{"Restore snapshot 5 times", snapshotRestoreFiveTimes, s.client},
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
			etcdsnapshot.SnapshotRestore(s.T(), s.client, s.client.RancherConfig.ClusterName, tt.etcdSnapshot, containerImage)
		})
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSnapshotAdditionalTestsTestSuite(t *testing.T) {
	suite.Run(t, new(SnapshotAdditionalTestsTestSuite))
}
