package integration

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/rancher/rancher/pkg/api/steve/catalog/types"
	rv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/dashboard/plugin"
	"github.com/rancher/rancher/pkg/namespace"
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
	"io"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"net/http"
	"testing"
	"time"
)

type UIPluginTest struct {
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

func (w *UIPluginTest) TearDownSuite() {
	w.Require().NoError(w.uninstallApp(namespace.UIPluginNamespace, "uk-locale"))
	w.Require().NoError(w.uninstallApp(namespace.UIPluginNamespace, "clock"))
	w.Require().NoError(w.catalogClient.ClusterRepos().Delete(context.Background(), "extensions-examples", metav1.DeleteOptions{PropagationPolicy: &propagation}))
	w.session.Cleanup()

}

func (w *UIPluginTest) SetupSuite() {
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
	require.NoError(w.T(), err)
	_, err = w.catalogClient.ClusterRepos().Create(context.Background(), &rv1.ClusterRepo{
		ObjectMeta: metav1.ObjectMeta{Name: "extensions-examples"},
		Spec:       rv1.RepoSpec{GitRepo: "https://github.com/diogoasouza/ui-plugin-examples", GitBranch: "main"},
	}, metav1.CreateOptions{})
	w.Require().NoError(err)
	w.Require().NoError(w.pollUntilDownloaded("extensions-examples", metav1.Time{}))
	w.Require().NoError(w.catalogClient.InstallChartFromRepo(&types.ChartInstallAction{
		DisableHooks:             false,
		Timeout:                  &metav1.Duration{Duration: 60 * time.Second},
		Wait:                     true,
		Namespace:                namespace.UIPluginNamespace,
		DisableOpenAPIValidation: false,
		Charts: []types.ChartInstall{{
			ChartName:   "uk-locale",
			Version:     "0.1.0",
			ReleaseName: "uk-locale",
			Description: "locale",
		}},
	}, "extensions-examples"))
	w.Require().NoError(w.waitForChart(rv1.StatusDeployed, "uk-locale", 0))

	w.Require().NoError(w.catalogClient.InstallChartFromRepo(&types.ChartInstallAction{
		DisableHooks:             false,
		Timeout:                  &metav1.Duration{Duration: 60 * time.Second},
		Wait:                     true,
		Namespace:                namespace.UIPluginNamespace,
		DisableOpenAPIValidation: false,
		Charts: []types.ChartInstall{{
			ChartName:   "clock",
			Version:     "0.2.0",
			ReleaseName: "clock",
			Description: "clock",
		}},
	}, "extensions-examples"))
	w.Require().NoError(w.waitForChart(rv1.StatusDeployed, "clock", 0))

	//Waiting for controller cache to update
	time.Sleep(10 * time.Second)
}

func (w *UIPluginTest) resetSettings() {
}

func TestUIPluginSuite(t *testing.T) {
	suite.Run(t, new(UIPluginTest))
}

// TestGetIndexAuthenticated Tests if all extensions are returned in the index if the user is authenticated
func (w *UIPluginTest) TestGetIndexAuthenticated() {
	defer w.resetSettings()
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}
	req, err := http.NewRequest(http.MethodGet, "https://localhost:8443/v1/uiplugins", nil)
	req.AddCookie(&http.Cookie{
		Name:  "R_SESS",
		Value: w.client.RancherConfig.AdminToken,
	})

	res, err := client.Do(req)
	w.Require().NoError(err)
	body, err := io.ReadAll(res.Body)
	res.Body.Close()
	var index plugin.SafeIndex
	w.Require().NoError(json.Unmarshal(body, &index))
	w.Require().Equal(len(index.Entries), 2)
}

// TestGetIndexUnauthenticated Tests if the unauthenticated extensions (and only them) are present
// in the anonymous index and that it is returned if the user is not authenticated
func (w *UIPluginTest) TestGetIndexUnauthenticated() {
	defer w.resetSettings()
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}

	res, err := client.Get("https://localhost:8443/v1/uiplugins")
	w.Require().NoError(err)
	body, err := io.ReadAll(res.Body)
	res.Body.Close()
	var index plugin.SafeIndex
	w.Require().NoError(json.Unmarshal(body, &index))
	w.Require().Equal(len(index.Entries), 1)
	_, ok := index.Entries["uk-locale"]
	w.Require().True(ok)
}

// TestGetSingleExtensionAuthenticated Tests that the requests succeeds if the user is authenticated
func (w *UIPluginTest) TestGetSingleExtensionAuthenticated() {
	defer w.resetSettings()
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}
	req, _ := http.NewRequest(http.MethodGet, "https://localhost:8443/v1/uiplugins/clock/0.2.0/plugin/clock-0.2.0.umd.min.js", nil)
	req.AddCookie(&http.Cookie{
		Name:  "R_SESS",
		Value: w.client.RancherConfig.AdminToken,
	})

	res, _ := client.Do(req)
	w.Require().Equal(res.StatusCode, http.StatusOK)
}

// TestGetSingleExtensionUnauthenticated Tests that the requests succeeds if
// the user is unauthenticated when the requested extension does not require authentication
func (w *UIPluginTest) TestGetSingleExtensionUnauthenticated() {
	defer w.resetSettings()
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}
	res, _ := client.Get("https://localhost:8443/v1/uiplugins/uk-locale/0.1.0/plugin/uk-locale-0.1.0.umd.min.js")

	w.Require().Equal(res.StatusCode, http.StatusOK)
}

// TestGetSingleUnauthorizedExtension Tests that the requests fails and returns 404 if the
// extension requires authentication and the user is not authenticated
func (w *UIPluginTest) TestGetSingleUnauthorizedExtension() {
	defer w.resetSettings()
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}
	res, _ := client.Get("https://localhost:8443/v1/uiplugins/clock/0.2.0/plugin/clock-0.2.0.umd.min.js")
	w.Require().Equal(res.StatusCode, http.StatusNotFound)
}

func (w *UIPluginTest) WaitForConfigMap(namespace, name, latestVersion string) error {
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

func (w *UIPluginTest) updateConfigMap(cfgMap *v1.ConfigMap) string {
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

func (w *UIPluginTest) waitForChart(status rv1.Status, name string, previousVersion int) error {
	t := 360
	var app *rv1.App
	err := kwait.Poll(time.Duration(500*time.Millisecond), time.Duration(t)*time.Second, func() (done bool, err error) {
		app, err = w.catalogClient.Apps(namespace.UIPluginNamespace).Get(context.TODO(), name, metav1.GetOptions{})
		e, ok := err.(*errors.StatusError)
		if ok && errors.IsNotFound(e) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if app.Spec.Info.Status == status && app.Spec.Version > previousVersion {
			return true, nil
		}
		return false, nil
	})
	w.Require().NoError(err)
	return err
}

func (w *UIPluginTest) updateManagementCluster() error {
	w.cluster.AKSConfig = &client.AKSClusterConfigSpec{}
	c, err := w.client.Management.Cluster.Replace(w.cluster)
	w.cluster = c
	return err
}

func (w *UIPluginTest) resetManagementCluster() {
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

func (w *UIPluginTest) updateSetting(name, value string) error {
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

func (w *UIPluginTest) uninstallApp(namespace, chartName string) error {
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
func (w *UIPluginTest) pollUntilDownloaded(ClusterRepoName string, prevDownloadTime metav1.Time) error {
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
