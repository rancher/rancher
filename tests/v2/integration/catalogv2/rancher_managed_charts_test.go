package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	rv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/clients/rancher/catalog"
	client "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/kubeconfig"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const rancherLocalDir = "../rancher-data/local-catalogs/v2"
const smallForkURL = "https://github.com/rancher/charts-small-fork"
const smallForkClusterRepoName = "rancher-charts-small-fork"

var propagation = metav1.DeletePropagationForeground

type RancherManagedChartsTest struct {
	suite.Suite
	client           *rancher.Client
	session          *session.Session
	restClientGetter genericclioptions.RESTClientGetter
	catalogClient    *catalog.Client
	cluster          *client.Cluster
	corev1           corev1.CoreV1Interface
	originalBranch   string
	originalGitRepo  string
}

func (w *RancherManagedChartsTest) TearDownSuite() {
	w.session.Cleanup()
}

func (w *RancherManagedChartsTest) SetupSuite() {
	var err error
	testSession := session.NewSession()
	w.session = testSession
	w.client, err = rancher.NewClient("", testSession)
	require.NoError(w.T(), err)
	insecure := true
	w.client.RancherConfig.Insecure = &insecure
	w.catalogClient, err = w.client.GetClusterCatalogClient("local")
	require.NoError(w.T(), err)

	kubeConfig, err := kubeconfig.GetKubeconfig(w.client, "local")
	require.NoError(w.T(), err)

	restConfig, err := (*kubeConfig).ClientConfig()
	require.NoError(w.T(), err)
	//restConfig.Insecure = true
	cset, err := kubernetes.NewForConfig(restConfig)
	require.NoError(w.T(), err)
	w.corev1 = cset.CoreV1()

	w.restClientGetter, err = kubeconfig.NewRestGetter(restConfig, *kubeConfig)
	require.NoError(w.T(), err)
	c, err := w.client.Management.Cluster.ByID("local")
	require.NoError(w.T(), err)
	w.cluster = c
}

func TestRancherManagedChartsSuite(t *testing.T) {
	suite.Run(t, new(RancherManagedChartsTest))
}

func (w *RancherManagedChartsTest) updateManagementCluster() error {
	w.cluster.AKSConfig = &client.AKSClusterConfigSpec{}
	c, err := w.client.Management.Cluster.Replace(w.cluster)
	w.cluster = c
	return err
}

// pollUntilDownloaded Polls until the ClusterRepo of the given name has been downloaded (by comparing prevDownloadTime against the current DownloadTime)
func (w *RancherManagedChartsTest) pollUntilDownloaded(ClusterRepoName string, prevDownloadTime metav1.Time) error {
	err := kwait.Poll(PollInterval, time.Minute, func() (done bool, err error) {
		clusterRepo, err := w.catalogClient.ClusterRepos().Get(context.TODO(), ClusterRepoName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		w.Require().NoError(err)
		if clusterRepo.Name != ClusterRepoName {
			return false, nil
		}

		return clusterRepo.Status.DownloadTime != prevDownloadTime, nil
	})
	return err
}

func (w *RancherManagedChartsTest) TestServeIcons() {
	// Testing: Chart.icon field with (https:// scheme)
	// https://RANCHER_DOMAIN:8443/v1/catalog.cattle.io.clusterrepos/rancher-charts?link=index
	charts, err := w.catalogClient.GetChartsFromClusterRepo("rancher-charts")
	w.Require().NoError(err)
	w.Assert().Greater(len(charts), 1)

	chartsAndLatestVersions := extractChartsAndLatestVersions(charts)

	// https://RANCHER_DOMAIN:8443/v1/catalog.cattle.io.clusterrepos/rancher-charts?chartName=<SOME_CHART>&link=icon&version=<SOME_VERSION>
	imgLength, err := w.catalogClient.FetchChartIcon("rancher-charts", "rancher-istio", chartsAndLatestVersions["rancher-istio"])
	w.Require().NoError(err)
	w.Assert().Greater(imgLength, 0)
	imgLength, err = w.catalogClient.FetchChartIcon("rancher-charts", "rancher-project-monitoring", chartsAndLatestVersions["rancher-project-monitoring"])
	w.Require().NoError(err)
	w.Assert().Greater(imgLength, 0)
	imgLength, err = w.catalogClient.FetchChartIcon("rancher-charts", "longhorn", chartsAndLatestVersions["longhorn"])
	w.Require().NoError(err)
	w.Assert().Greater(imgLength, 0)
	imgLength, err = w.catalogClient.FetchChartIcon("rancher-charts", "rancher-monitoring", chartsAndLatestVersions["rancher-monitoring"])
	w.Require().NoError(err)
	w.Assert().Greater(imgLength, 0)
	imgLength, err = w.catalogClient.FetchChartIcon("rancher-charts", "prometheus-federator", chartsAndLatestVersions["prometheus-federator"])
	w.Require().NoError(err)
	w.Assert().Greater(imgLength, 0)

	// Testing: Chart.icon field with (file:// scheme)
	// Create ClusterRepo for charts-small-fork
	clusterRepoToCreate := rv1.NewClusterRepo("", smallForkClusterRepoName,
		rv1.ClusterRepo{
			Spec: rv1.RepoSpec{
				GitRepo:   smallForkURL,
				GitBranch: "main",
			},
		},
	)
	_, err = w.client.Steve.SteveType(catalog.ClusterRepoSteveResourceType).Create(clusterRepoToCreate)
	w.Require().NoError(err)
	time.Sleep(1 * time.Second)

	w.Require().NoError(w.pollUntilDownloaded(smallForkClusterRepoName, metav1.Time{}))

	// Get Charts from the ClusterRepo
	smallForkCharts, err := w.catalogClient.GetChartsFromClusterRepo(smallForkClusterRepoName)
	w.Require().NoError(err)
	w.Assert().Greater(len(smallForkCharts), 1)

	// Get the client settings to update settings.SystemCatalog
	systemCatalog, err := w.client.Management.Setting.ByID("system-catalog")
	w.Require().NoError(err)
	w.Assert().Equal("external", systemCatalog.Value)

	// Update settings.SystemCatalog to bundled
	systemCatalogUpdated, err := w.client.Management.Setting.Update(systemCatalog, map[string]interface{}{"value": "bundled"})
	w.Require().NoError(err)
	w.Assert().Equal("bundled", systemCatalogUpdated.Value)

	// Fetch one icon with https:// scheme, it should return an empty object (i.e length of image equals 0) with nil error
	imgLength, err = w.catalogClient.FetchChartIcon(smallForkClusterRepoName, "fleet", "102.0.0+up0.6.0")
	w.Require().NoError(err)
	w.Assert().Equal(0, imgLength)

	// Update settings.SystemCatalog to external
	_, err = w.client.Management.Setting.Update(systemCatalog, map[string]interface{}{"value": "external"})
	w.Require().NoError(err)

	// Deleting clusterRepo
	err = w.catalogClient.ClusterRepos().Delete(context.Background(), smallForkClusterRepoName, metav1.DeleteOptions{})
	w.Require().NoError(err)
}

// extractChartsAndLatestVersions returns a map of chartName -> latestVersion
func extractChartsAndLatestVersions(charts map[string]interface{}) map[string]string {
	chartVersions := make(map[string]string)
	for chartName, chartVersionsList := range charts {
		// exclude charts for crd's
		if strings.HasSuffix(chartName, "-crd") {
			continue
		}
		chartVersionsList := chartVersionsList.([]interface{})
		// exclude charts with the hidden annotation
		_, hidden := chartVersionsList[0].(map[string]interface{})["annotations"].(map[string]interface{})["catalog.cattle.io/hidden"]
		if hidden {
			continue
		}
		chartVersions[chartName] = chartVersionsList[0].(map[string]interface{})["version"].(string)
	}
	return chartVersions
}
