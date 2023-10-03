package integration

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	users "github.com/rancher/rancher/tests/framework/extensions/users"
	password "github.com/rancher/rancher/tests/framework/extensions/users/passwordgenerator"

	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/clients/rancher/catalog"
	stevev1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	LocalClusterID                    = "local"
	ChartsSmallForkRepoName           = "charts-small-fork"
	ChartsSmallForkGitRepoURL         = "https://github.com/rancher/charts-small-fork"
	ChartsSmallForkGitRepoFirstBranch = "test-1"
	ChartsSmallForkGitRepoLastBranch  = "main"

	RKE2ChartsGitRepoURL = "https://github.com/rancher/rke2-charts"

	HTTPClusterRepoName = "test-http-cluster-repo"
	LatestHTTPRepoURL   = "https://releases.rancher.com/server-charts/latest"
	StableHTTPRepoURL   = "https://releases.rancher.com/server-charts/stable"
)

var (
	ChartSmallForkDir = fmt.Sprintf("/go/src/github.com/rancher/rancher/build/testdata/management-state/git-repo/%s", ChartsSmallForkRepoName)
	PollInterval      = time.Duration(500 * time.Millisecond)
	PollTimeout       = time.Duration(5 * time.Minute)
)

// ClusterRepoParams is used to pass params to func testClusterRepo for testing
type ClusterRepoParams struct {
	Name string   // Name of the ClusterRepo resource
	Type RepoType // Type of the ClusterRepo resource
	URL1 string   // URL to use when creating the ClusterRepo resource
	URL2 string   // URL to use when updating the ClusterRepo resource to a new URL
}

// ClusterRepoParams is used to pass params to func testClusterRepo for testing
type ChartsSmallForkRepoParams struct {
	Name    string   // Name of the ClusterRepo resource
	Type    RepoType // Type of the ClusterRepo resource
	URL     string
	Branch1 string // First branch to test at charts-small-fork
	Branch2 string // Last branch to test at charts-small-fork
}

type RepoType int64

const (
	Git RepoType = iota
	HTTP
)

type ClusterRepoTestSuite struct {
	suite.Suite
	client        *rancher.Client
	session       *session.Session
	clusterID     string
	catalogClient *catalog.Client
	ctx           context.Context
}

func TestClusterRepoTestSuite(t *testing.T) {
	suite.Run(t, new(ClusterRepoTestSuite))
}

func (c *ClusterRepoTestSuite) TearDownSuite() {
	c.session.Cleanup()
}

func (c *ClusterRepoTestSuite) SetupSuite() {
	var err error
	c.ctx = context.Background()

	testSession := session.NewSession()
	c.session = testSession

	c.client, err = rancher.NewClient("", testSession)
	require.NoError(c.T(), err)

	c.clusterID = LocalClusterID
	c.catalogClient, err = c.client.GetClusterCatalogClient(c.clusterID)
	require.NoError(c.T(), err)
}

// TestHTTPRepo tests CREATE, UPDATE, and DELETE operations of HTTP ClusterRepo resources
func (c *ClusterRepoTestSuite) TestHTTPRepo() {
	c.testClusterRepo(ClusterRepoParams{
		Name: HTTPClusterRepoName,
		URL1: LatestHTTPRepoURL,
		URL2: StableHTTPRepoURL,
		Type: HTTP,
	})
}

// TestChartSmallForkGitRepo tests the local repository git state vs ClusterRepo(Spec and Status)
func (c *ClusterRepoTestSuite) TestChartSmallForkGitRepo() {
	c.testSmallForkClusterRepo(ChartsSmallForkRepoParams{
		Name:    ChartsSmallForkRepoName,
		Type:    Git,
		URL:     ChartsSmallForkGitRepoURL,
		Branch1: ChartsSmallForkGitRepoFirstBranch,
		Branch2: ChartsSmallForkGitRepoLastBranch,
	})
}

// testClusterRepo takes in ClusterRepoParams and tests CREATE, UPDATE, and DELETE operations
func (c *ClusterRepoTestSuite) testClusterRepo(params ClusterRepoParams) {
	// Create a ClusterRepo
	cr := v1.NewClusterRepo("", params.Name, v1.ClusterRepo{})
	setClusterRepoURL(&cr.Spec, params.Type, params.URL1)
	_, err := c.client.Steve.SteveType(catalog.ClusterRepoSteveResourceType).Create(cr)
	require.NoError(c.T(), err)
	time.Sleep(1 * time.Second)

	// Validate the ClusterRepo was created and resources were downloaded
	clusterRepo, err := c.pollUntilDownloaded(params.Name, metav1.Time{})
	require.NoError(c.T(), err)

	status := c.getStatusFromClusterRepo(clusterRepo)
	assert.Equal(c.T(), params.URL1, status.URL)

	// Save download timestamp and generation count before changing the URL
	downloadTime := status.DownloadTime
	observedGeneration := status.ObservedGeneration

	// Validate updating the ClusterRepo by changing the repo URL and verifying DownloadTime was updated (meaning new resources were pulled)
	spec := c.getSpecFromClusterRepo(clusterRepo)
	setClusterRepoURL(spec, params.Type, params.URL2)
	clusterRepoUpdated := *clusterRepo
	clusterRepoUpdated.Spec = spec
	_, err = c.client.Steve.SteveType(catalog.ClusterRepoSteveResourceType).Replace(&clusterRepoUpdated)
	require.NoError(c.T(), err)
	time.Sleep(1 * time.Second)

	clusterRepo, err = c.pollUntilDownloaded(params.Name, downloadTime)
	require.NoError(c.T(), err)

	status = c.getStatusFromClusterRepo(clusterRepo)
	assert.Equal(c.T(), params.URL2, status.URL)
	assert.Greater(c.T(), status.ObservedGeneration, observedGeneration)

	// Validate deleting the ClusterRepo
	err = c.client.Steve.SteveType(catalog.ClusterRepoSteveResourceType).Delete(clusterRepo)
	require.NoError(c.T(), err)

	_, err = c.client.Steve.SteveType(catalog.ClusterRepoSteveResourceType).ByID(params.Name)
	require.Error(c.T(), err)
}

// testSmallForkClusterRepo takes in ChartsSmallForkRepoParams
// and asserts the current state of the local repository directory to the Spec and Status of created and updated ClusterRepo.
func (c *ClusterRepoTestSuite) testSmallForkClusterRepo(params ChartsSmallForkRepoParams) {
	var err error
	var firstCommit, firstBranch string
	var lastCommit, lastBranch string
	var createdClusterRepo, testClusterRepo, updatedClusterRepo *v1.ClusterRepo
	var wg sync.WaitGroup

	// Operations as Admin
	// Creates a new ClusterRepo Kubernetes custom resource
	createdClusterRepo, err = c.catalogClient.ClusterRepos().Create(c.ctx,
		&v1.ClusterRepo{
			ObjectMeta: metav1.ObjectMeta{
				Name: ChartsSmallForkRepoName,
			},
			Spec: v1.RepoSpec{
				GitRepo:   ChartsSmallForkGitRepoURL,
				GitBranch: ChartsSmallForkGitRepoFirstBranch,
			},
		}, metav1.CreateOptions{})

	require.NoError(c.T(), err)

	// Test RBAC concurrently after creating the test cluster catalog "charts-small-fork"
	wg.Add(1)
	go c.testRBACClusterRepo(&wg)

	// List all available installed Cluster Repos
	installedClusterRepos, err := c.catalogClient.ClusterRepos().List(c.ctx, metav1.ListOptions{})
	require.NoError(c.T(), err)

	// Check if our created ClusterRepo (charts-small-fork) was created
	success := false
	for _, cr := range installedClusterRepos.Items {
		logrus.Debugf("Installed Cluster Repo: %s", cr.Name)
		if cr.Name == createdClusterRepo.Name {
			success = true
		}
	}
	require.Equal(c.T(), true, success)
	require.NoError(c.T(), err)

	// Wait until ClusterRepo.Status.Commit reflects the first commit at the local repository
	err = kwait.Poll(5*time.Second, 2*time.Minute, func() (done bool, err error) {
		// Get the path to the local repository and assert it has no error
		testClusterRepo, err = c.catalogClient.ClusterRepos().Get(c.ctx, createdClusterRepo.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if testClusterRepo.Status.Commit != "" {
			return true, nil
		}
		return false, nil
	})
	require.NoError(c.T(), err)

	// We have waited for ClusterRepo status to update and the local repository to be created
	repoPath, err := getCurrentRepoDirSmallFork()
	require.NoError(c.T(), err)
	firstCommit, firstBranch, err = getLocalRepoCurrentCommitAndBranch(repoPath)
	require.NoError(c.T(), err)

	// Compare ClusterRepo Values with the local repository
	assert.Equal(c.T(), firstBranch, testClusterRepo.Spec.GitBranch)
	assert.Equal(c.T(), firstBranch, testClusterRepo.Status.Branch)
	assert.Equal(c.T(), firstCommit, testClusterRepo.Status.Commit)
	assert.Equal(c.T(), int64(1), testClusterRepo.Status.ObservedGeneration)

	// Updating ClusterRepo Spec Branch to a newer one
	testClusterRepo.Spec.GitBranch = ChartsSmallForkGitRepoLastBranch
	updatedClusterRepo, err = c.catalogClient.ClusterRepos().Update(c.ctx, testClusterRepo.DeepCopy(), metav1.UpdateOptions{})
	require.NoError(c.T(), err)
	assert.Equal(c.T(), ChartsSmallForkGitRepoLastBranch, updatedClusterRepo.Spec.GitBranch)

	// The Spec from ClusterRepo is updated almost instantly, the status and local repository take more time
	err = kwait.Poll(5*time.Second, 10*time.Minute, func() (done bool, err error) {
		lastCommit, _, err := getLocalRepoCurrentCommitAndBranch(repoPath)
		require.NoError(c.T(), err)
		updatedClusterRepo, err = c.catalogClient.ClusterRepos().Get(c.ctx, testClusterRepo.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		// Assertions
		if lastCommit == updatedClusterRepo.Status.Commit && lastCommit != firstCommit {
			return true, nil
		}
		return false, nil
	})
	logrus.Debug("last commit: ", lastCommit)
	logrus.Debug("last branch: ", lastBranch)
	require.NoError(c.T(), err)
	// Wait for both tests finishing
	wg.Wait()
}

// testRBACClusterRepo tests RBAC (Role-Based Access Control) functionality for Cluster Repositories.
// It creates roles, users with roles, Cluster RoleTemplate Bindings, and performs RBAC checks.
func (c *ClusterRepoTestSuite) testRBACClusterRepo(wg *sync.WaitGroup) {
	defer wg.Done()
	// Create role templates
	roleName1 := "catalog-view-target"
	roleName2 := "catalog-view-all"
	role1, role2 := c.createRoleTemplates(roleName1, roleName2)
	// Create users with roles
	user1 := c.createUserWithDefaultGlobalRole("rbac-catalog-user-test-1")
	user2 := c.createUserWithDefaultGlobalRole("rbac-catalog-user-test-2")
	// Create Cluster RoleTemplate Bindings
	c.createClusterRoleTemplateBindings(user1.ID, user2.ID, role1.ID, role2.ID)

	ctx := context.Background()
	// Test user1's access to Cluster Repositories
	testUser1, err := c.client.AsUser(user1)
	require.NoError(c.T(), err)
	_, err = testUser1.Catalog.ClusterRepos().List(ctx, metav1.ListOptions{})
	var expectedErrorCode int32 = 403
	var expectedErrorReason string = "Forbidden"
	statusErr, ok := err.(*errors.StatusError)
	require.True(c.T(), ok, "Expected error of type StatusError, but got a different error type.")
	require.Equal(c.T(), expectedErrorCode, statusErr.ErrStatus.Code, "Expected error Code to be %d, but got %d.", expectedErrorCode, statusErr.ErrStatus.Code)
	require.Equal(c.T(), expectedErrorReason, string(statusErr.ErrStatus.Reason), "Expected error Reason to be %s, but got %s.", expectedErrorReason, string(statusErr.ErrStatus.Reason))

	user1ClusterRepos, err := testUser1.Catalog.ClusterRepos().Get(ctx, ChartsSmallForkRepoName, metav1.GetOptions{})
	_ = user1ClusterRepos
	require.NoError(c.T(), err)
	require.Equal(c.T(), user1ClusterRepos.Name, string(ChartsSmallForkRepoName))
	// Test user2's access to Cluster Repositories
	testUser2, err := c.client.AsUser(user2)
	require.NoError(c.T(), err)
	user2ClusterRepos, err := testUser2.Catalog.ClusterRepos().List(ctx, metav1.ListOptions{})
	require.NoError(c.T(), err)
	require.GreaterOrEqual(c.T(), len(user2ClusterRepos.Items), 4)
}

// createUserWithDefaultGlobalRole creates a new user with the specified username
// and assigns them the "user-base" General Role Template, which grants only the login permission.
// It generates a random password for the user and returns the created user object.
func (c *ClusterRepoTestSuite) createUserWithDefaultGlobalRole(userName string) *management.User {
	// Enable the user account
	enabled := true

	// Generate a random test password for the user
	var testPassword = password.GenerateUserPassword("testpass-")

	// Create a new user object with the provided username, password, and name
	user := &management.User{
		Username: userName,
		Password: testPassword,
		Name:     userName,
		Enabled:  &enabled,
	}

	// Create the new user with the "user-base" role
	newUser, err := users.CreateUserWithRole(c.client, user, "user-base")
	require.NoError(c.T(), err)

	// Set the user's password to the generated password
	newUser.Password = user.Password

	// Return the created user object
	return newUser
}

// createRoleTemplates creates two Role Templates with slightly different sets of rules for testing purposes.
// It takes two role names as input and returns pointers to the created Role Template objects.
func (c *ClusterRepoTestSuite) createRoleTemplates(roleName1, roleName2 string) (*management.RoleTemplate, *management.RoleTemplate) {
	// Create the first Role Template with target resourceNames
	roleTemplate1, err := c.client.Management.RoleTemplate.Create(&management.RoleTemplate{
		Context: "cluster",
		Name:    roleName1,
		Rules: []management.PolicyRule{
			{
				APIGroups:     []string{"catalog.cattle.io"},
				Resources:     []string{"clusterrepos"},
				ResourceNames: []string{ChartsSmallForkRepoName},
				Verbs:         []string{"get", "list", "watch"},
			},
		},
	})
	require.NoError(c.T(), err)

	// Create the second Role Template
	roleTemplate2, err := c.client.Management.RoleTemplate.Create(&management.RoleTemplate{
		Context: "cluster",
		Name:    roleName2,
		Rules: []management.PolicyRule{
			{
				APIGroups:     []string{"catalog.cattle.io"},
				Resources:     []string{"clusterrepos"},
				ResourceNames: []string{},
				Verbs:         []string{"get", "list", "watch"},
			},
		},
	})
	require.NoError(c.T(), err)

	return roleTemplate1, roleTemplate2
}

// createClusterRoleTemplateBindings creates ClusterRoleTemplateBindings for two users with corresponding roles.
func (c *ClusterRepoTestSuite) createClusterRoleTemplateBindings(user1ID, user2ID, role1ID, role2ID string) {
	// Create ClusterRoleTemplateBinding for user1 and role1
	_, err := c.client.Management.ClusterRoleTemplateBinding.Create(&management.ClusterRoleTemplateBinding{
		Name:            "cluster-role-template-binding-1",
		ClusterID:       c.clusterID,
		RoleTemplateID:  role1ID,
		UserPrincipalID: fmt.Sprintf("%s://%s", c.clusterID, user1ID),
	})
	require.NoError(c.T(), err)

	// Create ClusterRoleTemplateBinding for user2 and role2
	_, err = c.client.Management.ClusterRoleTemplateBinding.Create(&management.ClusterRoleTemplateBinding{
		Name:            "cluster-role-template-binding-2",
		ClusterID:       c.clusterID,
		RoleTemplateID:  role2ID,
		UserPrincipalID: fmt.Sprintf("%s://%s", c.clusterID, user2ID),
	})
	require.NoError(c.T(), err)
}

// pollUntilDownloaded Polls until the ClusterRepo of the given name has been downloaded (by comparing prevDownloadTime against the current DownloadTime)
func (c *ClusterRepoTestSuite) pollUntilDownloaded(ClusterRepoName string, prevDownloadTime metav1.Time) (*stevev1.SteveAPIObject, error) {
	var clusterRepo *stevev1.SteveAPIObject
	err := wait.Poll(PollInterval, PollTimeout, func() (done bool, err error) {
		clusterRepo, err = c.client.Steve.SteveType(catalog.ClusterRepoSteveResourceType).ByID(ClusterRepoName)
		if err != nil {
			return false, err
		}
		status := c.getStatusFromClusterRepo(clusterRepo)
		if clusterRepo.Name != ClusterRepoName {
			return false, nil
		}

		return status.DownloadTime != prevDownloadTime, nil
	})

	return clusterRepo, err
}

func (c *ClusterRepoTestSuite) getSpecFromClusterRepo(obj *stevev1.SteveAPIObject) *v1.RepoSpec {
	spec := &v1.RepoSpec{}
	err := stevev1.ConvertToK8sType(obj.Spec, spec)
	require.NoError(c.T(), err)

	return spec
}

func (c *ClusterRepoTestSuite) getStatusFromClusterRepo(obj *stevev1.SteveAPIObject) *v1.RepoStatus {
	status := &v1.RepoStatus{}
	err := stevev1.ConvertToK8sType(obj.Status, status)
	require.NoError(c.T(), err)

	return status
}

func getCurrentRepoDirSmallFork() (string, error) {
	directories, err := os.ReadDir(ChartSmallForkDir)
	if err != nil {
		return "", fmt.Errorf("failed to find local git repository directory: %w", err)
	}
	// Join the target directory with the parent directory
	targetPath := filepath.Join(ChartSmallForkDir, directories[0].Name())

	return targetPath, nil
}

func getLocalRepoCurrentCommitAndBranch(repoPath string) (string, string, error) {
	// Get commit hash
	var commitOut bytes.Buffer
	commitCmd := exec.Command("git", "-C", repoPath, "rev-parse", "HEAD")
	commitCmd.Stdout = &commitOut
	err := commitCmd.Run()
	if err != nil {
		return "", "", fmt.Errorf("failed to get local repository commit: %v", err)
	}

	currentHeadCommit := commitOut.String()

	// Get branch name
	var branchOut bytes.Buffer
	branchCmd := exec.Command("git", "-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	branchCmd.Stdout = &branchOut
	err = branchCmd.Run()
	if err != nil {
		return "", "", fmt.Errorf("failed to get local repository branch: %v", err)
	}

	currentBranch := branchOut.String()
	return strings.TrimSpace(currentHeadCommit), strings.TrimSpace(currentBranch), nil
}

func setClusterRepoURL(spec *v1.RepoSpec, repoType RepoType, URL string) {
	switch repoType {
	case Git:
		spec.GitRepo = URL
	case HTTP:
		spec.URL = URL
	}
}
