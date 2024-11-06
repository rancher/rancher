//go:build validation

package airgap

import (
	"fmt"
	"strings"
	"testing"

	"github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/clusters"
	"github.com/rancher/rancher/tests/v2/actions/fleet"
	provisioning "github.com/rancher/rancher/tests/v2/actions/provisioning"
	"github.com/rancher/rancher/tests/v2/actions/provisioning/permutations"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/rancher/tests/v2/actions/reports"
	"github.com/rancher/shepherd/clients/corral"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters/kubernetesversions"
	extensionsfleet "github.com/rancher/shepherd/extensions/fleet"
	"github.com/rancher/shepherd/extensions/users"
	password "github.com/rancher/shepherd/extensions/users/passwordgenerator"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/namegenerator"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AirGapRKE2CustomClusterTestSuite struct {
	suite.Suite
	client         *rancher.Client
	standardClient *rancher.Client
	session        *session.Session
	corralPackage  *corral.Packages
	clustersConfig *provisioninginput.Config
	fleetGitRepo   *v1alpha1.GitRepo
}

func (a *AirGapRKE2CustomClusterTestSuite) TearDownSuite() {
	a.session.Cleanup()
}

func (a *AirGapRKE2CustomClusterTestSuite) SetupSuite() {
	testSession := session.NewSession()
	a.session = testSession

	a.clustersConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, a.clustersConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(a.T(), err)

	a.client = client

	var testuser = namegen.AppendRandomString("testuser-")
	var testpassword = password.GenerateUserPassword("testpass-")
	enabled := true

	user := &management.User{
		Username: testuser,
		Password: testpassword,
		Name:     testuser,
		Enabled:  &enabled,
	}

	newUser, err := users.CreateUserWithRole(client, user, "user")
	require.NoError(a.T(), err)

	standardUserClient, err := client.AsUser(newUser)
	require.NoError(a.T(), err)

	a.standardClient = standardUserClient

	corralConfig := corral.Configurations()
	err = corral.SetupCorralConfig(corralConfig.CorralConfigVars, corralConfig.CorralConfigUser, corralConfig.CorralSSHPath)
	require.NoError(a.T(), err)

	a.corralPackage = corral.PackagesConfig()

	bastionIP := corralConfig.CorralConfigVars[corralBastionIP]
	bastionUser := corralConfig.CorralConfigVars[corralSSHUser]

	// format the privateKey string for use with ssh package.
	privateKey := corralConfig.CorralConfigVars[corralPrivateKey]
	privateKey = strings.Replace(privateKey, "\\n", "\n", -1)
	privateKey = strings.Replace(privateKey, "\"", "", -1)

	err = corral.UpdateCorralConfig(corralBastionIP, bastionIP)
	require.NoError(a.T(), err)

	internalIP, err := setupAirgapFleetResources(bastionUser, bastionIP, privateKey)
	require.NoError(a.T(), err, "failed to setup local git repo on bastion")

	fleetSecretName, err := createFleetSSHSecret(client, privateKey)
	require.NoError(a.T(), err, "failed to create SSH Secrets for fleet")

	a.fleetGitRepo = &v1alpha1.GitRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fleet.FleetMetaName + namegenerator.RandStringLower(5),
			Namespace: fleet.Namespace,
		},
		Spec: v1alpha1.GitRepoSpec{
			Repo:            fmt.Sprintf("%s@%s:/home/%s/%s", bastionUser, internalIP, bastionUser, fleetExampleFolderName),
			Branch:          fleet.BranchName,
			Paths:           []string{fleet.GitRepoPathLinux},
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
			ClientSecretName: fleetSecretName,
		},
	}

	if a.clustersConfig.RKE2KubernetesVersions == nil {
		rke2Versions, err := kubernetesversions.ListRKE2AllVersions(a.client)
		require.NoError(a.T(), err)

		a.clustersConfig.RKE2KubernetesVersions = []string{rke2Versions[len(rke2Versions)-1]}
	}

	if a.clustersConfig.CNIs == nil {
		a.clustersConfig.CNIs = []string{fleet.CniCalico}
	}
}

func (a *AirGapRKE2CustomClusterTestSuite) TestCustomClusterWithGitRepo() {
	fleetVersion, err := fleet.GetDeploymentVersion(a.client, fleet.FleetControllerName, fleet.LocalName)
	require.NoError(a.T(), err)

	singleNodeRoles := []provisioninginput.MachinePools{provisioninginput.AllRolesMachinePool}

	testSession := session.NewSession()
	defer testSession.Cleanup()

	adminClient, err := a.client.WithSession(testSession)
	require.NoError(a.T(), err)

	provisioningConfig := *a.clustersConfig

	provisioningConfig.Hardened = true
	provisioningConfig.MachinePools = singleNodeRoles

	a.Run(fleet.FleetName+" "+fleetVersion+"-"+permutations.RKE2AirgapCluster, func() {

		logrus.Info("Deploying airgap fleet gitRepo")
		gitRepoObject, err := extensionsfleet.CreateFleetGitRepo(adminClient, a.fleetGitRepo)
		require.NoError(a.T(), err)

		logrus.Info("Deploying Custom Airgap Cluster")
		testClusterConfig := clusters.ConvertConfigToClusterConfig(&provisioningConfig)

		testClusterConfig.KubernetesVersion = a.clustersConfig.RKE2KubernetesVersions[0]
		testClusterConfig.CNI = a.clustersConfig.CNIs[0]

		clusterObject, err := provisioning.CreateProvisioningAirgapCustomCluster(a.standardClient, testClusterConfig, a.corralPackage)
		reports.TimeoutClusterReport(clusterObject, err)
		require.NoError(a.T(), err)

		provisioning.VerifyCluster(a.T(), a.standardClient, testClusterConfig, clusterObject)

		status := &apisV1.ClusterStatus{}
		err = steveV1.ConvertToK8sType(clusterObject.Status, status)
		require.NoError(a.T(), err)

		err = fleet.VerifyGitRepo(adminClient, gitRepoObject.ID, status.ClusterName, clusterObject.ID)
		require.NoError(a.T(), err)
	})

}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestFleetInAirGapProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(AirGapRKE2CustomClusterTestSuite))
}
