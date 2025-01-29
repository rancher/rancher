package update

import (
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"testing"
	"time"

	"errors"

	"github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/git"
	"github.com/rancher/rancher/tests/v2/actions/clusters"
	"github.com/rancher/rancher/tests/v2/actions/fleet"
	projectsapi "github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/services"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	extensionClusters "github.com/rancher/shepherd/extensions/clusters"
	extensionsfleet "github.com/rancher/shepherd/extensions/fleet"
	"github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	nodePoolsize   = 3
	guestbookLabel = "labelSelector=app=guestbook"
)

type UpgradeTestSuite struct {
	suite.Suite
	client    *rancher.Client
	session   *session.Session
	cluster   *management.Cluster
	gitRepo   *v1alpha1.GitRepo
	clusterID string
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

	u.clusterID = client.RancherConfig.ClusterName
	if !strings.Contains(newCluster.Spec.KubernetesVersion, "k3s") && !strings.Contains(newCluster.Spec.KubernetesVersion, "rke2") {
		u.clusterID = u.cluster.ID
	}

	require.NotEmptyf(u.T(), u.clusterID, "Cluster ID should be set")

	u.gitRepo = fleet.GitRepoConfig()
}

func (u *UpgradeTestSuite) TestDeployFleetRepo() {
	u.session = session.NewSession()

	steveClient, err := u.client.Steve.ProxyDownstream(u.cluster.ID)
	require.NoError(u.T(), err)

	err = clusters.VerifyNodePoolSize(steveClient, nodePoolsize)
	if errors.Is(err, clusters.SmallerPoolClusterSize) {
		u.T().Skip("The deploy fleet repo and upgrade test requires at least 3 worker nodes.")
	} else {
		require.NoError(u.T(), err)
	}

	if u.gitRepo.Spec.Repo == "" {
		u.T().Skip("The deploy fleet repo and upgrade test require the gitRepo Repo field to be configured.")
	}

	if !strings.Contains(u.gitRepo.Spec.Repo, "https") {
		u.T().Skip("The deploy fleet repo and upgrade test require the gitRepo Repo field to be a https url.")
	}

	if !strings.Contains(u.gitRepo.Spec.Repo, "fleet-examples") {
		u.T().Skip("The deploy fleet repo and upgrade test require the gitRepo Repo field to be a fork of fleet-examples repository.")
	}

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(u.client, u.cluster.ID)
	require.NoError(u.T(), err)

	fleetGitRepo := &v1alpha1.GitRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fleet.FleetMetaName + namegenerator.RandStringLower(5),
			Namespace: fleet.Namespace,
		},
		Spec: v1alpha1.GitRepoSpec{
			Repo:            u.gitRepo.Spec.Repo,
			Branch:          fleet.BranchName,
			Paths:           []string{fleet.GitRepoPathLinux},
			TargetNamespace: namespace.Name,
			Targets: []v1alpha1.GitTarget{
				{
					ClusterName: u.clusterID,
				},
			},
		},
	}

	u.T().Log("Creating a fleet git repo")
	repoObject, err := extensionsfleet.CreateFleetGitRepo(u.client, fleetGitRepo)
	require.NoError(u.T(), err)

	u.T().Log("Verifying the Git repository")
	err = fleet.VerifyGitRepo(u.client, repoObject.ID, u.cluster.ID, fmt.Sprintf("%s/%s", fleet.Namespace, u.clusterID))
	require.NoError(u.T(), err)

	query, err := url.ParseQuery(guestbookLabel)
	require.NoError(u.T(), err)

	servicesResp, err := steveClient.SteveType(services.ServiceSteveType).NamespacedSteveClient(namespace.Name).List(query)
	require.NoError(u.T(), err)
	require.NotEmpty(u.T(), servicesResp.Data)

	u.T().Log("Verifying Cluster IP")
	for _, serviceResp := range servicesResp.Data {
		err = services.VerifyClusterIP(u.client, u.clusterID, steveClient, serviceResp.ID, "80", "Guestbook")
		require.NoError(u.T(), err)
	}
}

func (u *UpgradeTestSuite) TestAutoUpgradeFleetRepo() {
	u.session = session.NewSession()

	steveClient, err := u.client.Steve.ProxyDownstream(u.cluster.ID)
	require.NoError(u.T(), err)

	err = clusters.VerifyNodePoolSize(steveClient, nodePoolsize)
	if errors.Is(err, clusters.SmallerPoolClusterSize) {
		u.T().Skip("The deploy fleet repo and upgrade test requires at least 3 worker nodes.")
	} else {
		require.NoError(u.T(), err)
	}

	if u.gitRepo.Spec.Repo == "" {
		u.T().Skip("The deploy fleet repo and upgrade test require the gitRepo Repo field to be configured.")
	}

	if !strings.Contains(u.gitRepo.Spec.Repo, "https") {
		u.T().Skip("The deploy fleet repo and upgrade test require the gitRepo Repo field to be a https url.")
	}

	if !strings.Contains(u.gitRepo.Spec.Repo, "fleet-examples") {
		u.T().Skip("The deploy fleet repo and upgrade test require the gitRepo Repo field to be a fork of fleet-examples repository.")
	}

	if u.gitRepo.Spec.ClientSecretName == "" {
		u.T().Skip("The deploy fleet repo and upgrade test require the gitRepo ClientSecretName field to be configured.")
	}

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(u.client, u.cluster.ID)
	require.NoError(u.T(), err)

	secret, err := u.client.WranglerContext.Core.Secret().Get(fleet.Namespace, u.gitRepo.Spec.ClientSecretName, v1.GetOptions{})
	require.NoError(u.T(), err)

	username := string(secret.Data["username"])
	require.NotEmpty(u.T(), username)

	accessToken := string(secret.Data["password"])
	require.NotEmpty(u.T(), accessToken)

	fleetGitRepo := &v1alpha1.GitRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fleet.FleetMetaName + namegenerator.RandStringLower(5),
			Namespace: fleet.Namespace,
		},
		Spec: v1alpha1.GitRepoSpec{
			Repo:            u.gitRepo.Spec.Repo,
			Branch:          fleet.BranchName,
			TargetNamespace: namespace.Name,
			Paths:           []string{fleet.GitRepoPathLinux},
			Targets: []v1alpha1.GitTarget{
				{
					ClusterName: u.clusterID,
				},
			},
		},
	}

	u.T().Log("Creating a fleet git repo")
	repoObject, err := extensionsfleet.CreateFleetGitRepo(u.client, fleetGitRepo)
	require.NoError(u.T(), err)

	u.T().Log("Verifying the Git repository")
	err = fleet.VerifyGitRepo(u.client, repoObject.ID, u.cluster.ID, fmt.Sprintf("%s/%s", fleet.Namespace, u.clusterID))
	require.NoError(u.T(), err)

	query, err := url.ParseQuery(guestbookLabel)
	require.NoError(u.T(), err)

	servicesResp, err := steveClient.SteveType(services.ServiceSteveType).NamespacedSteveClient(namespace.Name).List(query)
	require.NoError(u.T(), err)
	require.NotEmpty(u.T(), servicesResp.Data)

	u.T().Log("Verifying Cluster IP")
	for _, serviceResp := range servicesResp.Data {
		err = services.VerifyClusterIP(u.client, u.clusterID, steveClient, serviceResp.ID, "80", "Guestbook")
		require.NoError(u.T(), err)
	}

	gitRepo, err := u.client.Steve.SteveType(extensionsfleet.FleetGitRepoResourceType).ByID(repoObject.ID)
	require.NoError(u.T(), err)

	gitStatus := &v1alpha1.GitRepoStatus{}
	err = steveV1.ConvertToK8sType(gitRepo.Status, gitStatus)
	require.NoError(u.T(), err)

	repoName := namegenerator.AppendRandomString("repo-name")

	u.T().Log("Cloning Repository")
	err = git.Clone(repoName, u.gitRepo.Spec.Repo, fleet.BranchName)
	require.NoError(u.T(), err)

	u.T().Log("Configuring Git User Name")
	cmd := exec.Command("git", "-C", repoName, "config", "--local", "user.name", username)
	err = cmd.Run()
	require.NoError(u.T(), err)

	u.T().Log("Configuring Git User Email")
	cmd = exec.Command("git", "-C", repoName, "config", "--local", "user.email", username)
	err = cmd.Run()
	require.NoError(u.T(), err)

	u.T().Log("Git Commit")
	cmd = exec.Command("git", "-C", repoName, "commit", "--allow-empty", "-m", "Trigger fleet update")
	err = cmd.Run()
	require.NoError(u.T(), err)

	u.T().Log("Git Push")
	repoWithoutHttps := strings.Replace(u.gitRepo.Spec.Repo, "https://", "", 1)
	repoWithToken := fmt.Sprintf("https://%s:%s@%s", username, accessToken, repoWithoutHttps)
	cmd = exec.Command("git", "-C", repoName, "push", "--force", "-u", repoWithToken)
	err = cmd.Run()
	require.NoError(u.T(), err)

	u.T().Log("Verifying the Git repository")
	err = fleet.VerifyGitRepo(u.client, repoObject.ID, u.cluster.ID, fmt.Sprintf("%s/%s", fleet.Namespace, u.clusterID))
	require.NoError(u.T(), err)

	u.T().Log("Checking Git Commit")
	err = verifyGitCommit(u.client, gitStatus, repoObject.ID)
	require.NoError(u.T(), err)
}

func (u *UpgradeTestSuite) TestForceUpdateFleetRepo() {
	u.session = session.NewSession()

	steveClient, err := u.client.Steve.ProxyDownstream(u.cluster.ID)
	require.NoError(u.T(), err)

	err = clusters.VerifyNodePoolSize(steveClient, nodePoolsize)
	if errors.Is(err, clusters.SmallerPoolClusterSize) {
		u.T().Skip("The deploy fleet repo and upgrade test requires at least 3 worker nodes.")
	} else {
		require.NoError(u.T(), err)
	}

	if u.gitRepo.Spec.Repo == "" {
		u.T().Skip("The deploy fleet repo and upgrade test require the gitRepo Repo field to be configured.")
	}

	if !strings.Contains(u.gitRepo.Spec.Repo, "https") {
		u.T().Skip("The deploy fleet repo and upgrade test require the gitRepo Repo field to be a https url.")
	}

	if !strings.Contains(u.gitRepo.Spec.Repo, "fleet-examples") {
		u.T().Skip("The deploy fleet repo and upgrade test require the gitRepo Repo field to be a fork of fleet-examples repository.")
	}

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(u.client, u.cluster.ID)
	require.NoError(u.T(), err)

	u.T().Log("Creating a fleet git repo")
	fleetGitRepo := &v1alpha1.GitRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fleet.FleetMetaName + namegenerator.RandStringLower(5),
			Namespace: fleet.Namespace,
		},
		Spec: v1alpha1.GitRepoSpec{
			Repo:            u.gitRepo.Spec.Repo,
			Branch:          fleet.BranchName,
			Paths:           []string{fleet.GitRepoPathLinux},
			TargetNamespace: namespace.Name,
			Targets: []v1alpha1.GitTarget{
				{
					ClusterName: u.clusterID,
				},
			},
		},
	}

	repoObject, err := extensionsfleet.CreateFleetGitRepo(u.client, fleetGitRepo)
	require.NoError(u.T(), err)

	u.T().Log("Verifying the Git repository")
	err = fleet.VerifyGitRepo(u.client, repoObject.ID, u.cluster.ID, fmt.Sprintf("%s/%s", fleet.Namespace, u.clusterID))
	require.NoError(u.T(), err)

	query, err := url.ParseQuery(guestbookLabel)
	require.NoError(u.T(), err)

	servicesResp, err := steveClient.SteveType(services.ServiceSteveType).NamespacedSteveClient(namespace.Name).List(query)
	require.NoError(u.T(), err)
	require.NotEmpty(u.T(), servicesResp.Data)

	u.T().Log("Verifying Cluster IP")
	for _, serviceResp := range servicesResp.Data {
		err = services.VerifyClusterIP(u.client, u.clusterID, steveClient, serviceResp.ID, "80", "Guestbook")
		require.NoError(u.T(), err)
	}

	lastRepoObject, err := u.client.Steve.SteveType(extensionsfleet.FleetGitRepoResourceType).ByID(repoObject.ID)
	require.NoError(u.T(), err)

	gitRepo := &v1alpha1.GitRepo{}
	err = steveV1.ConvertToK8sType(lastRepoObject, gitRepo)
	require.NoError(u.T(), err)

	u.T().Log("Updating Fleet Repo")
	_, err = u.client.Steve.SteveType(extensionsfleet.FleetGitRepoResourceType).Update(lastRepoObject, gitRepo)
	require.NoError(u.T(), err)

	u.T().Log("Verifying the Git repository")
	err = fleet.VerifyGitRepo(u.client, repoObject.ID, u.cluster.ID, fmt.Sprintf("%s/%s", fleet.Namespace, u.clusterID))
	require.NoError(u.T(), err)
}

func (u *UpgradeTestSuite) TestPauseUpdateFleetRepo() {
	u.session = session.NewSession()

	steveClient, err := u.client.Steve.ProxyDownstream(u.cluster.ID)
	require.NoError(u.T(), err)

	err = clusters.VerifyNodePoolSize(steveClient, nodePoolsize)
	if errors.Is(err, clusters.SmallerPoolClusterSize) {
		u.T().Skip("The deploy fleet repo and upgrade test requires at least 3 worker nodes.")
	} else {
		require.NoError(u.T(), err)
	}

	if u.gitRepo.Spec.Repo == "" {
		u.T().Skip("The deploy fleet repo and upgrade test require the gitRepo Repo field to be configured.")
	}

	if !strings.Contains(u.gitRepo.Spec.Repo, "https") {
		u.T().Skip("The deploy fleet repo and upgrade test require the gitRepo Repo field to be a https url.")
	}

	if !strings.Contains(u.gitRepo.Spec.Repo, "fleet-examples") {
		u.T().Skip("The deploy fleet repo and upgrade test require the gitRepo Repo field to be a fork of fleet-examples repository.")
	}

	if u.gitRepo.Spec.ClientSecretName == "" {
		u.T().Skip("The deploy fleet repo and upgrade test require the gitRepo ClientSecretName field to be configured.")
	}

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(u.client, u.cluster.ID)
	require.NoError(u.T(), err)

	secret, err := u.client.WranglerContext.Core.Secret().Get(fleet.Namespace, u.gitRepo.Spec.ClientSecretName, v1.GetOptions{})
	require.NoError(u.T(), err)

	username := string(secret.Data["username"])
	require.NotEmpty(u.T(), username)

	accessToken := string(secret.Data["password"])
	require.NotEmpty(u.T(), accessToken)

	u.T().Log("Creating a fleet git repo")
	fleetGitRepo := &v1alpha1.GitRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fleet.FleetMetaName + namegenerator.RandStringLower(5),
			Namespace: fleet.Namespace,
		},
		Spec: v1alpha1.GitRepoSpec{
			Repo:            u.gitRepo.Spec.Repo,
			Branch:          fleet.BranchName,
			TargetNamespace: namespace.Name,
			Paths:           []string{fleet.GitRepoPathLinux},
			Targets: []v1alpha1.GitTarget{
				{
					ClusterName: u.clusterID,
				},
			},
		},
	}

	repoObject, err := extensionsfleet.CreateFleetGitRepo(u.client, fleetGitRepo)
	require.NoError(u.T(), err)

	u.T().Log("Verifying the Git repository")
	err = fleet.VerifyGitRepo(u.client, repoObject.ID, u.cluster.ID, fmt.Sprintf("%s/%s", fleet.Namespace, u.clusterID))
	require.NoError(u.T(), err)

	query, err := url.ParseQuery(guestbookLabel)
	require.NoError(u.T(), err)

	servicesResp, err := steveClient.SteveType(services.ServiceSteveType).NamespacedSteveClient(namespace.Name).List(query)
	require.NoError(u.T(), err)
	require.NotEmpty(u.T(), servicesResp.Data)

	u.T().Log("Verifying Cluster IP")
	for _, serviceResp := range servicesResp.Data {
		err = services.VerifyClusterIP(u.client, u.clusterID, steveClient, serviceResp.ID, "80", "Guestbook")
		require.NoError(u.T(), err)
	}

	lastRepoObject, err := u.client.Steve.SteveType(extensionsfleet.FleetGitRepoResourceType).ByID(repoObject.ID)
	require.NoError(u.T(), err)

	gitRepo := &v1alpha1.GitRepo{}
	err = steveV1.ConvertToK8sType(lastRepoObject, gitRepo)
	require.NoError(u.T(), err)

	u.T().Log("Pausing Fleet Repo")
	gitRepo.Spec.Paused = true
	repoObject, err = u.client.Steve.SteveType(extensionsfleet.FleetGitRepoResourceType).Update(lastRepoObject, gitRepo)
	require.NoError(u.T(), err)

	u.T().Log("Verifying the Git repository")
	err = fleet.VerifyGitRepo(u.client, repoObject.ID, u.cluster.ID, fmt.Sprintf("%s/%s", fleet.Namespace, u.clusterID))
	require.NoError(u.T(), err)

	repoName := namegenerator.AppendRandomString("repo-name")

	u.T().Log("Cloning Repository")
	err = git.Clone(repoName, u.gitRepo.Spec.Repo, fleet.BranchName)
	require.NoError(u.T(), err)

	u.T().Log("Configuring Git User Name")
	cmd := exec.Command("git", "-C", repoName, "config", "--local", "user.name", username)
	err = cmd.Run()
	require.NoError(u.T(), err)

	u.T().Log("Configuring Git User Email")
	cmd = exec.Command("git", "-C", repoName, "config", "--local", "user.email", username)
	err = cmd.Run()
	require.NoError(u.T(), err)

	u.T().Log("Git Commit")
	cmd = exec.Command("git", "-C", repoName, "commit", "--allow-empty", "-m", "Trigger fleet update")
	err = cmd.Run()
	require.NoError(u.T(), err)

	u.T().Log("Git Push")
	repoWithoutHttps := strings.Replace(u.gitRepo.Spec.Repo, "https://", "", 1)
	repoWithToken := fmt.Sprintf("https://%s:%s@%s", username, accessToken, repoWithoutHttps)
	cmd = exec.Command("git", "-C", repoName, "push", "--force", "-u", repoWithToken)
	err = cmd.Run()
	require.NoError(u.T(), err)

	u.T().Log("Verifying the Git repository")
	err = fleet.VerifyGitRepo(u.client, repoObject.ID, u.cluster.ID, fmt.Sprintf("%s/%s", fleet.Namespace, u.clusterID))
	require.NoError(u.T(), err)

	lastRepoObject, err = u.client.Steve.SteveType(extensionsfleet.FleetGitRepoResourceType).ByID(repoObject.ID)
	require.NoError(u.T(), err)

	lastGitRepo := &v1alpha1.GitRepo{}
	err = steveV1.ConvertToK8sType(lastRepoObject, lastGitRepo)
	require.NoError(u.T(), err)

	u.T().Log("Unpausing Fleet Repo")
	lastGitRepo.Spec.Paused = false
	repoObject, err = u.client.Steve.SteveType(extensionsfleet.FleetGitRepoResourceType).Update(repoObject, lastGitRepo)
	require.NoError(u.T(), err)

	u.T().Log("Verifying the Git repository")
	err = fleet.VerifyGitRepo(u.client, repoObject.ID, u.cluster.ID, fmt.Sprintf("%s/%s", fleet.Namespace, u.clusterID))
	require.NoError(u.T(), err)

	u.T().Log("Checking Git Commit")
	err = verifyGitCommit(u.client, &gitRepo.Status, repoObject.ID)
	require.NoError(u.T(), err)
}

func verifyGitCommit(client *rancher.Client, gitStatus *v1alpha1.GitRepoStatus, repoObjectID string) error {
	backoff := kwait.Backoff{
		Duration: 1 * time.Second,
		Factor:   1.1,
		Jitter:   0.1,
		Steps:    20,
	}

	err := kwait.ExponentialBackoff(backoff, func() (finished bool, err error) {
		gitRepo, err := client.Steve.SteveType(extensionsfleet.FleetGitRepoResourceType).ByID(repoObjectID)
		if err != nil {
			return true, err
		}

		newGitStatus := &v1alpha1.GitRepoStatus{}
		err = steveV1.ConvertToK8sType(gitRepo.Status, newGitStatus)
		if err != nil {
			return true, err
		}

		return gitStatus.Commit != newGitStatus.Commit, nil
	})
	return err
}
func TestUpgradeTestSuite(t *testing.T) {
	suite.Run(t, new(UpgradeTestSuite))
}
