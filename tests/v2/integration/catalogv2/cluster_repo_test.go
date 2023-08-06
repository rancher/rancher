package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/clients/rancher/catalog"
	stevev1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	clusters "github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	clusterWait "github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	"github.com/rancher/wrangler/pkg/data"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	LocalClusterID                    = "local"
	ChartsSmallForkRepoName           = "charts-small-fork"
	ChartsSmallForkGitRepoURL         = "https://github.com/rancher/charts-small-fork"
	ChartsSmallForkGitRepoFirstBranch = "test-1"
	ChartsSmallForkGitRepoLastBranch  = "main"

	GitClusterRepoName      = "test-git-cluster-repo"
	RancherChartsGitRepoURL = "https://git.rancher.io/charts"
	RKE2ChartsGitRepoURL    = "https://git.rancher.io/rke2-charts"

	HTTPClusterRepoName = "test-http-cluster-repo"
	LatestHTTPRepoURL   = "https://releases.rancher.com/server-charts/latest"
	StableHTTPRepoURL   = "https://releases.rancher.com/server-charts/stable"
)

var (
	CICD              = false
	ChartSmallForkDir = ""
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

// ClusterRepoParams is used to pass params to func testClusterRepo for testing
type ChartsSmallForkRepoParams struct {
	Name    string   // Name of the ClusterRepo resource
	Type    RepoType // Type of the ClusterRepo resource
	URL     string
	Branch1 string // URL to use when creating the ClusterRepo resource
	Branch2 string // URL to use when updating the ClusterRepo resource to a new URL
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

	if os.Getenv("LOCAL_MODE") == LocalClusterID {
		CICD = false
		c.clusterID = LocalClusterID
		ChartSmallForkDir = fmt.Sprintf("../../../../management-state/git-repo/%s", ChartsSmallForkRepoName)
	} else {
		CICD = true
		clusterName := c.client.RancherConfig.ClusterName
		c.clusterID, err = clusters.GetClusterIDByName(c.client, clusterName)
		require.NoError(c.T(), err)
		ChartSmallForkDir = fmt.Sprintf("/go/src/github.com/rancher/rancher/bin/build/%s", ChartsSmallForkRepoName)
	}

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

	testClusterRepo, err := c.catalogClient.ClusterRepos().Create(c.ctx,
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

	installedClusterRepos, err := c.catalogClient.ClusterRepos().List(c.ctx, metav1.ListOptions{})
	require.NoError(c.T(), err)

	success := false
	for _, cr := range installedClusterRepos.Items {
		logrus.Infof("Installed Cluster Repo: %s", cr.Name)
		if cr.Name == testClusterRepo.Name {
			success = true
		}
	}
	require.Equal(c.T(), true, success)

	watcherEnsure, err := c.catalogClient.ClusterRepos().Watch(c.ctx, metav1.ListOptions{
		FieldSelector:  fmt.Sprintf("metadata.name=%s", ChartsSmallForkRepoName),
		TimeoutSeconds: &defaults.QuickWatchTimeoutSeconds,
	})
	require.NoError(c.T(), err)

	err = clusterWait.WatchWait(watcherEnsure, func(event watch.Event) (ensured bool, err error) {
		if event.Type == watch.Error {
			return false, fmt.Errorf("there was an error syncing the cluster repo charts-small-fork")
		}
		if event.Type == watch.Modified {
			return checkStatusFromClusterRepo(event)
		}
		return false, nil
	})
	require.NoError(c.T(), err)

	repoPath, err := getCurrentRepoDirSmallFork()
	require.NoError(c.T(), err)
	localRepoCommit, localRepoBranch, err := getLocalRepoCurrentCommitAndBranch(repoPath)
	require.NoError(c.T(), err)

	testClusterRepo, err = c.catalogClient.ClusterRepos().Get(c.ctx, testClusterRepo.Name, metav1.GetOptions{})
	require.NoError(c.T(), err)
	assert.Equal(c.T(), localRepoBranch, testClusterRepo.Spec.GitBranch)
	assert.Equal(c.T(), localRepoBranch, testClusterRepo.Status.Branch)
	assert.Equal(c.T(), localRepoCommit, testClusterRepo.Status.Commit)
	assert.Equal(c.T(), int64(1), testClusterRepo.Status.ObservedGeneration)

	testClusterRepo.Spec.GitBranch = ChartsSmallForkGitRepoLastBranch
	updatedClusterRepo, err := c.catalogClient.ClusterRepos().Update(c.ctx, testClusterRepo.DeepCopy(), metav1.UpdateOptions{})
	require.NoError(c.T(), err)
	assert.Equal(c.T(), ChartsSmallForkGitRepoLastBranch, updatedClusterRepo.Spec.GitBranch)

	watcherUpdate, err := c.catalogClient.ClusterRepos().Watch(c.ctx, metav1.ListOptions{
		FieldSelector:  fmt.Sprintf("metadata.name=%s", ChartsSmallForkRepoName),
		TimeoutSeconds: &defaults.QuickWatchTimeoutSeconds,
	})
	require.NoError(c.T(), err)

	err = clusterWait.WatchWait(watcherUpdate, func(event watch.Event) (ensured bool, err error) {
		if event.Type == watch.Error {
			return false, fmt.Errorf("there was an error syncing the cluster repo charts-small-fork")
		}
		if event.Type == watch.Modified {
			return checkObservedGeneration(event)
		}
		return false, nil
	})
	require.NoError(c.T(), err)

	updatedRepoCommit, updatedRepoBranch, err := getLocalRepoCurrentCommitAndBranch(repoPath)
	require.NoError(c.T(), err)

	updatedClusterRepo, err = c.catalogClient.ClusterRepos().Get(c.ctx, testClusterRepo.Name, metav1.GetOptions{})
	require.NoError(c.T(), err)
	assert.Equal(c.T(), updatedRepoBranch, updatedClusterRepo.Spec.GitBranch)
	assert.Equal(c.T(), updatedRepoBranch, updatedClusterRepo.Status.Branch)
	assert.Equal(c.T(), updatedRepoCommit, updatedClusterRepo.Status.Commit)
	assert.Equal(c.T(), int64(2), updatedClusterRepo.Status.ObservedGeneration)
}

func checkObservedGeneration(event watch.Event) (bool, error) {
	crObj, err := data.Convert(event.Object.DeepCopyObject())
	if err != nil {
		return false, err
	}

	status := crObj.Map("status")
	observed := status["observedGeneration"]

	updated := false
	observed, ok := observed.(interface{})
	if !ok {
		return false, fmt.Errorf("observed is not the expected type")
	}

	if obsNum, ok := observed.(json.Number); obsNum.String() == "2" && ok {
		updated = true
	}

	return updated, nil
}

func checkStatusFromClusterRepo(event watch.Event) (bool, error) {
	crObj, err := data.Convert(event.Object.DeepCopyObject())
	if err != nil {
		return false, err
	}

	status := crObj.Map("status")
	conditions := status["conditions"]

	ensured := false
	conditionsSlice, ok := conditions.([]interface{})
	if !ok {
		return false, fmt.Errorf("conditions is not the expected type")
	}

	if len(conditionsSlice) > 1 {
		for _, conditionsInterface := range conditionsSlice {
			conditionMap, ok := conditionsInterface.(map[string]interface{})
			if !ok {
				return false, fmt.Errorf("type assertion failed for conditions")
			}
			status, ok := conditionMap["status"].(string)
			if !ok {
				return false, fmt.Errorf("type assertion failed for conditions")
			}
			conditionType, ok := conditionMap["type"].(string)
			if !ok {
				return false, fmt.Errorf("type assertion failed for conditions")
			}
			ensured = status == "True"
			ensured = ensured && (conditionType == "FollowerDownloaded" || conditionType == "Downloaded")
		}
	}

	return ensured, nil
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
	if CICD {
		return ChartSmallForkDir, nil
	}
	// Read the directory
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
