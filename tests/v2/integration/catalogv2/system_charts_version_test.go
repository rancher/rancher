package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	catalogv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/clients/rancher/catalog"
	stevev1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/kubeconfig"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

const (
	cattleSystemNameSpace = "cattle-system"
	rancherWebhook        = "rancher-webhook"
)

type SystemChartsVersionSuite struct {
	suite.Suite
	client               *rancher.Client
	session              *session.Session
	restClientGetter     genericclioptions.RESTClientGetter
	catalogClient        *catalog.Client
	latestWebhookVersion string
}

func (w *SystemChartsVersionSuite) TearDownSuite() {
	w.session.Cleanup()
}

func (w *SystemChartsVersionSuite) SetupSuite() {
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
	// increasing the burst to allow helm to complete without client-side throttling
	restConfig.Burst = 250
	restConfig.QPS = 250
	if w.client.RancherConfig.Insecure != nil && *w.client.RancherConfig.Insecure {
		// if rancher is insecure, it's unlikely that the route to the cluster is secure
		restConfig.Insecure = true
	}
	require.NoError(w.T(), err)
	w.restClientGetter, err = kubeconfig.NewRestGetter(restConfig, *kubeConfig)
	require.NoError(w.T(), err)

	w.latestWebhookVersion, err = w.catalogClient.GetLatestChartVersion(rancherWebhook)
	require.NoError(w.T(), err)

	require.NoError(w.T(), w.updateSetting("rancher-webhook-version", ""))
	require.NoError(w.T(), w.updateSetting("rancher-webhook-min-version", ""))
}

func (w *SystemChartsVersionSuite) resetSettings() {
	w.T().Helper()
	// fully reset the webhook and wait for the final version to rollout
	require.NoError(w.T(), w.uninstallApp("cattle-system", "rancher-webhook"))
	require.NoError(w.T(), w.updateSetting("rancher-webhook-version", ""))
	require.NoError(w.T(), w.updateSetting("rancher-webhook-min-version", ""))
	require.NoError(w.T(), w.waitForChartDeploy("cattle-system", "rancher-webhook", w.latestWebhookVersion))
}

func TestSystemChartsVersionSuite(t *testing.T) {
	suite.Run(t, new(SystemChartsVersionSuite))
}

func (w *SystemChartsVersionSuite) TestInstallWebhook() {
	defer w.resetSettings()

	const exactVersion = "2.0.3+up0.3.3"
	w.Require().NoError(w.uninstallApp("cattle-system", "rancher-webhook"))
	w.Require().NoError(w.updateSetting("rancher-webhook-version", exactVersion))
	w.Require().NoError(w.waitForChartDeploy("cattle-system", "rancher-webhook", exactVersion))
}

// TODO (maxsokolovsky) remove once the rancher-webhook-min-version setting is fully removed.
func (w *SystemChartsVersionSuite) TestInstallWebhookMinVersion() {
	defer w.resetSettings()

	const minVersion = "2.0.3+up0.3.3"
	w.Require().NoError(w.uninstallApp("cattle-system", "rancher-webhook"))
	w.Require().NoError(w.updateSetting("rancher-webhook-min-version", minVersion))

	w.Require().NoError(w.waitForChartDeploy("cattle-system", "rancher-webhook", w.latestWebhookVersion))
}

func (w *SystemChartsVersionSuite) uninstallApp(namespace, chartName string) error {
	var cfg action.Configuration
	if err := cfg.Init(w.restClientGetter, namespace, "", logrus.Infof); err != nil {
		return err
	}
	releases, err := w.getReleases(&cfg)
	if err != nil {
		return fmt.Errorf("failed to fetch all releases in the %s namespace: %w", namespace, err)
	}
	var targetRelease *release.Release
	for _, r := range releases {
		if r.Chart.Name() == chartName {
			targetRelease = r
		}
	}
	if targetRelease == nil {
		return fmt.Errorf("the chart name %s was never installed", chartName)
	}
	if _, err := action.NewUninstall(&cfg).Run(targetRelease.Name); err != nil {
		return err
	}
	// remove the app associated with this release so later on we can tell that this was uninstalled
	return w.catalogClient.Apps(namespace).Delete(context.Background(), chartName, metav1.DeleteOptions{})
}

func (w *SystemChartsVersionSuite) waitForChartDeploy(namespace, chartName, version string) error {
	watcher, err := w.catalogClient.Apps(namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + chartName,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	if err != nil {
		return err
	}

	err = wait.WatchWait(watcher, func(event watch.Event) (ready bool, err error) {
		switch event.Type {
		case watch.Added:
			return false, nil
		case watch.Modified:
			app := event.Object.(*catalogv1.App)
			if app.Status.Summary.State == string(catalogv1.StatusDeployed) {
				return true, nil
			}
			return false, nil
		case watch.Error:
			return false, fmt.Errorf("there was an error installing the rancher-webhook chart")
		default:
			return false, nil
		}
	})
	if err != nil {
		return fmt.Errorf("failed to wait for app to rollout %w", err)
	}
	// even though the app has rolled out, there's a short time where the release may not be installed
	return kwait.Poll(10*time.Second, 30*time.Second, func() (done bool, err error) {
		newRelease, err := w.fetchRelease(namespace, chartName)
		if err != nil {
			return false, nil
		}
		if v := newRelease.Chart.Metadata.Version; v != version {
			w.T().Logf("%s version %s does not yet match expected %s", newRelease.Chart.Name(), v, version)
			return false, nil
		}
		return true, nil
	})

}

func (w *SystemChartsVersionSuite) getReleases(cfg *action.Configuration) ([]*release.Release, error) {
	l := action.NewList(cfg)
	return l.Run()
}

func (w *SystemChartsVersionSuite) fetchRelease(namespace, chartName string) (*release.Release, error) {
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

func (w *SystemChartsVersionSuite) updateSetting(name, value string) error {
	// Use the Steve client instead of the main one to be able to set a setting's value to an empty string.
	existing, err := w.client.Steve.SteveType("management.cattle.io.setting").ByID(name)
	if err != nil {
		return err
	}

	var s v3.Setting
	if err := stevev1.ConvertToK8sType(existing.JSONResp, &s); err != nil {
		return err
	}
	// setting sourced from env vars won't react to k8s changes, so we need to un-register them as env vars
	if s.Source == "env" {
		s.Source = ""
	}

	s.Value = value
	_, err = w.client.Steve.SteveType("management.cattle.io.setting").Update(existing, s)
	return err
}
