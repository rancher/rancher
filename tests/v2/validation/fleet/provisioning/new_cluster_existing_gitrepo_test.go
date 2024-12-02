//go:build validation

package provisioning

import (
	"testing"

	"github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/clusters"
	"github.com/rancher/rancher/tests/v2/actions/fleet"
	"github.com/rancher/rancher/tests/v2/actions/provisioning"
	"github.com/rancher/rancher/tests/v2/actions/provisioning/permutations"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/rancher/tests/v2/actions/reports"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters/kubernetesversions"
	extensionsfleet "github.com/rancher/shepherd/extensions/fleet"
	"github.com/rancher/shepherd/extensions/users"
	password "github.com/rancher/shepherd/extensions/users/passwordgenerator"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/environmentflag"
	"github.com/rancher/shepherd/pkg/namegenerator"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type FleetWithProvisioningTestSuite struct {
	suite.Suite
	client             *rancher.Client
	standardUserClient *rancher.Client
	session            *session.Session
	provisioningConfig *provisioninginput.Config
	fleetGitRepo       *v1alpha1.GitRepo
}

func (f *FleetWithProvisioningTestSuite) TearDownSuite() {
	f.session.Cleanup()
}

func (f *FleetWithProvisioningTestSuite) SetupSuite() {
	f.session = session.NewSession()

	client, err := rancher.NewClient("", f.session)
	require.NoError(f.T(), err)

	f.client = client

	f.fleetGitRepo = &v1alpha1.GitRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fleet.FleetMetaName + namegenerator.RandStringLower(5),
			Namespace: fleet.Namespace,
		},
		Spec: v1alpha1.GitRepoSpec{
			Repo:            fleet.ExampleRepo,
			Branch:          fleet.BranchName,
			Paths:           []string{"hardened"},
			CorrectDrift:    &v1alpha1.CorrectDrift{},
			ImageScanCommit: v1alpha1.CommitSpec{AuthorName: "", AuthorEmail: ""},
			Targets: []v1alpha1.GitTarget{
				{
					ClusterSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      fleet.MatchKey,
								Operator: fleet.MatchOperator,
								Values: []string{
									fleet.HarvesterName,
								},
							},
						},
					},
				},
			},
		},
	}

	f.client, err = f.client.ReLogin()
	require.NoError(f.T(), err)

	userConfig := new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, userConfig)

	f.provisioningConfig = userConfig

	if f.provisioningConfig.RKE2KubernetesVersions == nil {
		rke2Versions, err := kubernetesversions.ListRKE2AllVersions(f.client)
		require.NoError(f.T(), err)

		f.provisioningConfig.RKE2KubernetesVersions = []string{rke2Versions[len(rke2Versions)-1]}
	}

	if f.provisioningConfig.CNIs == nil {
		f.provisioningConfig.CNIs = []string{fleet.CniCalico}
	}

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
	require.NoError(f.T(), err)

	newUser.Password = user.Password

	standardUserClient, err := client.AsUser(newUser)
	require.NoError(f.T(), err)

	f.standardUserClient = standardUserClient
}

func (f *FleetWithProvisioningTestSuite) TestHardenedAfterAddedGitRepo() {
	fleetVersion, err := fleet.GetDeploymentVersion(f.client, fleet.FleetControllerName, fleet.LocalName)
	require.NoError(f.T(), err)

	nodeRolesDedicated := []provisioninginput.MachinePools{
		provisioninginput.EtcdMachinePool,
		provisioninginput.ControlPlaneMachinePool,
		provisioninginput.WorkerMachinePool,
	}

	tests := []struct {
		name         string
		client       *rancher.Client
		machinePools []provisioninginput.MachinePools
		runFlag      bool
	}{
		{fleet.FleetName + " " + fleetVersion, f.standardUserClient, nodeRolesDedicated, f.client.Flags.GetValue(environmentflag.Long)},
	}

	for _, tt := range tests {
		if !tt.runFlag {
			f.T().Logf("SKIPPED")
			continue
		}

		testSession := session.NewSession()
		defer testSession.Cleanup()

		adminClient, err := f.client.WithSession(testSession)
		require.NoError(f.T(), err)

		provisioningConfig := *f.provisioningConfig

		provisioningConfig.Hardened = true
		provisioningConfig.MachinePools = tt.machinePools
		provisioningConfig.PSACT = string(provisioninginput.RancherRestricted)

		f.Run(tt.name, func() {
			logrus.Info("Deploying public fleet gitRepo")
			gitRepoObject, err := extensionsfleet.CreateFleetGitRepo(adminClient, f.fleetGitRepo)
			require.NoError(f.T(), err)

			logrus.Info("Deploying Custom Cluster")
			testClusterConfig := clusters.ConvertConfigToClusterConfig(&provisioningConfig)

			_, _, customProvider, _ := permutations.GetClusterProvider(permutations.RKE2CustomCluster, f.provisioningConfig.NodeProviders[0], &provisioningConfig)

			testClusterConfig.KubernetesVersion = f.provisioningConfig.RKE2KubernetesVersions[0]
			testClusterConfig.CNI = f.provisioningConfig.CNIs[0]

			clusterObject, err := provisioning.CreateProvisioningCustomCluster(tt.client, customProvider, testClusterConfig)
			require.NoError(f.T(), err)

			reports.TimeoutClusterReport(clusterObject, err)
			require.NoError(f.T(), err)

			provisioning.VerifyCluster(f.T(), tt.client, testClusterConfig, clusterObject)

			status := &provv1.ClusterStatus{}
			err = steveV1.ConvertToK8sType(clusterObject.Status, status)
			require.NoError(f.T(), err)

			err = fleet.VerifyGitRepo(adminClient, gitRepoObject.ID, status.ClusterName, clusterObject.ID)
			require.NoError(f.T(), err)
		})
	}

}

func (f *FleetWithProvisioningTestSuite) TestWindowsAfterAddedGitRepo() {
	fleetVersion, err := fleet.GetDeploymentVersion(f.client, fleet.FleetControllerName, fleet.LocalName)
	require.NoError(f.T(), err)

	nodeRolesDedicatedWindows := []provisioninginput.MachinePools{
		provisioninginput.EtcdMachinePool,
		provisioninginput.ControlPlaneMachinePool,
		provisioninginput.WorkerMachinePool,
		provisioninginput.WindowsMachinePool,
	}

	tests := []struct {
		name         string
		client       *rancher.Client
		machinePools []provisioninginput.MachinePools
		runFlag      bool
	}{
		{fleet.FleetName + " " + fleetVersion, f.standardUserClient, nodeRolesDedicatedWindows, f.client.Flags.GetValue(environmentflag.Long)},
	}

	for _, tt := range tests {
		if !tt.runFlag {
			f.T().Logf("SKIPPED")
			continue
		}

		provisioningConfig := *f.provisioningConfig
		provisioningConfig.MachinePools = tt.machinePools

		f.fleetGitRepo.Name += "windows"
		f.fleetGitRepo.Spec.Paths = []string{fleet.GitRepoPathWindows}

		f.Run(tt.name, func() {
			testSession := session.NewSession()
			defer testSession.Cleanup()

			adminClient, err := f.client.WithSession(testSession)
			require.NoError(f.T(), err)

			logrus.Info("Deploying public fleet gitRepo")
			gitRepoObject, err := extensionsfleet.CreateFleetGitRepo(adminClient, f.fleetGitRepo)
			require.NoError(f.T(), err)

			logrus.Info("Deploying Custom Cluster")
			testClusterConfig := clusters.ConvertConfigToClusterConfig(&provisioningConfig)

			_, _, customProvider, _ := permutations.GetClusterProvider(permutations.RKE2CustomCluster, f.provisioningConfig.NodeProviders[0], &provisioningConfig)

			testClusterConfig.KubernetesVersion = f.provisioningConfig.RKE2KubernetesVersions[0]
			testClusterConfig.CNI = f.provisioningConfig.CNIs[0]

			clusterObject, err := provisioning.CreateProvisioningCustomCluster(tt.client, customProvider, testClusterConfig)
			require.NoError(f.T(), err)

			reports.TimeoutClusterReport(clusterObject, err)
			require.NoError(f.T(), err)

			provisioning.VerifyCluster(f.T(), tt.client, testClusterConfig, clusterObject)

			status := &provv1.ClusterStatus{}
			err = steveV1.ConvertToK8sType(clusterObject.Status, status)
			require.NoError(f.T(), err)

			err = fleet.VerifyGitRepo(adminClient, gitRepoObject.ID, status.ClusterName, clusterObject.ID)
			require.NoError(f.T(), err)
		})

	}

}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestFleetWithProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(FleetWithProvisioningTestSuite))
}
