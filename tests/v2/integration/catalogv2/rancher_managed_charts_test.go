package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	rv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/catalog"
	client "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	stevev1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/kubeconfig"
	"github.com/rancher/shepherd/pkg/api/steve/catalog/types"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const smallForkURL = "https://github.com/rancher/charts-small-fork"
const smallForkClusterRepoName = "rancher-charts-small-fork"

// aksOperatorVersionSetting is the setting holding the exact version of the AKS operator chart Rancher installs. It
// defaults to the version pinned in build.yaml, which does not exist in the charts of the small fork.
const aksOperatorVersionSetting = "aks-operator-version"

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
	w.Require().NoError(w.updateSetting("system-managed-charts-operation-timeout", "300s"))
	w.Require().NoError(w.updateSetting("system-feature-chart-refresh-seconds", "21600"))
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
	w.Require().NoError(w.updateSetting("system-managed-charts-operation-timeout", "50s"))
	w.Require().NoError(w.updateSetting("system-feature-chart-refresh-seconds", "21600"))
	clusterRepo, err := w.catalogClient.ClusterRepos().Get(context.TODO(), "rancher-charts", metav1.GetOptions{})
	w.Require().NoError(err)
	w.originalBranch = clusterRepo.Spec.GitBranch
	w.originalGitRepo = clusterRepo.Spec.GitRepo
	w.resetManagementCluster()
}

func (w *RancherManagedChartsTest) resetSettings() {
	// An empty value brings back the default, which is the version pinned in build.yaml.
	w.Require().NoError(w.updateSetting(aksOperatorVersionSetting, ""))
	w.resetManagementCluster()

	w.uninstallApp("cattle-system", "rancher-aks-operator-crd")
	w.uninstallApp("cattle-system", "rancher-aks-operator")
	clusterRepo, err := w.catalogClient.ClusterRepos().Get(context.TODO(), "rancher-charts", metav1.GetOptions{})
	w.Require().NoError(err)
	if clusterRepo.Spec.GitRepo != w.originalGitRepo {
		clusterRepo.Spec.GitRepo = w.originalGitRepo
		clusterRepo.Spec.GitBranch = w.originalBranch
		downloadTime := clusterRepo.Status.DownloadTime
		_, err = w.catalogClient.ClusterRepos().Update(context.TODO(), clusterRepo, metav1.UpdateOptions{})
		w.Require().NoError(err)
		w.Require().NoError(w.pollUntilDownloaded("rancher-charts", downloadTime))
	}
}

func TestRancherManagedChartsSuite(t *testing.T) {
	suite.Run(t, new(RancherManagedChartsTest))
}

func (w *RancherManagedChartsTest) TestInstallChartLatestVersion() {
	defer w.resetSettings()
	ctx := context.Background()

	clusterRepo, err := w.catalogClient.ClusterRepos().Get(ctx, "rancher-charts", metav1.GetOptions{})
	w.Require().NoError(err)
	clusterRepo.Spec.GitRepo = "https://github.com/rancher/charts-small-fork"
	clusterRepo.Spec.GitBranch = "aks-integration-test-working-charts"
	clusterRepo, err = w.catalogClient.ClusterRepos().Update(ctx, clusterRepo, metav1.UpdateOptions{})
	w.Require().NoError(err)
	downloadTime := clusterRepo.Status.DownloadTime
	w.Require().NoError(w.pollUntilDownloaded("rancher-charts", downloadTime))

	// Rancher installs the pinned version of the operator chart, so pin the latest version in the repo.
	w.Require().NoError(w.updateSetting(aksOperatorVersionSetting, "104.0.2+up1.9.0"))
	w.Require().NoError(w.updateManagementCluster())
	app, _, err := w.waitForAksChart(rv1.StatusDeployed, "rancher-aks-operator", 0)
	w.Require().NoError(err)
	w.Require().Equal("104.0.2+up1.9.0", app.Spec.Chart.Metadata.Version)

	latest, err := w.catalogClient.GetLatestChartVersion("rancher-aks-operator", catalog.RancherChartRepo)
	w.Require().NoError(err)
	w.Assert().Equal(app.Spec.Chart.Metadata.Version, latest)
	w.Require().Nil(app.Spec.Values)
	w.Require().Nil(app.Spec.Chart.Values)
}

func (w *RancherManagedChartsTest) TestUpgradeChartToLatestVersion() {
	defer w.resetSettings()
	ctx := context.Background()

	clusterRepo, err := w.catalogClient.ClusterRepos().Get(ctx, "rancher-charts", metav1.GetOptions{})
	w.Require().NoError(err)
	clusterRepo.Spec.GitRepo = "https://github.com/rancher/charts-small-fork"
	clusterRepo.Spec.GitBranch = "aks-integration-test-working-charts"
	clusterRepo, err = w.catalogClient.ClusterRepos().Update(ctx, clusterRepo, metav1.UpdateOptions{})
	w.Require().NoError(err)
	downloadTime := clusterRepo.Status.DownloadTime
	w.Require().NoError(w.pollUntilDownloaded("rancher-charts", downloadTime))

	// Pin a version older than the latest in the repo.
	w.Require().NoError(w.updateSetting(aksOperatorVersionSetting, "104.0.1+up1.9.0"))
	w.Require().NoError(w.updateManagementCluster())

	app, _, err := w.waitForAksChart(rv1.StatusDeployed, "rancher-aks-operator", 0)
	w.Require().NoError(err)
	w.Require().Equal("104.0.1+up1.9.0", app.Spec.Chart.Metadata.Version)
	w.Require().Nil(app.Spec.Values)
	w.Require().Nil(app.Spec.Chart.Values)

	// Pin the latest version, as a Rancher upgrade would.
	w.Require().NoError(w.updateSetting(aksOperatorVersionSetting, "104.0.2+up1.9.0"))
	w.Require().NoError(w.retriggerManagementCluster())

	app = w.waitForAksChartVersion("104.0.2+up1.9.0")
	w.Require().Nil(app.Spec.Values)
	w.Require().Nil(app.Spec.Chart.Values)
}

func (w *RancherManagedChartsTest) TestUpgradeToWorkingVersion() {
	defer w.resetSettings()
	ctx := context.Background()
	w.Require().Nil(w.cluster.AKSConfig)
	_, err := w.catalogClient.Apps("cattle-system").Get(ctx, "rancher-aks-charts", metav1.GetOptions{})
	w.Require().Error(err)

	clusterRepo, err := w.catalogClient.ClusterRepos().Get(ctx, "rancher-charts", metav1.GetOptions{})
	w.Require().NoError(err)
	clusterRepo.Spec.GitRepo = "https://github.com/rancher/charts-small-fork"
	clusterRepo.Spec.GitBranch = "aks-integration-test-1"
	clusterRepo, err = w.catalogClient.ClusterRepos().Update(ctx, clusterRepo, metav1.UpdateOptions{})
	w.Require().NoError(err)
	downloadTime := clusterRepo.Status.DownloadTime
	w.Require().NoError(w.pollUntilDownloaded("rancher-charts", downloadTime))

	// Pin a version that fails to install.
	w.Require().NoError(w.updateSetting(aksOperatorVersionSetting, "102.0.0+up1.1.0"))
	list, err := w.catalogClient.Operations("cattle-system").List(ctx, metav1.ListOptions{})
	w.Require().NoError(err)
	numberOfOps := countNumberOfOperations(list, "rancher-aks-operator", time.Now())
	w.Require().NoError(w.updateManagementCluster())

	app, at, err := w.waitForAksChart(rv1.StatusFailed, "rancher-aks-operator", 0)
	w.Require().NoError(err)
	w.Require().Nil(app.Spec.Values)
	w.Require().Nil(app.Spec.Chart.Values)
	list, err = w.catalogClient.Operations("cattle-system").List(ctx, metav1.ListOptions{})
	w.Require().NoError(err)
	w.Require().LessOrEqual(countNumberOfOperations(list, "rancher-aks-operator", at), numberOfOps+2)

	// Pin the working version.
	w.Require().NoError(w.updateSetting(aksOperatorVersionSetting, "102.0.1+up1.1.0"))
	w.Require().NoError(w.retriggerManagementCluster())

	app = w.waitForAksChartVersion("102.0.1+up1.1.0")
	w.Require().Nil(app.Spec.Values)
	w.Require().Nil(app.Spec.Chart.Values)
}

func (w *RancherManagedChartsTest) TestUpgradeToBrokenVersion() {
	defer w.resetSettings()
	ctx := context.Background()

	clusterRepo, err := w.catalogClient.ClusterRepos().Get(ctx, "rancher-charts", metav1.GetOptions{})
	w.Require().NoError(err)
	clusterRepo.Spec.GitRepo = "https://github.com/rancher/charts-small-fork"
	clusterRepo.Spec.GitBranch = "aks-integration-test-2"
	clusterRepo, err = w.catalogClient.ClusterRepos().Update(ctx, clusterRepo, metav1.UpdateOptions{})
	w.Require().NoError(err)

	downloadTime := clusterRepo.Status.DownloadTime
	w.Require().NoError(w.pollUntilDownloaded("rancher-charts", downloadTime))

	// Pin a working version.
	w.Require().NoError(w.updateSetting(aksOperatorVersionSetting, "102.0.0+up1.1.0"))
	w.Require().NoError(w.updateManagementCluster())

	app, at, err := w.waitForAksChart(rv1.StatusDeployed, "rancher-aks-operator", 0)
	w.Require().NoError(err)
	w.Require().Nil(app.Spec.Values)
	w.Require().Nil(app.Spec.Chart.Values)
	w.Require().Equal("102.0.0+up1.1.0", app.Spec.Chart.Metadata.Version)

	ops := w.catalogClient.Operations("cattle-system")
	list, err := ops.List(ctx, metav1.ListOptions{})
	w.Require().NoError(err)
	numberOfOps := countNumberOfOperations(list, "rancher-aks-operator", at)

	// Pin a version that fails to install.
	w.Require().NoError(w.updateSetting(aksOperatorVersionSetting, "102.0.1+up1.1.0"))
	w.Require().NoError(w.retriggerManagementCluster())

	app, at, err = w.waitForAksChart(rv1.StatusFailed, "rancher-aks-operator", app.Spec.Version)
	w.Require().NoError(err)
	w.Require().Nil(app.Spec.Values)
	w.Require().Nil(app.Spec.Chart.Values)
	list, err = ops.List(ctx, metav1.ListOptions{})
	w.Require().NoError(err)
	w.Require().LessOrEqual(countNumberOfOperations(list, "rancher-aks-operator", at), numberOfOps+2)
}

func countNumberOfOperations(ops *rv1.OperationList, name string, at time.Time) int {
	count := 0
	for _, item := range ops.Items {
		if item.Status.Release == name && item.CreationTimestamp.Time.Before(at) {
			count += 1
		}
	}
	return count
}

func (w *RancherManagedChartsTest) waitForAksChart(status rv1.Status, name string, previousVersion int) (*rv1.App, time.Time, error) {
	t := 360
	var app *rv1.App
	var at time.Time
	err := kwait.Poll(PollInterval, time.Duration(t)*time.Second, func() (done bool, err error) {
		app, err = w.catalogClient.Apps("cattle-system").Get(context.TODO(), name, metav1.GetOptions{})
		e, ok := err.(*errors.StatusError)
		if ok && errors.IsNotFound(e) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if app.Spec.Info.Status == status && app.Spec.Version > previousVersion {
			at = time.Now().Add(-(2 * PollInterval)).UTC()
			return true, nil
		}
		return false, nil
	})
	w.Require().NoError(err)
	return app, at, err
}

// waitForAksChartVersion waits until the AKS operator chart is deployed at the given version.
func (w *RancherManagedChartsTest) waitForAksChartVersion(version string) *rv1.App {
	var app *rv1.App
	err := kwait.Poll(2*time.Second, 3*time.Minute, func() (done bool, err error) {
		app, err = w.catalogClient.Apps("cattle-system").Get(context.TODO(), "rancher-aks-operator", metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		return app.Spec.Info.Status == rv1.StatusDeployed && app.Spec.Chart.Metadata.Version == version, nil
	})
	w.Require().NoError(err)
	return app
}

// retriggerManagementCluster turns the AKS config off and on again. Rancher only evaluates the operator chart to
// install when the cluster changes, not when the operator version setting changes.
func (w *RancherManagedChartsTest) retriggerManagementCluster() error {
	w.resetManagementCluster()
	return w.updateManagementCluster()
}

func (w *RancherManagedChartsTest) updateManagementCluster() error {
	w.cluster.AKSConfig = &client.AKSClusterConfigSpec{}
	c, err := w.client.Management.Cluster.Replace(w.cluster)
	w.cluster = c
	return err
}

func (w *RancherManagedChartsTest) resetManagementCluster() {
	w.cluster.AKSConfig = nil
	w.cluster.AppliedSpec.AKSConfig = nil
	c, err := w.client.Management.Cluster.Replace(w.cluster)
	w.Require().NoError(err)
	err = kwait.Poll(5*time.Second, 2*time.Minute, func() (done bool, err error) {
		c, err = w.client.Management.Cluster.ByID("local")
		if err != nil {
			return false, err
		}
		if c.AKSConfig == nil {
			return true, nil
		}
		return false, nil
	})
	w.Require().NoError(err)
	w.cluster = c
}

func (w *RancherManagedChartsTest) updateSetting(name, value string) error {
	// Use the Steve client instead of the main one to be able to set a setting's value to an empty string.
	existing, err := w.client.Steve.SteveType("management.cattle.io.setting").ByID(name)
	if err != nil {
		return err
	}

	var s v3.Setting
	if err := stevev1.ConvertToK8sType(existing.JSONResp, &s); err != nil {
		return err
	}

	s.Value = value
	_, err = w.client.Steve.SteveType("management.cattle.io.setting").Update(existing, s)
	return err
}

func (w *RancherManagedChartsTest) uninstallApp(namespace, chartName string) {
	err := kwait.Poll(10*time.Second, 10*time.Minute, func() (done bool, err error) {
		w.catalogClient.UninstallChart(chartName, namespace, &types.ChartUninstallAction{})

		// Make sure that all helm release secrets are deleted before proceeding.
		helmReleaseSecretLabels := fmt.Sprintf("name=%s,owner=helm", chartName)
		secrets, err := w.corev1.Secrets(namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: helmReleaseSecretLabels,
		})
		w.Require().NoError(err)

		if len(secrets.Items) == 0 {
			return true, nil
		}
		return false, nil
	})
	w.Require().NoError(err)
}

// pollUntilDownloaded Polls until the ClusterRepo of the given name has been downloaded (by comparing prevDownloadTime against the current DownloadTime)
func (w *RancherManagedChartsTest) pollUntilDownloaded(ClusterRepoName string, prevDownloadTime metav1.Time) error {
	var r *rv1.ClusterRepo
	err := kwait.Poll(PollInterval, time.Minute, func() (done bool, err error) {
		r, err = w.catalogClient.ClusterRepos().Get(context.TODO(), ClusterRepoName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		w.Require().NoError(err)
		if r.Name != ClusterRepoName {
			return false, nil
		}

		return r.Status.DownloadTime != prevDownloadTime, nil
	})
	if err != nil {
		logrus.Errorf("Error while downloading ClusterRepo %s from catalog: %v. \n CR: \n %v", ClusterRepoName, err, r)
	}
	return err
}

func (w *RancherManagedChartsTest) TestServeIcons() {
	// Clone the git repository at a spcecific location so
	// that Rancher assumes it as prebuild helm repository.
	// Since Rancher starts at build/testdata, the LocalDir would
	// be build/rancher-data.... Also since this test resides in
	// tests/v2/integration/catalogv2, the cloneDir would be
	// ../../../../build/rancher-data/...
	repoURL := "https://github.com/rancher/charts-small-fork"
	cloneDir := "../../../../build/rancher-data/local-catalogs/v2/rancher-charts-small-fork/d39a2f6abd49e537e5015bbe1a4cd4f14919ba1c3353208a7ff6be37ffe00c52"

	err := os.MkdirAll(cloneDir, os.ModePerm)
	w.Require().NoError(err)

	_, err = git.PlainClone(cloneDir, false, &git.CloneOptions{
		URL:   repoURL,
		Depth: 1,
	})
	w.Require().NoError(err)

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

	imgLength, err := w.catalogClient.FetchChartIcon(smallForkClusterRepoName, "rancher-compliance")
	w.Require().NoError(err)
	w.Assert().Greater(imgLength, 0)

	// Update settings.SystemCatalog to external
	_, err = w.client.Management.Setting.Update(systemCatalog, map[string]interface{}{"value": "external"})
	w.Require().NoError(err)

	// Deleting clusterRepo
	err = w.catalogClient.ClusterRepos().Delete(context.Background(), smallForkClusterRepoName, metav1.DeleteOptions{})
	w.Require().NoError(err)

	// Delete the cloneDir
	err = os.RemoveAll(cloneDir)
	w.Require().NoError(err)
}
