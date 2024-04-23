package integration

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	registryGoogle "github.com/google/go-containerregistry/pkg/registry"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/catalogv2/oci"
	"github.com/rancher/rancher/pkg/controllers/dashboard/helm"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/catalog"
	stevev1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/defaults/stevetypes"
	"github.com/rancher/shepherd/extensions/defaults/timeouts"
	"github.com/rancher/shepherd/extensions/kubeconfig"
	"github.com/rancher/shepherd/pkg/api/steve/catalog/types"
	"github.com/rancher/shepherd/pkg/session"
	rancherWait "github.com/rancher/shepherd/pkg/wait"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/repo"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
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
	client        *rancher.Client
	session       *session.Session
	catalogClient *catalog.Client
	corev1        corev1client.CoreV1Interface
}

func (c *ClusterRepoTestSuite) TearDownSuite() {
	c.session.Cleanup()
}

func (c *ClusterRepoTestSuite) SetupSuite() {
	var err error
	testSession := session.NewSession()
	c.session = testSession

	c.client, err = rancher.NewClient("", testSession)
	require.NoError(c.T(), err)
	insecure := true
	c.client.RancherConfig.Insecure = &insecure
	c.catalogClient, err = c.client.GetClusterCatalogClient("local")
	require.NoError(c.T(), err)

	kubeConfig, err := kubeconfig.GetKubeconfig(c.client, "local")
	require.NoError(c.T(), err)
	restConfig, err := (*kubeConfig).ClientConfig()
	require.NoError(c.T(), err)
	cset, err := kubernetes.NewForConfig(restConfig)
	require.NoError(c.T(), err)
	c.corev1 = cset.CoreV1()
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
	StatusCode        int
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
	s := httptest.NewServer(registryGoogle.New())
	u, err := url.Parse(s.URL)
	if err != nil {
		return nil, err
	}

	return u, nil
}

func StartErrorRegistry(status int) (*url.URL, error) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	}))

	u, err := url.Parse(ts.URL)
	if err != nil {
		return nil, err
	}

	return u, nil
}

func Start429Registry(t assert.TestingT) (*url.URL, error) {
	testingChartPath := "../../../testdata/testingchart-0.1.0.tgz"
	testChartPath := "../../../testdata/testchart-1.0.0.tgz"
	helmChartTar, err := os.ReadFile(testingChartPath)
	assert.NoError(t, err)

	layerDesc := ocispec.Descriptor{
		MediaType: registry.ChartLayerMediaType,
		Digest:    digest.FromBytes(helmChartTar),
		Size:      int64(len(helmChartTar)),
	}

	helmChartTar2, err := os.ReadFile(testChartPath)
	assert.NoError(t, err)

	layerDesc2 := ocispec.Descriptor{
		MediaType: registry.ChartLayerMediaType,
		Digest:    digest.FromBytes(helmChartTar2),
		Size:      int64(len(helmChartTar2)),
	}

	configBlob := []byte("config")
	configDesc := ocispec.Descriptor{
		MediaType: registry.ConfigMediaType,
		Digest:    digest.FromBytes(configBlob),
		Size:      int64(len(configBlob)),
	}

	manifest := ocispec.Manifest{
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    configDesc,
		Layers:    []ocispec.Descriptor{layerDesc},
	}
	manifestJSON, err := json.Marshal(manifest)
	assert.NoError(t, err)
	manifestDesc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageManifest,
		Digest:    digest.FromBytes(manifestJSON),
		Size:      int64(len(manifestJSON)),
	}

	manifest2 := ocispec.Manifest{
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    configDesc,
		Layers:    []ocispec.Descriptor{layerDesc2},
	}
	manifestJSON2, err := json.Marshal(manifest2)
	assert.NoError(t, err)
	manifestDesc2 := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageManifest,
		Digest:    digest.FromBytes(manifestJSON2),
		Size:      int64(len(manifestJSON2)),
	}

	manifestCount := 1
	timerStart := false

	// Create a OCI Registry Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		switch r.URL.Path {

		case "/v2/testingchart/tags/list":
			t := `{"tags": ["0.1.0","0.0.1","sha256"]}`
			w.Write([]byte(t))
		case "/v2/testchart/tags/list":
			t := `{"tags": ["1.0.0","0.1.1","sha256"]}`
			w.Write([]byte(t))

		case "/v2/_catalog":
			t := `{"repositories": ["testingchart","testchart"]}`
			w.Write([]byte(t))

		case "/v2/testingchart/blobs/" + layerDesc.Digest.String():
			http.ServeFile(w, r, testingChartPath)
		case "/v2/testchart/blobs/" + layerDesc2.Digest.String():
			http.ServeFile(w, r, testChartPath)
		case "/v2/testingchart/manifests/0.1.0":
			if accept := r.Header.Get("Accept"); !strings.Contains(accept, manifestDesc.MediaType) {
				assert.NoError(t, err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", manifestDesc.MediaType)
			w.Header().Set("Docker-Content-Digest", manifestDesc.Digest.String())
			if _, err := w.Write(manifestJSON); err != nil {
				assert.NoError(t, err)
			}
		case "/v2/testchart/manifests/1.0.0":
			manifestCount++
			if manifestCount > 1 {
				if !timerStart {
					go func() {
						d := time.NewTicker(1 * time.Minute)
						for {
							<-d.C
							manifestCount = 0
							return
						}
					}()
					timerStart = true
				}
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			if accept := r.Header.Get("Accept"); !strings.Contains(accept, manifestDesc2.MediaType) {
				assert.NoError(t, err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", manifestDesc2.MediaType)
			w.Header().Set("Docker-Content-Digest", manifestDesc2.Digest.String())
			if _, err := w.Write(manifestJSON2); err != nil {
				assert.NoError(t, err)
			}

		}
	}))

	u, err := url.Parse(ts.URL)
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
		MediaType: registry.ConfigMediaType,
		Digest:    digest.FromBytes(configBlob),
		Size:      int64(len(configBlob)),
	}
	layerDesc := ocispec.Descriptor{
		MediaType: registry.ChartLayerMediaType,
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

// TestOCIRepo2 tests CREATE, UPDATE, and DELETE operations of OCI ClusterRepo additional cases
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

// TestOCIRepo3 tests 4xx response codes recieved from the registry
func (c *ClusterRepoTestSuite) TestOCIRepo3() {
	statusCodes := [3]int{404, 401, 403}
	for _, statusCode := range statusCodes {
		u, err := StartErrorRegistry(statusCode)
		assert.NoError(c.T(), err)
		c.test4xxErrors(ClusterRepoParams{
			Name:              OCIClusterRepoName,
			URL1:              fmt.Sprintf("oci://%s/rancher", u.Host),
			StatusCode:        statusCode,
			InsecurePlainHTTP: true,
			Type:              OCI,
		})
	}
}

// TestOCIRepo4 tests 429 response code recieved from the registry
func (c *ClusterRepoTestSuite) TestOCIRepo4() {
	u, err := Start429Registry(c.T())
	assert.NoError(c.T(), err)
	c.test429Error(ClusterRepoParams{
		Name:              OCIClusterRepoName,
		URL1:              fmt.Sprintf("oci://%s/", u.Host),
		InsecurePlainHTTP: true,
		Type:              OCI,
	})
}

func (c *ClusterRepoTestSuite) test429Error(params ClusterRepoParams) {
	var err error

	clusterRepo := v1.NewClusterRepo("", params.Name, v1.ClusterRepo{})
	setClusterRepoURL(&clusterRepo.Spec, params.Type, params.URL1)
	clusterRepo.Spec.InsecurePlainHTTP = params.InsecurePlainHTTP
	expoValues := v1.ExponentialBackOffValues{
		MinWait:    "1s",
		MaxWait:    "1s",
		MaxRetries: 1,
	}
	clusterRepo.Spec.ExponentialBackOffValues = &expoValues
	clusterRepo, err = c.catalogClient.ClusterRepos().Create(context.TODO(), clusterRepo, metav1.CreateOptions{})
	assert.NoError(c.T(), err)

	err = wait.Poll(50*time.Millisecond, 5*time.Minute, func() (done bool, err error) {
		clusterRepo, err = c.catalogClient.ClusterRepos().Get(context.TODO(), params.Name, metav1.GetOptions{})
		assert.NoError(c.T(), err)

		for _, condition := range clusterRepo.Status.Conditions {
			if v1.RepoCondition(condition.Type) == v1.OCIDownloaded {
				return condition.Status == corev1.ConditionFalse, nil
			}
		}

		return false, nil
	})
	assert.NoError(c.T(), err)

	configMap, err := c.corev1.ConfigMaps(helm.GetConfigMapNamespace(clusterRepo.Namespace)).Get(context.TODO(), helm.GenerateConfigMapName(clusterRepo.Name, 0, clusterRepo.UID), metav1.GetOptions{})
	assert.NoError(c.T(), err)

	data := configMap.BinaryData["content"]
	gz, err := gzip.NewReader(bytes.NewBuffer(data))
	assert.NoError(c.T(), err)
	defer gz.Close()
	data, err = io.ReadAll(gz)
	assert.NoError(c.T(), err)
	index := &repo.IndexFile{}
	err = json.Unmarshal(data, index)
	assert.NoError(c.T(), err)

	index.SortEntries()
	assert.Equal(c.T(), len(index.Entries), 2)
	assert.Equal(c.T(), len(index.Entries["testingchart"]), 2)
	assert.NotEmpty(c.T(), index.Entries["testingchart"][0].Digest)
	time.Sleep(1 * time.Minute)

	// Refresh the clusterRepo
	clusterRepo, err = c.catalogClient.ClusterRepos().Get(context.TODO(), params.Name, metav1.GetOptions{})
	assert.NoError(c.T(), err)
	clusterRepo.Spec.ForceUpdate = &metav1.Time{Time: time.Now()}
	clusterRepo, err = c.catalogClient.ClusterRepos().Update(context.TODO(), clusterRepo.DeepCopy(), metav1.UpdateOptions{})
	assert.NoError(c.T(), err)

	err = wait.Poll(PollInterval, 5*time.Second, func() (done bool, err error) {
		clusterRepo, err = c.catalogClient.ClusterRepos().Get(context.TODO(), params.Name, metav1.GetOptions{})
		assert.NoError(c.T(), err)

		for _, condition := range clusterRepo.Status.Conditions {
			if v1.RepoCondition(condition.Type) == v1.OCIDownloaded {
				return condition.Status == corev1.ConditionTrue, nil
			}
		}

		return false, nil
	})

	clusterRepo, err = c.catalogClient.ClusterRepos().Get(context.TODO(), params.Name, metav1.GetOptions{})
	assert.NoError(c.T(), err)
	configMap, err = c.corev1.ConfigMaps(clusterRepo.Status.IndexConfigMapNamespace).Get(context.TODO(), clusterRepo.Status.IndexConfigMapName, metav1.GetOptions{})
	assert.NoError(c.T(), err)

	data = configMap.BinaryData["content"]
	gz, err = gzip.NewReader(bytes.NewBuffer(data))
	assert.NoError(c.T(), err)
	defer gz.Close()
	data, err = io.ReadAll(gz)
	assert.NoError(c.T(), err)
	index = &repo.IndexFile{}
	err = json.Unmarshal(data, index)
	assert.NoError(c.T(), err)

	assert.Equal(c.T(), len(index.Entries), 2)
	assert.Equal(c.T(), len(index.Entries["testchart"]), 2)
	assert.Equal(c.T(), len(index.Entries["testingchart"]), 2)
	assert.NotEmpty(c.T(), index.Entries["testchart"][0].Digest)
	assert.NotEmpty(c.T(), index.Entries["testingchart"][0].Digest)

	err = c.catalogClient.ClusterRepos().Delete(context.TODO(), params.Name, metav1.DeleteOptions{})
	assert.NoError(c.T(), err)

	clusterRepo, err = c.catalogClient.ClusterRepos().Get(context.TODO(), params.Name, metav1.GetOptions{})
	assert.Error(c.T(), err)
}

func (c *ClusterRepoTestSuite) test4xxErrors(params ClusterRepoParams) {
	// Create a ClusterRepo
	cr := v1.NewClusterRepo("", params.Name, v1.ClusterRepo{})
	setClusterRepoURL(&cr.Spec, params.Type, params.URL1)
	cr.Spec.InsecurePlainHTTP = params.InsecurePlainHTTP
	_, err := c.catalogClient.ClusterRepos().Create(context.TODO(), cr, metav1.CreateOptions{})
	assert.NoError(c.T(), err)

	err = wait.Poll(PollInterval, 5*time.Second, func() (done bool, err error) {
		clusterRepo, err := c.catalogClient.ClusterRepos().Get(context.TODO(), params.Name, metav1.GetOptions{})
		assert.NoError(c.T(), err)

		for _, condition := range clusterRepo.Status.Conditions {
			if v1.RepoCondition(condition.Type) == v1.OCIDownloaded {
				return condition.Status == corev1.ConditionFalse, nil
			}
		}

		return false, nil
	})

	clusterRepo, err := c.catalogClient.ClusterRepos().Get(context.TODO(), params.Name, metav1.GetOptions{})
	assert.NoError(c.T(), err)

	_, err = c.corev1.ConfigMaps(helm.GetConfigMapNamespace(clusterRepo.Namespace)).Get(context.TODO(), helm.GenerateConfigMapName(clusterRepo.Name, 0, clusterRepo.UID), metav1.GetOptions{})
	assert.True(c.T(), apierrors.IsNotFound(err))

	err = c.catalogClient.ClusterRepos().Delete(context.TODO(), params.Name, metav1.DeleteOptions{})
	assert.NoError(c.T(), err)

	_, err = c.catalogClient.ClusterRepos().Get(context.TODO(), params.Name, metav1.GetOptions{})
	assert.Error(c.T(), err)
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
		TimeoutSeconds: timeouts.TenMinuteWatch(),
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
		TimeoutSeconds: timeouts.TenMinuteWatch(),
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
	_, err := c.client.Steve.SteveType(stevetypes.ClusterRepo).Create(cr)
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

	_, err = c.client.Steve.SteveType(stevetypes.ClusterRepo).Replace(&clusterRepoUpdated)
	require.NoError(c.T(), err)

	clusterRepo, err = c.pollUntilDownloaded(params.Name, downloadTime)
	require.NoError(c.T(), err)

	status = c.getStatusFromClusterRepo(clusterRepo)
	assert.Equal(c.T(), params.URL2, status.URL)
	assert.Greater(c.T(), status.ObservedGeneration, observedGeneration)

	// Validate deleting the ClusterRepo
	err = c.client.Steve.SteveType(stevetypes.ClusterRepo).Delete(clusterRepo)
	require.NoError(c.T(), err)

	_, err = c.client.Steve.SteveType(stevetypes.ClusterRepo).ByID(params.Name)
	require.Error(c.T(), err)
}

// pollUntilDownloaded Polls until the ClusterRepo of the given name has been downloaded (by comparing prevDownloadTime against the current DownloadTime)
func (c *ClusterRepoTestSuite) pollUntilDownloaded(ClusterRepoName string, prevDownloadTime metav1.Time) (*stevev1.SteveAPIObject, error) {
	var clusterRepo *stevev1.SteveAPIObject
	err := wait.Poll(PollInterval, PollTimeout, func() (done bool, err error) {
		clusterRepo, err = c.client.Steve.SteveType(stevetypes.ClusterRepo).ByID(ClusterRepoName)
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
