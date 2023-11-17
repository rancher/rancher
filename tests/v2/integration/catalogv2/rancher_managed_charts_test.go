package integration

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	rv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/clients/rancher/catalog"
	client "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	stevev1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/kubeconfig"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/repo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

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
	w.resetSettings()
}

func (w *RancherManagedChartsTest) resetSettings() {
	w.resetManagementCluster()
	list, err := w.catalogClient.Operations("cattle-system").List(context.TODO(), metav1.ListOptions{})
	w.Require().NoError(err)
	for _, item := range list.Items {
		if item.Status.Release == "rancher-aks-operator" || item.Status.Release == "rancher-aks-operator-crd" {
			w.Require().NoError(w.catalogClient.Operations("cattle-system").Delete(context.TODO(), item.Name, metav1.DeleteOptions{PropagationPolicy: &propagation}))
		}
	}
	err = kwait.Poll(2*time.Second, time.Minute, func() (done bool, err error) {
		list, err := w.catalogClient.Operations("cattle-system").List(context.TODO(), metav1.ListOptions{})
		w.Require().NoError(err)
		for _, item := range list.Items {
			if item.Status.Release == "rancher-aks-operator" || item.Status.Release == "rancher-aks-operator-crd" {
				return false, nil
			}
		}
		return true, nil
	})
	w.Require().NoError(err)
	w.Require().NoError(w.uninstallApp("cattle-system", "rancher-aks-operator"))
	w.Require().NoError(w.uninstallApp("cattle-system", "rancher-aks-operator-crd"))
	clusterRepo, err := w.catalogClient.ClusterRepos().Get(context.TODO(), "rancher-charts", metav1.GetOptions{})
	w.Require().NoError(err)
	if clusterRepo.Spec.GitRepo != w.originalGitRepo {
		clusterRepo.Spec.GitRepo = w.originalGitRepo
		clusterRepo.Spec.GitBranch = w.originalBranch
		downloadTime := clusterRepo.Status.DownloadTime
		clusterRepo, err = w.catalogClient.ClusterRepos().Update(context.TODO(), clusterRepo, metav1.UpdateOptions{})
		w.Require().NoError(err)
		w.Require().NoError(w.pollUntilDownloaded("rancher-charts", downloadTime))
	}
}

func TestRancherManagedChartsSuite(t *testing.T) {
	suite.Run(t, new(RancherManagedChartsTest))
}

func (w *RancherManagedChartsTest) TestInstallChartLatestVersion() {
	defer w.resetSettings()

	w.Require().NoError(w.updateManagementCluster())
	app, _, err := w.waitForAksChart(rv1.StatusDeployed, "rancher-aks-operator", 0)
	w.Require().NoError(err)
	latest, err := w.catalogClient.GetLatestChartVersion("rancher-aks-operator", catalog.RancherChartRepo)
	w.Require().NoError(err)
	w.Assert().Equal(app.Spec.Chart.Metadata.Version, latest)
}

func (w *RancherManagedChartsTest) TestUpgradeChartToLatestVersion() {
	defer w.resetSettings()

	clusterRepo, err := w.catalogClient.ClusterRepos().Get(context.TODO(), "rancher-charts", metav1.GetOptions{})
	w.Require().NoError(err)
	cfgMap, err := w.corev1.ConfigMaps(clusterRepo.Status.IndexConfigMapNamespace).Get(context.TODO(), clusterRepo.Status.IndexConfigMapName, metav1.GetOptions{})
	w.Require().NoError(err)
	origCfg := cfgMap.DeepCopy()

	// GETTING INDEX FROM CONFIGMAP AND MODIFYING IT
	originalLatestVersion := w.updateConfigMap(cfgMap)

	//UPDATING THE CONFIGMAP
	cfgMap, err = w.corev1.ConfigMaps(clusterRepo.Status.IndexConfigMapNamespace).Update(context.TODO(), cfgMap, metav1.UpdateOptions{})
	w.Require().NoError(err)

	//KWait for config map to be updated
	w.Require().NoError(w.WaitForConfigMap(clusterRepo.Status.IndexConfigMapNamespace, clusterRepo.Status.IndexConfigMapName, originalLatestVersion))

	//Updating the cluster
	w.Require().NoError(w.updateManagementCluster())

	app, _, err := w.waitForAksChart(rv1.StatusDeployed, "rancher-aks-operator", 0)
	w.Require().NoError(err)

	w.Require().NoError(err)
	w.Assert().Greater(originalLatestVersion, app.Spec.Chart.Metadata.Version)

	//REVERT CONFIGMAP TO ORIGINAL VALUE
	cfgMap.BinaryData["content"] = origCfg.BinaryData["content"]
	cfgMap, err = w.corev1.ConfigMaps(clusterRepo.Status.IndexConfigMapNamespace).Update(context.TODO(), cfgMap, metav1.UpdateOptions{})
	w.Require().NoError(err)

	clusterRepo, err = w.catalogClient.ClusterRepos().Get(context.TODO(), "rancher-charts", metav1.GetOptions{})
	w.Require().NoError(err)
	clusterRepo.Spec.ForceUpdate = &metav1.Time{Time: time.Now()}
	_, err = w.catalogClient.ClusterRepos().Update(context.TODO(), clusterRepo.DeepCopy(), metav1.UpdateOptions{})
	w.Require().NoError(err)

	app, _, err = w.waitForAksChart(rv1.StatusDeployed, "rancher-aks-operator", app.Spec.Version)
	w.Require().NoError(err)

	w.Assert().Equal(originalLatestVersion, app.Spec.Chart.Metadata.Version)
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
	cfgMap, err := w.corev1.ConfigMaps(clusterRepo.Status.IndexConfigMapNamespace).Get(context.TODO(), clusterRepo.Status.IndexConfigMapName, metav1.GetOptions{})
	w.Require().NoError(err)
	origCfg := cfgMap.DeepCopy()

	// GETTING INDEX FROM CONFIGMAP AND MODIFYING iT
	latestVersion := w.updateConfigMap(cfgMap)
	//UPDATING THE CONFIGMAP
	cfgMap, err = w.corev1.ConfigMaps(clusterRepo.Status.IndexConfigMapNamespace).Update(context.TODO(), cfgMap, metav1.UpdateOptions{})
	w.Require().NoError(err)

	//KWait for config map to be updated
	w.Require().NoError(w.WaitForConfigMap(clusterRepo.Status.IndexConfigMapNamespace, clusterRepo.Status.IndexConfigMapName, latestVersion))
	list, err := w.catalogClient.Operations("cattle-system").List(ctx, metav1.ListOptions{})
	w.Require().NoError(err)
	numberOfOps := countNumberOfOperations(list, "rancher-aks-operator", time.Now())
	//Updating the cluster
	w.Require().NoError(w.updateManagementCluster())

	_, at, err := w.waitForAksChart(rv1.StatusFailed, "rancher-aks-operator", 0)
	w.Require().NoError(err)
	list, err = w.catalogClient.Operations("cattle-system").List(ctx, metav1.ListOptions{})
	w.Require().NoError(err)
	w.Require().LessOrEqual(countNumberOfOperations(list, "rancher-aks-operator", at), numberOfOps+2)

	//REVERT CONFIGMAP TO ORIGINAL VALUE
	cfgMap.BinaryData["content"] = origCfg.BinaryData["content"]
	cfgMap, err = w.corev1.ConfigMaps(clusterRepo.Status.IndexConfigMapNamespace).Update(context.TODO(), cfgMap, metav1.UpdateOptions{})
	w.Require().NoError(err)
	clusterRepo, err = w.catalogClient.ClusterRepos().Get(ctx, "rancher-charts", metav1.GetOptions{})
	w.Require().NoError(err)
	clusterRepo.Spec.ForceUpdate = &metav1.Time{Time: time.Now()}
	_, err = w.catalogClient.ClusterRepos().Update(context.TODO(), clusterRepo.DeepCopy(), metav1.UpdateOptions{})
	w.Require().NoError(err)

	app, _, err := w.waitForAksChart(rv1.StatusDeployed, "rancher-aks-operator", 0)
	w.Require().NoError(err)
	w.Assert().Equal(latestVersion, app.Spec.Chart.Metadata.Version)
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
	cfgMap, err := w.corev1.ConfigMaps(clusterRepo.Status.IndexConfigMapNamespace).Get(context.TODO(), clusterRepo.Status.IndexConfigMapName, metav1.GetOptions{})
	w.Require().NoError(err)
	origCfg := cfgMap.DeepCopy()

	// GETTING INDEX FROM CONFIGMAP AND MODIFYING iT
	latestVersion := w.updateConfigMap(cfgMap)
	//UPDATING THE CONFIGMAP
	cfgMap, err = w.corev1.ConfigMaps(clusterRepo.Status.IndexConfigMapNamespace).Update(context.TODO(), cfgMap, metav1.UpdateOptions{})
	w.Require().NoError(err)

	//KWait for config map to be updated
	w.Require().NoError(w.WaitForConfigMap(clusterRepo.Status.IndexConfigMapNamespace, clusterRepo.Status.IndexConfigMapName, latestVersion))

	//Updating the cluster
	w.Require().NoError(w.updateManagementCluster())

	app, at, err := w.waitForAksChart(rv1.StatusDeployed, "rancher-aks-operator", 0)
	w.Require().NoError(err)

	ops := w.catalogClient.Operations("cattle-system")
	list, err := ops.List(ctx, metav1.ListOptions{})
	w.Require().NoError(err)
	numberOfOps := countNumberOfOperations(list, "rancher-aks-operator", at)

	//REVERT CONFIGMAP TO ORIGINAL VALUE
	cfgMap.BinaryData["content"] = origCfg.BinaryData["content"]
	cfgMap, err = w.corev1.ConfigMaps(clusterRepo.Status.IndexConfigMapNamespace).Update(context.TODO(), cfgMap, metav1.UpdateOptions{})
	w.Require().NoError(err)

	clusterRepo, err = w.catalogClient.ClusterRepos().Get(ctx, "rancher-charts", metav1.GetOptions{})
	w.Require().NoError(err)
	clusterRepo.Spec.ForceUpdate = &metav1.Time{Time: time.Now()}
	_, err = w.catalogClient.ClusterRepos().Update(context.TODO(), clusterRepo.DeepCopy(), metav1.UpdateOptions{})
	w.Require().NoError(err)

	_, at, err = w.waitForAksChart(rv1.StatusFailed, "rancher-aks-operator", app.Spec.Version)
	w.Require().NoError(err)
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

func (w *RancherManagedChartsTest) WaitForConfigMap(namespace, name, latestVersion string) error {
	return kwait.Poll(1*time.Second, 3*time.Minute, func() (done bool, err error) {
		cfgMap, err := w.corev1.ConfigMaps(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		w.Require().NoError(err)
		gz, err := gzip.NewReader(bytes.NewBuffer(cfgMap.BinaryData["content"]))
		w.Require().NoError(err)
		defer gz.Close()
		data, err := io.ReadAll(gz)
		w.Require().NoError(err)
		index := &repo.IndexFile{}
		w.Require().NoError(json.Unmarshal(data, index))
		index.SortEntries()
		return index.Entries["rancher-aks-operator"][0].Version < latestVersion, nil
	})
}

func (w *RancherManagedChartsTest) updateConfigMap(cfgMap *v1.ConfigMap) string {
	gz, err := gzip.NewReader(bytes.NewBuffer(cfgMap.BinaryData["content"]))
	w.Require().NoError(err)
	defer gz.Close()
	data, err := io.ReadAll(gz)
	w.Require().NoError(err)
	index := &repo.IndexFile{}
	w.Require().NoError(json.Unmarshal(data, index))
	index.SortEntries()
	latestVersion := index.Entries["rancher-aks-operator"][0].Version
	index.Entries["rancher-aks-operator"] = index.Entries["rancher-aks-operator"][1:]
	marshal, err := json.Marshal(index)
	w.Require().NoError(err)
	var compressedData bytes.Buffer
	writer := gzip.NewWriter(&compressedData)
	_, err = writer.Write(marshal)
	w.Require().NoError(err)
	w.Require().NoError(writer.Close())
	cfgMap.BinaryData["content"] = compressedData.Bytes()
	return latestVersion
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
	err = kwait.Poll(5*time.Second, 2*time.Minute, func() (done bool, err error) {
		list, err := w.corev1.Secrets("cattle-system").List(context.TODO(), metav1.ListOptions{LabelSelector: "name in (rancher-aks-operator, rancher-aks-operator-crd)"})
		w.Require().NoError(err)
		if len(list.Items) == 0 {
			return true, nil
		}
		for _, s := range list.Items {
			w.Require().NoError(w.corev1.Secrets("cattle-system").Delete(context.Background(), s.Name, metav1.DeleteOptions{PropagationPolicy: &propagation}))
		}
		return false, nil
	})
	w.Require().NoError(err)
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

func (w *RancherManagedChartsTest) uninstallApp(namespace, chartName string) error {
	var cfg action.Configuration
	if err := cfg.Init(w.restClientGetter, namespace, "", logrus.Infof); err != nil {
		return err
	}
	l := action.NewList(&cfg)
	l.All = true
	l.SetStateMask()
	releases, err := l.Run()
	if err != nil {
		return fmt.Errorf("failed to fetch all releases in the %s namespace: %w", namespace, err)
	}
	for _, r := range releases {
		if r.Chart.Name() == chartName {
			err = kwait.Poll(10*time.Second, time.Minute, func() (done bool, err error) {
				act := action.NewUninstall(&cfg)
				act.Wait = true
				act.Timeout = time.Minute
				if _, err = act.Run(r.Name); err != nil {
					return false, nil
				}
				return true, nil
			})
			w.Require().NoError(err)
		}
	}
	return nil
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
