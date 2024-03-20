package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/catalogv2/oci"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/catalog"
	stevev1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/pkg/api/steve/catalog/types"
	"github.com/rancher/shepherd/pkg/session"
	rancherWait "github.com/rancher/shepherd/pkg/wait"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	helmregistry "helm.sh/helm/v3/pkg/registry"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
)

const (
	HTTPClusterRepoName = "test-http-cluster-repo"
	LatestHTTPRepoURL   = "https://releases.rancher.com/server-charts/latest"
	StableHTTPRepoURL   = "https://releases.rancher.com/server-charts/stable"

	GitClusterRepoName      = "test-git-cluster-repo"
	RancherChartsGitRepoURL = "https://git.rancher.io/charts"
	RKE2ChartsGitRepoURL    = "https://git.rancher.io/rke2-charts"

	OCIClusterRepoName = "test-oci-cluster-repo"
)

var (
	PollInterval = time.Duration(500 * time.Millisecond)
	PollTimeout  = time.Duration(5 * time.Minute)
)

type ClusterRepoTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
}

func (c *ClusterRepoTestSuite) TearDownSuite() {
	c.session.Cleanup()
}

func (c *ClusterRepoTestSuite) SetupSuite() {
	testSession := session.NewSession()
	c.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(c.T(), err)
	c.client = client
}

type RepoType int64

const (
	Git RepoType = iota
	HTTP
	OCI
)

// ClusterRepoParams is used to pass params to func testClusterRepo for testing
type ClusterRepoParams struct {
	Name              string   // Name of the ClusterRepo resource
	Type              RepoType // Type of the ClusterRepo resource
	URL1              string   // URL to use when creating the ClusterRepo resource
	URL2              string   // URL to use when updating the ClusterRepo resource to a new URL
	InsecurePlainHTTP bool
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

// TestGitRepo tests CREATE, UPDATE, and DELETE operations of Git ClusterRepo resources
func (c *ClusterRepoTestSuite) TestGitRepo() {
	c.testClusterRepo(ClusterRepoParams{
		Name: GitClusterRepoName,
		URL1: RancherChartsGitRepoURL,
		URL2: RKE2ChartsGitRepoURL,
		Type: Git,
	})
}

func StartRegistry() (*url.URL, error) {
	s := httptest.NewServer(registry.New())
	u, err := url.Parse(s.URL)
	if err != nil {
		return nil, err
	}

	return u, nil
}

func AddHelmChart(u *url.URL) error {
	chartTar, err := os.ReadFile("../../../testdata/testingchart-0.1.0.tgz")
	if err != nil {
		return err
	}

	configBlob := []byte("config")
	configDesc := ocispec.Descriptor{
		MediaType: helmregistry.ConfigMediaType,
		Digest:    digest.FromBytes(configBlob),
		Size:      int64(len(configBlob)),
	}
	layerDesc := ocispec.Descriptor{
		MediaType: helmregistry.ChartLayerMediaType,
		Digest:    digest.FromBytes(chartTar),
		Size:      int64(len(chartTar)),
	}
	manifest := ocispec.Manifest{
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    configDesc,
		Layers:    []ocispec.Descriptor{layerDesc},
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return err
	}

	manifestDesc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageManifest,
		Digest:    digest.FromBytes(manifestJSON),
		Size:      int64(len(manifestJSON)),
	}

	target := memory.New()
	target.Push(context.Background(), configDesc, bytes.NewReader(configBlob))
	target.Push(context.Background(), layerDesc, bytes.NewReader(chartTar))
	target.Push(context.Background(), manifestDesc, bytes.NewReader(manifestJSON))
	err = target.Tag(context.Background(), manifestDesc, "0.1.0")
	if err != nil {
		return err
	}

	ociClient, err := oci.NewClient(fmt.Sprintf("oci://%s/rancher/testingchart", u.Host), v1.RepoSpec{}, nil)
	if err != nil {
		return err
	}

	orasRepository, err := ociClient.GetOrasRepository()
	if err != nil {
		return err
	}
	orasRepository.PlainHTTP = true

	_, err = oras.Copy(context.Background(), target, "0.1.0", orasRepository, "", oras.DefaultCopyOptions)
	if err != nil {
		return err
	}

	return nil
}

// TestOCIRepo tests CREATE, UPDATE, and DELETE operations of OCI ClusterRepo resources
func (c *ClusterRepoTestSuite) TestOCIRepo() {
	//start registry
	u, err := StartRegistry()
	assert.NoError(c.T(), err)

	//push testingchart helm chart
	err = AddHelmChart(u)
	assert.NoError(c.T(), err)

	c.testClusterRepo(ClusterRepoParams{
		Name:              OCIClusterRepoName,
		URL1:              fmt.Sprintf("oci://%s/rancher/testingchart", u.Host),
		URL2:              fmt.Sprintf("oci://%s/rancher/testingchart:0.1.0", u.Host),
		Type:              OCI,
		InsecurePlainHTTP: true,
	})
}

// TestOCIRepo tests CREATE, UPDATE, and DELETE operations of OCI ClusterRepo additional cases
func (c *ClusterRepoTestSuite) TestOCIRepo2() {
	//start registry
	u, err := StartRegistry()
	assert.NoError(c.T(), err)

	//push testingchart helm chart
	err = AddHelmChart(u)
	assert.NoError(c.T(), err)

	c.testClusterRepo(ClusterRepoParams{
		Name:              OCIClusterRepoName,
		URL1:              fmt.Sprintf("oci://%s/rancher", u.Host),
		URL2:              fmt.Sprintf("oci://%s/", u.Host),
		Type:              OCI,
		InsecurePlainHTTP: true,
	})
}

// TestOCI tests creating a OCI clusterrepo and install a chart
func (c *ClusterRepoTestSuite) TestOCIRepoChartInstallation() {
	//start registry
	u, err := StartRegistry()
	assert.NoError(c.T(), err)

	//push testingchart helm chart
	err = AddHelmChart(u)
	assert.NoError(c.T(), err)

	repoName := "oci"

	// create cluster repo
	catalogClient, err := c.client.GetClusterCatalogClient("local")
	assert.NoError(c.T(), err)

	clusterRepo := &v1.ClusterRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name: repoName,
		},
		Spec: v1.RepoSpec{
			URL:               fmt.Sprintf("oci://%s/rancher", u.Host),
			InsecurePlainHTTP: true,
		},
	}
	_, err = catalogClient.ClusterRepos().Create(context.Background(), clusterRepo, metav1.CreateOptions{})
	assert.NoError(c.T(), err)

	// Validate the ClusterRepo was created
	_, err = c.pollUntilDownloaded(repoName, metav1.Time{})
	require.NoError(c.T(), err)

	// check if chart can be fetched
	chartInstallAction := types.ChartInstallAction{
		DisableHooks: false,
		Timeout:      nil,
	}

	chartInstallAction.Charts = []types.ChartInstall{
		{
			ChartName:   "testingchart",
			Version:     "0.1.0",
			ReleaseName: "testreleasename",
		},
	}

	err = catalogClient.InstallChart(&chartInstallAction, repoName)
	assert.NoError(c.T(), err)

	// wait for chart to be full deployed
	watchAppInterface, err := catalogClient.Apps("default").Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + "testreleasename",
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	assert.NoError(c.T(), err)

	err = rancherWait.WatchWait(watchAppInterface, func(event watch.Event) (ready bool, err error) {
		app := event.Object.(*v1.App)

		state := app.Status.Summary.State
		if state == string(v1.StatusDeployed) {
			return true, nil
		}
		return false, nil
	})
	assert.NoError(c.T(), err)

	// Validate uninstalling the chart
	chartUninstallAction := types.ChartUninstallAction{
		DisableHooks: false,
		Timeout:      nil,
	}

	err = catalogClient.UninstallChart("testreleasename", "default", &chartUninstallAction)
	assert.NoError(c.T(), err)

	watchAppInterface, err = catalogClient.Apps("default").Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + "testreleasename",
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	assert.NoError(c.T(), err)

	err = rancherWait.WatchWait(watchAppInterface, func(event watch.Event) (ready bool, err error) {
		if event.Type == watch.Deleted {
			return true, nil
		}
		return false, nil
	})
	assert.NoError(c.T(), err)

	// Validate deleting the ClusterRepo
	err = catalogClient.ClusterRepos().Delete(context.Background(), "oci", metav1.DeleteOptions{})
	assert.NoError(c.T(), err)

	err = catalogClient.ClusterRepos().Delete(context.Background(), "oci", metav1.DeleteOptions{})
	assert.Error(c.T(), err)
}

// testClusterRepo takes in ClusterRepoParams and tests CREATE, UPDATE, and DELETE operations
func (c *ClusterRepoTestSuite) testClusterRepo(params ClusterRepoParams) {
	// Create a ClusterRepo
	cr := v1.NewClusterRepo("", params.Name, v1.ClusterRepo{})
	setClusterRepoURL(&cr.Spec, params.Type, params.URL1)
	cr.Spec.InsecurePlainHTTP = params.InsecurePlainHTTP
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

func setClusterRepoURL(spec *v1.RepoSpec, repoType RepoType, URL string) {
	switch repoType {
	case Git:
		spec.GitRepo = URL
	case HTTP:
		spec.URL = URL
	case OCI:
		spec.URL = URL
	}
}

func TestClusterRepoTestSuite(t *testing.T) {
	suite.Run(t, new(ClusterRepoTestSuite))
}
