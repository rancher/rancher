package update

import (
	"fmt"
	"strings"
	"testing"

	"github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/fleet"
	projectsapi "github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	extensionClusters "github.com/rancher/shepherd/extensions/clusters"
	extensionsfleet "github.com/rancher/shepherd/extensions/fleet"
	"github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/nodes"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type UpgradeTestSuite struct {
	suite.Suite
	client          *rancher.Client
	session         *session.Session
	cluster         *management.Cluster
	clusterName     string
	sshNode         *nodes.Node
	fleetSecretName string
}

func (u *UpgradeTestSuite) TearDownSuite() {
	u.session.Cleanup()
}

func (u *UpgradeTestSuite) SetupSuite() {
	u.session = session.NewSession()

	client, err := rancher.NewClient("", u.session)
	require.NoError(u.T(), err)

	u.client = client

	log.Info("Getting cluster name from the config file and append cluster details in connection")
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(u.T(), clusterName, "Cluster name to install should be set")

	clusterID, err := extensionClusters.GetClusterIDByName(u.client, clusterName)
	require.NoError(u.T(), err, "Error getting cluster ID")

	u.cluster, err = u.client.Management.Cluster.ByID(clusterID)
	require.NoError(u.T(), err)

	provisioningClusterID, err := extensionClusters.GetV1ProvisioningClusterByName(client, clusterName)
	require.NoError(u.T(), err)

	cluster, err := client.Steve.SteveType(extensionClusters.ProvisioningSteveResourceType).ByID(provisioningClusterID)
	require.NoError(u.T(), err)

	newCluster := &provv1.Cluster{}
	err = steveV1.ConvertToK8sType(cluster, newCluster)
	require.NoError(u.T(), err)

	u.clusterName = client.RancherConfig.ClusterName
	if !strings.Contains(newCluster.Spec.KubernetesVersion, "k3s") && !strings.Contains(newCluster.Spec.KubernetesVersion, "rke2") {
		u.clusterName = u.cluster.ID
	}

	steveClient, err := u.client.Steve.ProxyDownstream(u.cluster.ID)
	require.NoError(u.T(), err)

	u.sshNode, err = createSSHNode(u.client, steveClient, u.cluster.Name, u.cluster.ID)
	require.NoError(u.T(), err)

	u.fleetSecretName, err = createFleetSSHSecret(u.client, string(u.sshNode.SSHKey))
	require.NoError(u.T(), err)
}

func (u *UpgradeTestSuite) TestNewCommitFleetRepo() {
	u.session = session.NewSession()

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(u.client, u.cluster.ID)
	require.NoError(u.T(), err)

	log.Info("Cloning Git Repo")
	repoName := namegenerator.AppendRandomString("repo-name")
	_, err = u.sshNode.ExecuteCommand(fmt.Sprintf("cd ~/ && git clone %s %s", fleet.ExampleRepo, repoName))
	require.NoError(u.T(), err)

	log.Info("Creating Fleet repo")
	repoObject, err := createFleetGitRepo(u.client, u.sshNode, repoName, namespace.Name, u.clusterName, u.cluster.ID, u.fleetSecretName)
	require.NoError(u.T(), err)

	log.Info("Getting GitRepoStatus")
	gitRepo, err := u.client.Steve.SteveType(extensionsfleet.FleetGitRepoResourceType).ByID(repoObject.ID)
	require.NoError(u.T(), err)

	gitStatus := &v1alpha1.GitRepoStatus{}
	err = steveV1.ConvertToK8sType(gitRepo.Status, gitStatus)
	require.NoError(u.T(), err)

	err = gitPushCommit(u.client, u.sshNode, repoName)
	require.NoError(u.T(), err)

	u.T().Log("Verifying the Fleet GitRepo")
	err = fleet.VerifyGitRepo(u.client, repoObject.ID, u.cluster.ID, fmt.Sprintf("%s/%s", fleet.Namespace, u.clusterName))
	require.NoError(u.T(), err)

	u.T().Log("Checking Fleet Git Commit has been updated")
	err = verifyGitCommit(u.client, gitStatus.Commit, repoObject.ID)
	require.NoError(u.T(), err)
}

func (u *UpgradeTestSuite) TestGitRepoForceUpdate() {
	u.session = session.NewSession()

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(u.client, u.cluster.ID)
	require.NoError(u.T(), err)

	log.Info("Cloning Git Repo")
	repoName := namegenerator.AppendRandomString("repo-name")
	_, err = u.sshNode.ExecuteCommand(fmt.Sprintf("cd ~/ && git clone %s %s", fleet.ExampleRepo, repoName))
	require.NoError(u.T(), err)

	log.Info("Creating Fleet repo")
	repoObject, err := createFleetGitRepo(u.client, u.sshNode, repoName, namespace.Name, u.clusterName, u.cluster.ID, u.fleetSecretName)
	require.NoError(u.T(), err)

	log.Info("Getting GitRepo")
	lastRepoObject, err := u.client.Steve.SteveType(extensionsfleet.FleetGitRepoResourceType).ByID(repoObject.ID)
	require.NoError(u.T(), err)

	gitRepo := &v1alpha1.GitRepo{}
	err = steveV1.ConvertToK8sType(lastRepoObject, gitRepo)
	require.NoError(u.T(), err)

	u.T().Log("Updating Fleet Repo")
	_, err = u.client.Steve.SteveType(extensionsfleet.FleetGitRepoResourceType).Update(lastRepoObject, gitRepo)
	require.NoError(u.T(), err)

	u.T().Log("Verifying the Fleet GitRepo")
	err = fleet.VerifyGitRepo(u.client, repoObject.ID, u.cluster.ID, fmt.Sprintf("%s/%s", fleet.Namespace, u.clusterName))
	require.NoError(u.T(), err)
}

func (u *UpgradeTestSuite) TestPauseFleetRepo() {
	u.session = session.NewSession()

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(u.client, u.cluster.ID)
	require.NoError(u.T(), err)

	log.Info("Cloning Git Repo")
	repoName := namegenerator.AppendRandomString("repo-name")
	_, err = u.sshNode.ExecuteCommand(fmt.Sprintf("cd ~/ && git clone %s %s", fleet.ExampleRepo, repoName))
	require.NoError(u.T(), err)

	log.Info("Creating Fleet repo")
	repoObject, err := createFleetGitRepo(u.client, u.sshNode, repoName, namespace.Name, u.clusterName, u.cluster.ID, u.fleetSecretName)
	require.NoError(u.T(), err)

	log.Info("Getting GitRepo")
	lastRepoObject, err := u.client.Steve.SteveType(extensionsfleet.FleetGitRepoResourceType).ByID(repoObject.ID)
	require.NoError(u.T(), err)

	gitRepo := &v1alpha1.GitRepo{}
	err = steveV1.ConvertToK8sType(lastRepoObject, gitRepo)
	require.NoError(u.T(), err)

	u.T().Log("Pausing Fleet Repo")
	gitRepo.Spec.Paused = true
	repoObject, err = u.client.Steve.SteveType(extensionsfleet.FleetGitRepoResourceType).Update(lastRepoObject, gitRepo)
	require.NoError(u.T(), err)

	err = gitPushCommit(u.client, u.sshNode, repoName)
	require.NoError(u.T(), err)

	log.Info("Getting last GitRepo")
	lastRepoObject, err = u.client.Steve.SteveType(extensionsfleet.FleetGitRepoResourceType).ByID(repoObject.ID)
	require.NoError(u.T(), err)

	log.Info("Checking last GitRepo was not updated")
	lastGitRepo := &v1alpha1.GitRepo{}
	err = steveV1.ConvertToK8sType(lastRepoObject, lastGitRepo)
	require.True(u.T(), lastGitRepo.Spec.Paused)
	require.NoError(u.T(), err)

	u.T().Log("Unpausing Fleet Repo")
	lastGitRepo.Spec.Paused = false
	repoObject, err = u.client.Steve.SteveType(extensionsfleet.FleetGitRepoResourceType).Update(repoObject, lastGitRepo)
	require.NoError(u.T(), err)

	u.T().Log("Verifying the Fleet Repo")
	err = fleet.VerifyGitRepo(u.client, repoObject.ID, u.cluster.ID, fmt.Sprintf("%s/%s", fleet.Namespace, u.clusterName))
	require.NoError(u.T(), err)

	u.T().Log("Checking Fleet Git Commit has been updated")
	err = verifyGitCommit(u.client, gitRepo.Status.Commit, repoObject.ID)
	require.NoError(u.T(), err)
}

func TestUpgradeTestSuite(t *testing.T) {
	suite.Run(t, new(UpgradeTestSuite))
}
