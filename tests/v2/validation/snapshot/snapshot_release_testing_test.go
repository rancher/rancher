//go:build validation

package snapshot

import (
	"strings"
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/etcdsnapshot"
	"github.com/rancher/rancher/tests/v2/actions/provisioning/permutations"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/rancher/tests/v2/actions/qaseinput"
	qase "github.com/rancher/rancher/tests/v2/validation/pipeline/qase/results"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/users"
	password "github.com/rancher/shepherd/extensions/users/passwordgenerator"
	"github.com/rancher/shepherd/pkg/config"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SnapshotRestoreReleaseTestingTestSuite struct {
	suite.Suite
	session            *session.Session
	client             *rancher.Client
	standardUserClient *rancher.Client
	provisioningConfig *provisioninginput.Config
	clustersConfig     *etcdsnapshot.Config
	qaseConfig         *qaseinput.Config
}

func (s *SnapshotRestoreReleaseTestingTestSuite) TearDownSuite() {
	s.session.Cleanup()

	s.qaseConfig = new(qaseinput.Config)
	config.LoadConfig(qaseinput.ConfigurationFileKey, s.qaseConfig)

	if s.qaseConfig.LocalQaseReporting {
		err := qase.ReportTest()
		require.NoError(s.T(), err)
	}
}

func (s *SnapshotRestoreReleaseTestingTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	s.provisioningConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, s.provisioningConfig)

	s.clustersConfig = new(etcdsnapshot.Config)
	config.LoadConfig(etcdsnapshot.ConfigurationFileKey, s.clustersConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(s.T(), err)

	s.client = client

	enabled := true
	var testuser = namegen.AppendRandomString("testuser-")
	var testpassword = password.GenerateUserPassword("testpass-")
	user := &management.User{
		Username: testuser,
		Password: testpassword,
		Name:     testuser,
		Enabled:  &enabled,
	}

	newUser, err := users.CreateUserWithRole(client, user, "user")
	require.NoError(s.T(), err)

	newUser.Password = user.Password

	standardUserClient, err := client.AsUser(newUser)
	require.NoError(s.T(), err)

	s.standardUserClient = standardUserClient
}

func (s *SnapshotRestoreReleaseTestingTestSuite) TestRKE1SnapshotRestore() {
	nodeRolesDedicated := []provisioninginput.NodePools{
		provisioninginput.EtcdMultipleNodes,
		provisioninginput.ControlPlaneMultipleNodes,
		provisioninginput.WorkerMultipleNodes,
	}

	snapshotRestoreNone := &etcdsnapshot.Config{
		UpgradeKubernetesVersion: "",
		SnapshotRestore:          "none",
		RecurringRestores:        1,
	}

	snapshotRestoreK8sVersion := &etcdsnapshot.Config{
		UpgradeKubernetesVersion: "",
		SnapshotRestore:          "kubernetesVersion",
		RecurringRestores:        1,
	}

	snapshotRestoreAll := &etcdsnapshot.Config{
		UpgradeKubernetesVersion:     "",
		SnapshotRestore:              "all",
		ControlPlaneUnavailableValue: "3",
		WorkerUnavailableValue:       "15%",
		RecurringRestores:            1,
	}

	tests := []struct {
		name         string
		etcdSnapshot *etcdsnapshot.Config
		client       *rancher.Client
	}{
		{"RKE1 Local Restore etcd only", snapshotRestoreNone, s.standardUserClient},
		{"RKE1 Local Restore K8s version and etcd", snapshotRestoreK8sVersion, s.standardUserClient},
		{"RKE1 Local Restore cluster config, K8s version and etcd", snapshotRestoreAll, s.standardUserClient},
	}

	var clusterObject *management.Cluster
	provisioningConfig := *s.provisioningConfig
	provisioningConfig.NodePools = nodeRolesDedicated

	for _, tt := range tests {
		subSession := s.session.NewSession()
		defer subSession.Cleanup()

		client, err := tt.client.WithSession(subSession)
		require.NoError(s.T(), err)

		if clusterObject == nil {
			_, clusterObject = permutations.RunTestPermutations(&s.Suite, "Provision RKE1", client, &provisioningConfig, permutations.RKE1ProvisionCluster, nil, nil)
		}

		adminClient, err := rancher.NewClient(tt.client.RancherConfig.AdminToken, client.Session)
		require.NoError(s.T(), err)

		s.Run(tt.name, func() {
			snapshotRestore(s.T(), adminClient, clusterObject.Name, tt.etcdSnapshot, containerImage)
		})
	}
}

func (s *SnapshotRestoreReleaseTestingTestSuite) TestSnapshotRestore() {
	nodeRolesDedicated := []provisioninginput.MachinePools{
		provisioninginput.EtcdMultipleMachines,
		provisioninginput.ControlPlaneMultipleMachines,
		provisioninginput.WorkerMultipleMachines,
	}

	snapshotRestoreNone := &etcdsnapshot.Config{
		UpgradeKubernetesVersion: "",
		SnapshotRestore:          "none",
		RecurringRestores:        1,
	}

	snapshotRestoreK8sVersion := &etcdsnapshot.Config{
		UpgradeKubernetesVersion: "",
		SnapshotRestore:          "kubernetesVersion",
		RecurringRestores:        1,
	}

	snapshotRestoreAll := &etcdsnapshot.Config{
		UpgradeKubernetesVersion:     "",
		SnapshotRestore:              "all",
		ControlPlaneConcurrencyValue: "15%",
		WorkerConcurrencyValue:       "20%",
		RecurringRestores:            1,
	}

	tests := []struct {
		name         string
		etcdSnapshot *etcdsnapshot.Config
		client       *rancher.Client
	}{
		{"RKE2 Local Restore etcd only", snapshotRestoreNone, s.standardUserClient},
		{"RKE2 Local Restore K8s version and etcd", snapshotRestoreK8sVersion, s.standardUserClient},
		{"RKE2 Local Restore cluster config, K8s version and etcd", snapshotRestoreAll, s.standardUserClient},
		{"K3S Local Restore etcd only", snapshotRestoreNone, s.standardUserClient},
		{"K3S Local Restore K8s version and etcd", snapshotRestoreK8sVersion, s.standardUserClient},
		{"K3S Local Restore cluster config, K8s version and etcd", snapshotRestoreAll, s.standardUserClient},
	}

	var clusterObject *v1.SteveAPIObject
	provisioningConfig := *s.provisioningConfig
	provisioningConfig.MachinePools = nodeRolesDedicated

	for _, tt := range tests {
		subSession := s.session.NewSession()
		defer subSession.Cleanup()

		client, err := tt.client.WithSession(subSession)
		require.NoError(s.T(), err)

		if clusterObject == nil {
			if strings.Contains(tt.name, "RKE2") {
				clusterObject, _ = permutations.RunTestPermutations(&s.Suite, "Provision RKE2", client, &provisioningConfig, permutations.RKE2ProvisionCluster, nil, nil)
			} else {
				clusterObject, _ = permutations.RunTestPermutations(&s.Suite, "Provision K3S", client, &provisioningConfig, permutations.K3SProvisionCluster, nil, nil)
			}
		}

		adminClient, err := rancher.NewClient(tt.client.RancherConfig.AdminToken, tt.client.Session)
		require.NoError(s.T(), err)

		s.Run(tt.name, func() {
			snapshotRestore(s.T(), adminClient, clusterObject.Name, tt.etcdSnapshot, containerImage)
		})
	}
}

func (s *SnapshotRestoreReleaseTestingTestSuite) TestRKE1SnapshotReplaceNodes() {
	nodeRolesDedicated := []provisioninginput.NodePools{
		provisioninginput.EtcdMultipleNodes,
		provisioninginput.ControlPlaneMultipleNodes,
		provisioninginput.WorkerMultipleNodes,
	}

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
		{"RKE1 Replace control plane nodes", controlPlaneSnapshotRestore, s.standardUserClient},
		{"RKE1 Replace etcd nodes", etcdSnapshotRestore, s.standardUserClient},
		{"RKE1 Replace worker nodes", workerSnapshotRestore, s.standardUserClient},
	}

	var clusterObject *management.Cluster
	provisioningConfig := *s.provisioningConfig
	provisioningConfig.NodePools = nodeRolesDedicated

	for _, tt := range tests {
		subSession := s.session.NewSession()
		defer subSession.Cleanup()

		client, err := tt.client.WithSession(subSession)
		require.NoError(s.T(), err)

		if clusterObject == nil {
			_, clusterObject = permutations.RunTestPermutations(&s.Suite, "Provision RKE1", client, &provisioningConfig, permutations.RKE1ProvisionCluster, nil, nil)
		}

		adminClient, err := rancher.NewClient(tt.client.RancherConfig.AdminToken, client.Session)
		require.NoError(s.T(), err)

		s.Run(tt.name, func() {
			snapshotRestore(s.T(), adminClient, clusterObject.Name, tt.etcdSnapshot, containerImage)
		})
	}
}

func (s *SnapshotRestoreReleaseTestingTestSuite) TestSnapshotReplaceNodes() {
	nodeRolesDedicated := []provisioninginput.MachinePools{
		provisioninginput.EtcdMultipleMachines,
		provisioninginput.ControlPlaneMultipleMachines,
		provisioninginput.WorkerMultipleMachines,
	}

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
		{"RKE2 Replace control plane nodes", controlPlaneSnapshotRestore, s.standardUserClient},
		{"RKE2 Replace etcd nodes", etcdSnapshotRestore, s.standardUserClient},
		{"RKE2 Replace worker nodes", workerSnapshotRestore, s.standardUserClient},
		{"K3S Replace control plane nodes", controlPlaneSnapshotRestore, s.standardUserClient},
		{"K3S Replace etcd nodes", etcdSnapshotRestore, s.standardUserClient},
		{"K3S Replace worker nodes", workerSnapshotRestore, s.standardUserClient},
	}

	var clusterObject *v1.SteveAPIObject
	provisioningConfig := *s.provisioningConfig
	provisioningConfig.MachinePools = nodeRolesDedicated

	for _, tt := range tests {
		subSession := s.session.NewSession()
		defer subSession.Cleanup()

		client, err := tt.client.WithSession(subSession)
		require.NoError(s.T(), err)

		if clusterObject == nil {
			if strings.Contains(tt.name, "RKE2") {
				clusterObject, _ = permutations.RunTestPermutations(&s.Suite, "Provision RKE2", client, &provisioningConfig, permutations.RKE2ProvisionCluster, nil, nil)
			} else {
				clusterObject, _ = permutations.RunTestPermutations(&s.Suite, "Provision K3S", client, &provisioningConfig, permutations.K3SProvisionCluster, nil, nil)
			}
		}

		adminClient, err := rancher.NewClient(tt.client.RancherConfig.AdminToken, tt.client.Session)
		require.NoError(s.T(), err)

		s.Run(tt.name, func() {
			snapshotRestore(s.T(), adminClient, clusterObject.Name, tt.etcdSnapshot, containerImage)
		})
	}
}

func (s *SnapshotRestoreReleaseTestingTestSuite) TestRKE1SnapshotRecurringRestores() {
	nodeRolesDedicated := []provisioninginput.NodePools{
		provisioninginput.EtcdMultipleNodes,
		provisioninginput.ControlPlaneMultipleNodes,
		provisioninginput.WorkerMultipleNodes,
	}

	snapshotRestoreNone := &etcdsnapshot.Config{
		UpgradeKubernetesVersion: "",
		SnapshotRestore:          "none",
		RecurringRestores:        5,
	}

	tests := []struct {
		name         string
		etcdSnapshot *etcdsnapshot.Config
		client       *rancher.Client
	}{
		{"RKE1 Restore snapshot 5 times", snapshotRestoreNone, s.standardUserClient},
	}

	var clusterObject *management.Cluster
	provisioningConfig := *s.provisioningConfig
	provisioningConfig.NodePools = nodeRolesDedicated

	for _, tt := range tests {
		subSession := s.session.NewSession()
		defer subSession.Cleanup()

		client, err := tt.client.WithSession(subSession)
		require.NoError(s.T(), err)

		if clusterObject == nil {
			_, clusterObject = permutations.RunTestPermutations(&s.Suite, "Provision RKE1", client, &provisioningConfig, permutations.RKE1ProvisionCluster, nil, nil)
		}

		adminClient, err := rancher.NewClient(tt.client.RancherConfig.AdminToken, client.Session)
		require.NoError(s.T(), err)

		s.Run(tt.name, func() {
			snapshotRestore(s.T(), adminClient, clusterObject.Name, tt.etcdSnapshot, containerImage)
		})
	}
}

func (s *SnapshotRestoreReleaseTestingTestSuite) TestSnapshotRecurringRestores() {
	nodeRolesDedicated := []provisioninginput.MachinePools{
		provisioninginput.EtcdMultipleMachines,
		provisioninginput.ControlPlaneMultipleMachines,
		provisioninginput.WorkerMultipleMachines,
	}

	snapshotRestoreNone := &etcdsnapshot.Config{
		UpgradeKubernetesVersion: "",
		SnapshotRestore:          "none",
		RecurringRestores:        5,
	}

	tests := []struct {
		name         string
		etcdSnapshot *etcdsnapshot.Config
		client       *rancher.Client
	}{
		{"RKE2 Restore snapshot 5 times", snapshotRestoreNone, s.standardUserClient},
		{"K3S Restore snapshot 5 times", snapshotRestoreNone, s.standardUserClient},
	}

	var clusterObject *v1.SteveAPIObject
	provisioningConfig := *s.provisioningConfig
	provisioningConfig.MachinePools = nodeRolesDedicated

	for _, tt := range tests {
		subSession := s.session.NewSession()
		defer subSession.Cleanup()

		client, err := tt.client.WithSession(subSession)
		require.NoError(s.T(), err)

		if clusterObject == nil {
			if strings.Contains(tt.name, "RKE2") {
				clusterObject, _ = permutations.RunTestPermutations(&s.Suite, "Provision RKE2", client, &provisioningConfig, permutations.RKE2ProvisionCluster, nil, nil)
			} else {
				clusterObject, _ = permutations.RunTestPermutations(&s.Suite, "Provision K3S", client, &provisioningConfig, permutations.K3SProvisionCluster, nil, nil)
			}
		}

		adminClient, err := rancher.NewClient(tt.client.RancherConfig.AdminToken, client.Session)
		require.NoError(s.T(), err)

		s.Run(tt.name, func() {
			snapshotRestore(s.T(), adminClient, clusterObject.Name, tt.etcdSnapshot, containerImage)
		})
	}
}

func (s *SnapshotRestoreReleaseTestingTestSuite) TestSnapshotRestoreWindows() {
	nodeRolesDedicated := []provisioninginput.MachinePools{
		provisioninginput.EtcdMultipleMachines,
		provisioninginput.ControlPlaneMultipleMachines,
		provisioninginput.WorkerMultipleMachines,
		provisioninginput.WindowsMultipleMachines,
	}

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
		{"Restore Windows cluster config, Kubernetes version and etcd", snapshotRestoreAll, s.standardUserClient},
	}

	provisioningConfig := *s.provisioningConfig
	provisioningConfig.MachinePools = nodeRolesDedicated

	for _, tt := range tests {
		subSession := s.session.NewSession()
		defer subSession.Cleanup()

		client, err := tt.client.WithSession(subSession)
		require.NoError(s.T(), err)

		clusterObject, _ := permutations.RunTestPermutations(&s.Suite, "Provision RKE2 Windows", client, &provisioningConfig, permutations.RKE2CustomCluster, nil, nil)

		adminClient, err := rancher.NewClient(tt.client.RancherConfig.AdminToken, client.Session)
		require.NoError(s.T(), err)

		s.Run(tt.name, func() {
			snapshotRestore(s.T(), adminClient, clusterObject.Name, tt.etcdSnapshot, windowsContainerImage)
		})
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSnapshotRestoreReleaseTestingTestSuite(t *testing.T) {
	suite.Run(t, new(SnapshotRestoreReleaseTestingTestSuite))
}
