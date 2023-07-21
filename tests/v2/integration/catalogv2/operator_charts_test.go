package integration

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/clients/rancher/catalog"
	client "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	stevev1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/configmaps"
	"github.com/rancher/rancher/tests/framework/extensions/kubeconfig"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
	"io"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"testing"
	"time"
)

type OperatorChartsSuite struct {
	suite.Suite
	client           *rancher.Client
	session          *session.Session
	restClientGetter genericclioptions.RESTClientGetter
	catalogClient    *catalog.Client
	cluster          *client.Cluster
}

func (w *OperatorChartsSuite) TearDownSuite() {
	w.session.Cleanup()
}

func (w *OperatorChartsSuite) SetupSuite() {
	var err error
	testSession := session.NewSession()
	w.session = testSession
	w.client, err = rancher.NewClient("", testSession)
	require.NoError(w.T(), err)

	w.catalogClient, err = w.client.GetClusterCatalogClient("local")
	require.NoError(w.T(), err)

	kubeConfig, err := kubeconfig.GetKubeconfig(w.client, "local")
	require.NoError(w.T(), err)
	restConfig, err := (*kubeConfig).ClientConfig()
	require.NoError(w.T(), err)
	w.restClientGetter, err = kubeconfig.NewRestGetter(restConfig, *kubeConfig)
	require.NoError(w.T(), err)
	c, err := w.client.Management.Cluster.ByID("local")
	require.NoError(w.T(), err)
	w.cluster = c
}

func (w *OperatorChartsSuite) resetSettings() {
	w.T().Helper()
	w.Require().NoError(w.resetManagementCluster())
	w.Require().NoError(w.uninstallApp("cattle-system", "rancher-aks-operator"))
	w.Require().NoError(w.uninstallApp("cattle-system", "rancher-aks-operator-crd"))
	require.NoError(w.T(), w.updateSetting("system-feature-chart-refresh-seconds", "600"))
}

func TestOperatorChartsSuite(t *testing.T) {
	suite.Run(t, new(OperatorChartsSuite))
}

// Reset everything between tests
// DONE 1- Change management cluster crd spec to contain AKSConfig so that AKS operator is installed. Check if the operator was installed successfully and at the latest version
// 2- Change the config map containing the index.yaml and remove the latest version. Do the same steps as item 1, now change the config map to what it was before and check if the chart was upgraded
// 3- Change the config map containing the index.yaml and leave only a version that does not work. Install the aks chart and watch it fail. Trigger another installation by changing a timestamp in the cluster crd
// and check if there's a new operation pod. It shouldn't. Now change the config map to the original value and check if the chart was installed successfully.
// 4- Check if it's possible to install a working version and then upgrade to a broken version. If yes, check the operation pods.

func (w *OperatorChartsSuite) TestInstallChartLatestVersion() {
	w.T().Skip()
	defer w.resetSettings()

	_, err := w.fetchRelease("cattle-system", "rancher-aks-operator")
	w.Require().Error(err)

	w.Require().NoError(w.updateManagementCluster())
	w.Require().NoError(w.waitForAksChart(watch.Added))
	newRelease, err := w.waitForAksRelease(10)
	w.Require().NoError(err)
	latest, err := w.catalogClient.GetLatestChartVersion("rancher-aks-operator")
	w.Require().NoError(err)
	w.Assert().Equal(newRelease.Chart.Metadata.Version, latest)
}

func (w *OperatorChartsSuite) TestUpgradeChartToLatestVersion() {
	defer w.resetSettings()
	var r *client.EKSClusterConfigSpec

	// GETTING CONFIG MAP
	w.Assert().Equal(r, w.cluster.EKSConfig)
	clusterRepo, err := w.catalogClient.ClusterRepos().Get(context.TODO(), "rancher-charts", metav1.GetOptions{})
	w.Require().NoError(err)
	origClusterRepo := clusterRepo.DeepCopy()
	cfg, err := w.client.Steve.SteveType(configmaps.ConfigMapSteveType).ByID(fmt.Sprintf("%s/%s", origClusterRepo.Status.IndexConfigMapNamespace, origClusterRepo.Status.IndexConfigMapName))
	w.Require().NoError(err)
	cfgMap := &v1.ConfigMap{}
	w.Require().NoError(stevev1.ConvertToK8sType(cfg.JSONResp, cfgMap))
	origCfg := cfgMap.DeepCopy()

	// GETTING INDEX FROM CONFIGMAP AND MODIFYING iT
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
	index.Entries["rancher-aks-operator-crd"] = index.Entries["rancher-aks-operator-crd"][1:]
	marshal, err := json.Marshal(index)
	w.Require().NoError(err)
	var compressedData bytes.Buffer
	writer := gzip.NewWriter(&compressedData)
	_, err = writer.Write(marshal)
	w.Require().NoError(err)
	w.Require().NoError(writer.Close())
	cfgMap.BinaryData["content"] = compressedData.Bytes()

	//UPDATING THE CONFIGMAP
	cfg, err = w.client.Steve.SteveType(configmaps.ConfigMapSteveType).Update(cfg, cfgMap)
	w.Require().NoError(stevev1.ConvertToK8sType(cfg.JSONResp, cfgMap))
	_, err = w.fetchRelease("cattle-system", "rancher-aks-operator")
	w.Require().Error(err)

	//UPDAting the CLUSteR
	w.Require().NoError(w.updateManagementCluster())

	w.Require().NoError(w.waitForAksChart(watch.Added))

	newRelease, err := w.waitForAksRelease(10)
	w.Require().NoError(err)

	latest, err := w.catalogClient.GetLatestChartVersion("rancher-aks-operator")
	w.Require().NoError(err)
	w.Assert().Equal(latest, newRelease.Chart.Metadata.Version)
	w.Assert().Greater(latestVersion, latest)

	//REVERT CONFIGMAP TO ORIGINAL VALUE
	cfgMap.BinaryData["content"] = origCfg.BinaryData["content"]
	cfg, err = w.client.Steve.SteveType(configmaps.ConfigMapSteveType).Update(cfg, cfgMap)
	w.Require().NoError(err)

	w.Require().NoError(w.updateSetting("system-feature-chart-refresh-seconds", "1"))

	w.Require().NoError(w.waitForAksChart(watch.Modified))
	newRelease, err = w.waitForAksRelease(10)
	w.Require().NoError(err)
	w.Assert().Equal(latestVersion, newRelease.Chart.Metadata.Version)
}

func (w *OperatorChartsSuite) waitForAksChart(eventType watch.EventType) error {
	t := int64(60)
	watcher, err := w.catalogClient.Apps("cattle-system").Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + "rancher-aks-operator",
		TimeoutSeconds: &t,
	})
	w.Require().NoError(err)

	err = wait.WatchWait(watcher, func(event watch.Event) (ready bool, err error) {
		if event.Type == watch.Error {
			return false, fmt.Errorf("there was an error installing the aks-operator chart")
		} else if event.Type == eventType {
			return true, nil
		}
		return false, nil
	})
	return err
}

func (w *OperatorChartsSuite) waitForAksRelease(secs time.Duration) (*release.Release, error) {
	var newRelease *release.Release
	err := kwait.Poll(secs*time.Second, 2*time.Minute, func() (done bool, err error) {
		newRelease, err = w.fetchRelease("cattle-system", "rancher-aks-operator")
		if err != nil {
			return false, nil
		}
		return true, nil
	})
	w.Require().NoError(err)
	return newRelease, nil
}

func (w *OperatorChartsSuite) getReleases(cfg *action.Configuration) ([]*release.Release, error) {
	l := action.NewList(cfg)
	return l.Run()
}

func (w *OperatorChartsSuite) fetchRelease(namespace, chartName string) (*release.Release, error) {
	var cfg action.Configuration
	if err := cfg.Init(w.restClientGetter, namespace, "", logrus.Infof); err != nil {
		return nil, err
	}
	releases, err := w.getReleases(&cfg)
	if err != nil {
		return nil, err
	}
	for _, r := range releases {
		if r.Chart.Name() == chartName {
			return r, nil
		}
	}
	return nil, fmt.Errorf("%s release not found", chartName)
}

func (w *OperatorChartsSuite) updateManagementCluster() error {
	w.cluster.AKSConfig = &client.AKSClusterConfigSpec{}
	_, err := w.client.Management.Cluster.Replace(w.cluster)
	return err
}

func (w *OperatorChartsSuite) resetManagementCluster() error {
	w.cluster.AKSConfig = nil
	_, err := w.client.Management.Cluster.Replace(w.cluster)
	return err
}

func (w *OperatorChartsSuite) updateSetting(name, value string) error {
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

func (w *OperatorChartsSuite) uninstallApp(namespace, chartName string) error {
	var cfg action.Configuration
	if err := cfg.Init(w.restClientGetter, namespace, "", logrus.Infof); err != nil {
		return err
	}
	releases, err := w.getReleases(&cfg)
	if err != nil {
		return fmt.Errorf("failed to fetch all releases in the %s namespace: %w", namespace, err)
	}
	for _, r := range releases {
		if r.Chart.Name() == chartName {
			err = kwait.Poll(10*time.Second, time.Minute, func() (done bool, err error) {
				if _, err := action.NewUninstall(&cfg).Run(r.Name); err != nil {
					return false, nil
				}
				return true, nil
			})
			return err
		}
	}
	return fmt.Errorf("the chartName %s was never installed", chartName)
}
