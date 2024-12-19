//go:build (validation || extended || infra.any || cluster.any) && !sanity && !stress

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

const (
	namespace      = "fleet-default"
	containerImage = "nginx"
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
			err := etcdsnapshot.CreateAndValidateSnapshotRestore(s.client, s.client.RancherConfig.ClusterName, tt.etcdSnapshot, containerImage)
			require.NoError(s.T(), err)
		})
	}
}

func (s *SnapshotRestoreTestSuite) TestSnapshotRestoreETCDOnlyDynamicInput() {
	if s.clustersConfig == nil {
		s.T().Skip()
	}

	err := etcdsnapshot.CreateAndValidateSnapshotRestore(s.client, s.client.RancherConfig.ClusterName, s.clustersConfig, containerImage)
	require.NoError(s.T(), err)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSnapshotRestoreETCDOnlyTestSuite(t *testing.T) {
	suite.Run(t, new(SnapshotRestoreTestSuite))
}
