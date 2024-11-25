//go:build (validation || extended || infra.any || cluster.any) && !sanity && !stress

package snapshot

import (
	"strings"
	"testing"

	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/etcdsnapshot"
	"github.com/rancher/shepherd/clients/rancher"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"

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
	snapshotRestoreAll := &etcdsnapshot.Config{
		UpgradeKubernetesVersion:     "",
		SnapshotRestore:              "all",
		ControlPlaneConcurrencyValue: "15%",
		ControlPlaneUnavailableValue: "3",
		WorkerConcurrencyValue:       "20%",
		WorkerUnavailableValue:       "15%",
		RecurringRestores:            1,
	}

	tests := []struct {
		name         string
		etcdSnapshot *etcdsnapshot.Config
		client       *rancher.Client
	}{
		{"Restore cluster config, Kubernetes version and etcd", snapshotRestoreAll, s.client},
	}

	for _, tt := range tests {
		clusterID, err := clusters.GetV1ProvisioningClusterByName(s.client, s.client.RancherConfig.ClusterName)
		require.NoError(s.T(), err)

		cluster, err := s.client.Steve.SteveType(clusters.ProvisioningSteveResourceType).ByID(clusterID)
		require.NoError(s.T(), err)

		spec := &apisV1.ClusterSpec{}
		err = steveV1.ConvertToK8sType(cluster.Spec, spec)
		require.NoError(s.T(), err)

		if strings.Contains(spec.KubernetesVersion, "-rancher") || len(spec.KubernetesVersion) == 0 {
			tt.name = "RKE1 " + tt.name
		} else {
			if strings.Contains(spec.KubernetesVersion, "k3s") {
				tt.name = "K3S " + tt.name
			} else {
				tt.name = "RKE2 " + tt.name
			}
		}

		s.Run(tt.name, func() {
			err := etcdsnapshot.CreateAndValidateSnapshotRestore(s.client, s.client.RancherConfig.ClusterName, tt.etcdSnapshot, containerImage)
			require.NoError(s.T(), err)
		})
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSnapshotRestoreUpgradeStrategyTestSuite(t *testing.T) {
	suite.Run(t, new(SnapshotRestoreUpgradeStrategyTestSuite))
}
