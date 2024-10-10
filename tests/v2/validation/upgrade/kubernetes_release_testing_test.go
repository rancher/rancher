//go:build validation

package upgrade

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/clusters"
	"github.com/rancher/rancher/tests/v2/actions/provisioning"
	"github.com/rancher/rancher/tests/v2/actions/provisioning/permutations"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/rancher/tests/v2/actions/qaseinput"
	"github.com/rancher/rancher/tests/v2/actions/upgradeinput"
	qase "github.com/rancher/rancher/tests/v2/validation/pipeline/qase/results"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	extensionscluster "github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/users"
	password "github.com/rancher/shepherd/extensions/users/passwordgenerator"
	"github.com/rancher/shepherd/pkg/config"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type UpgradeKubernetesReleaseTestingTestSuite struct {
	suite.Suite
	session            *session.Session
	client             *rancher.Client
	standardUserClient *rancher.Client
	clusters           []upgradeinput.Cluster
	provisioningConfig *provisioninginput.Config
	qaseConfig         *qaseinput.Config
}

func (u *UpgradeKubernetesReleaseTestingTestSuite) TearDownSuite() {
	u.session.Cleanup()

	u.qaseConfig = new(qaseinput.Config)
	config.LoadConfig(qaseinput.ConfigurationFileKey, u.qaseConfig)

	if u.qaseConfig.LocalQaseReporting {
		err := qase.ReportTest()
		require.NoError(u.T(), err)
	}
}

func (u *UpgradeKubernetesReleaseTestingTestSuite) SetupSuite() {
	testSession := session.NewSession()
	u.session = testSession

	u.provisioningConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, u.provisioningConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(u.T(), err)

	u.client = client

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
	require.NoError(u.T(), err)

	newUser.Password = user.Password

	standardUserClient, err := client.AsUser(newUser)
	require.NoError(u.T(), err)

	u.standardUserClient = standardUserClient

	clusters, err := upgradeinput.LoadUpgradeKubernetesConfig(client)
	require.NoError(u.T(), err)

	u.clusters = clusters
}

func (u *UpgradeKubernetesReleaseTestingTestSuite) TestUpgradeRKE1Kubernetes() {
	nodeRolesDedicated := []provisioninginput.NodePools{
		provisioninginput.EtcdMultipleNodes,
		provisioninginput.ControlPlaneMultipleNodes,
		provisioninginput.WorkerMultipleNodes,
	}

	tests := []struct {
		name   string
		client *rancher.Client
	}{
		{"Upgrading ", u.standardUserClient},
	}

	var clusterObject *management.Cluster
	provisioningConfig := *u.provisioningConfig
	provisioningConfig.NodePools = nodeRolesDedicated

	for _, tt := range tests {
		subSession := u.session.NewSession()
		defer subSession.Cleanup()

		client, err := tt.client.WithSession(subSession)
		require.NoError(u.T(), err)

		for _, cluster := range u.clusters {
			if clusterObject == nil {
				_, clusterObject = permutations.RunTestPermutations(&u.Suite, "Provision RKE1", client, &provisioningConfig, permutations.RKE1ProvisionCluster, nil, nil)
			}

			testConfig := clusters.ConvertConfigToClusterConfig(&cluster.ProvisioningInput)

			testConfig.KubernetesVersion = cluster.ProvisioningInput.RKE1KubernetesVersions[0]
			tt.name += "RKE1 cluster from " + provisioningConfig.RKE1KubernetesVersions[0] + " to " + testConfig.KubernetesVersion

			u.Run(tt.name, func() {
				adminClient, err := rancher.NewClient(tt.client.RancherConfig.AdminToken, client.Session)
				require.NoError(u.T(), err)

				createPreUpgradeWorkloads(u.T(), adminClient, clusterObject.Name, cluster.FeaturesToTest, nil, containerImage)

				upgradedCluster, err := upgradeRKE1Cluster(u.T(), client, cluster, testConfig)
				require.NoError(u.T(), err)

				clusterResp, err := extensionscluster.GetClusterIDByName(adminClient, upgradedCluster.Name)
				require.NoError(u.T(), err)

				upgradedRKE1Cluster, err := adminClient.Management.Cluster.ByID(clusterResp)
				require.NoError(u.T(), err)

				provisioning.VerifyRKE1Cluster(u.T(), adminClient, testConfig, upgradedRKE1Cluster)
				createPostUpgradeWorkloads(u.T(), adminClient, clusterObject.Name, cluster.FeaturesToTest)
			})
		}
	}
}

func (u *UpgradeKubernetesReleaseTestingTestSuite) TestUpgradeKubernetes() {
	nodeRolesDedicated := []provisioninginput.MachinePools{
		provisioninginput.EtcdMultipleMachines,
		provisioninginput.ControlPlaneMultipleMachines,
		provisioninginput.WorkerMultipleMachines,
	}

	nodeRolesWindows := []provisioninginput.MachinePools{
		provisioninginput.EtcdMultipleMachines,
		provisioninginput.ControlPlaneMultipleMachines,
		provisioninginput.WorkerMultipleMachines,
		provisioninginput.WindowsMultipleMachines,
	}

	tests := []struct {
		name         string
		client       *rancher.Client
		isRKE2       bool
		nodeSelector map[string]string
	}{
		{"Upgrading ", u.standardUserClient, true, nil},
		{"Upgrading ", u.standardUserClient, false, nil},
		{"Upgrading ", u.standardUserClient, true, map[string]string{"kubernetes.io/os": "windows"}},
	}

	var testConfig *clusters.ClusterConfig
	var image string

	provisioningConfig := *u.provisioningConfig
	provisioningConfig.MachinePools = nodeRolesDedicated

	for _, tt := range tests {
		subSession := u.session.NewSession()
		defer subSession.Cleanup()

		client, err := tt.client.WithSession(subSession)
		require.NoError(u.T(), err)

		for _, cluster := range u.clusters {
			var clusterObject *v1.SteveAPIObject

			if tt.isRKE2 && tt.nodeSelector == nil {
				clusterObject, _ = permutations.RunTestPermutations(&u.Suite, "Provision RKE2", client, &provisioningConfig, permutations.RKE2ProvisionCluster, nil, nil)
			} else if tt.isRKE2 && tt.nodeSelector != nil {
				provisioningConfig.MachinePools = nodeRolesWindows
				clusterObject, _ = permutations.RunTestPermutations(&u.Suite, "Provision RKE2 Windows", client, &provisioningConfig, permutations.RKE2CustomCluster, nil, nil)
			} else if !tt.isRKE2 && tt.nodeSelector == nil {
				clusterObject, _ = permutations.RunTestPermutations(&u.Suite, "Provision K3S", client, &provisioningConfig, permutations.K3SProvisionCluster, nil, nil)
			}

			testConfig = clusters.ConvertConfigToClusterConfig(&cluster.ProvisioningInput)

			if tt.isRKE2 {
				testConfig.KubernetesVersion = cluster.ProvisioningInput.RKE2KubernetesVersions[0]
				tt.name += "RKE2 cluster from " + provisioningConfig.RKE2KubernetesVersions[0] + " to " + testConfig.KubernetesVersion
			} else if tt.isRKE2 && tt.nodeSelector != nil {
				testConfig.KubernetesVersion = cluster.ProvisioningInput.RKE2KubernetesVersions[0]
				tt.name += "RKE2 Windows cluster from " + provisioningConfig.RKE2KubernetesVersions[0] + " to " + testConfig.KubernetesVersion
			} else if !tt.isRKE2 && tt.nodeSelector == nil {
				testConfig.KubernetesVersion = cluster.ProvisioningInput.K3SKubernetesVersions[0]
				tt.name += "K3S cluster from " + provisioningConfig.K3SKubernetesVersions[0] + " to " + testConfig.KubernetesVersion
			}

			if tt.nodeSelector == nil {
				image = containerImage
			} else {
				image = windowsContainerImage
			}

			u.Run(tt.name, func() {
				adminClient, err := rancher.NewClient(tt.client.RancherConfig.AdminToken, client.Session)
				require.NoError(u.T(), err)

				createPreUpgradeWorkloads(u.T(), adminClient, clusterObject.Name, cluster.FeaturesToTest, tt.nodeSelector, image)
				upgradedCluster, err := upgradeRKE2K3SCluster(u.T(), adminClient, clusterObject.Name, testConfig)
				require.NoError(u.T(), err)

				provisioning.VerifyCluster(u.T(), adminClient, testConfig, upgradedCluster)
				createPostUpgradeWorkloads(u.T(), adminClient, clusterObject.Name, cluster.FeaturesToTest)
			})
		}
	}
}

func TestUpgradeKubernetesReleaseTestingTestSuite(t *testing.T) {
	suite.Run(t, new(UpgradeKubernetesReleaseTestingTestSuite))
}
