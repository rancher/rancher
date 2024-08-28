package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/api/scheme"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/workloads/deployments"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/catalog"
	stevev1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/kubeapi/workloads/pods"
	"github.com/rancher/shepherd/extensions/kubeconfig"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/rancher/shepherd/pkg/wait"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
	appv1 "k8s.io/api/apps/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
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
	require.NoError(w.T(), err)
	w.restClientGetter, err = kubeconfig.NewRestGetter(restConfig, *kubeConfig)
	require.NoError(w.T(), err)

	w.latestWebhookVersion, err = w.catalogClient.GetLatestChartVersion(rancherWebhook, catalog.RancherChartRepo)
	require.NoError(w.T(), err)

	require.NoError(w.T(), w.updateSetting("rancher-webhook-version", w.latestWebhookVersion))
	require.NoError(w.T(), w.updateSetting("system-feature-chart-refresh-seconds", "10"))
}

func (w *SystemChartsVersionSuite) resetSettings() {
	w.T().Helper()
	require.NoError(w.T(), w.updateSetting("rancher-webhook-version", w.latestWebhookVersion))
	require.NoError(w.T(), w.updateSetting("system-feature-chart-refresh-seconds", "10"))

	// need to recreate the rancher-webhook pod because there are rbac issues without doing so.
	dynamicClient, err := w.client.GetRancherDynamicClient()
	require.NoError(w.T(), err)

	podList, err := dynamicClient.Resource(pods.PodGroupVersionResource).Namespace(cattleSystemNameSpace).List(context.Background(), metav1.ListOptions{})
	require.NoError(w.T(), err)

	var podName string

	for _, pod := range podList.Items {
		name := pod.GetName()
		if strings.Contains(name, rancherWebhook) {
			podName = name
			break
		}
	}

	err = dynamicClient.Resource(pods.PodGroupVersionResource).Namespace(cattleSystemNameSpace).Delete(context.Background(), podName, metav1.DeleteOptions{})
	require.NoError(w.T(), err)

	err = kwait.Poll(500*time.Millisecond, 10*time.Minute, func() (done bool, err error) {
		deployment, err := dynamicClient.Resource(deployments.DeploymentGroupVersionResource).Namespace(cattleSystemNameSpace).Get(context.TODO(), rancherWebhook, metav1.GetOptions{})
		if k8sErrors.IsNotFound(err) {
			return false, nil
		} else if err != nil {
			return false, err
		}

		newDeployment := &appv1.Deployment{}
		err = scheme.Scheme.Convert(deployment, newDeployment, deployment.GroupVersionKind())
		if err != nil {
			return false, err
		}
		if newDeployment.Status.ReadyReplicas == *newDeployment.Spec.Replicas {
			return true, nil
		}

		return false, nil
	})
	require.NoError(w.T(), err)

}

func TestSystemChartsVersionSuite(t *testing.T) {
	// suite.Run(t, new(SystemChartsVersionSuite))
}

func (w *SystemChartsVersionSuite) TestInstallWebhook() {
	defer w.resetSettings()

	const exactVersion = "2.0.3+up0.3.3"
	w.Require().NoError(w.uninstallApp("cattle-system", "rancher-webhook"))
	w.Require().NoError(w.updateSetting("rancher-webhook-version", exactVersion))

	watcher, err := w.catalogClient.Apps("cattle-system").Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + "rancher-webhook",
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	w.Require().NoError(err)

	err = wait.WatchWait(watcher, func(event watch.Event) (ready bool, err error) {
		if event.Type == watch.Error {
			return false, fmt.Errorf("there was an error installing the rancher-webhook chart")
		} else if event.Type == watch.Added {
			return true, nil
		}
		return false, nil
	})
	w.Require().NoError(err)

	// Allow the new release to fully deploy. Otherwise, the client won't find it among current releases.
	var newRelease *release.Release
	err = kwait.Poll(10*time.Second, 2*time.Minute, func() (done bool, err error) {
		newRelease, err = w.fetchRelease("cattle-system", "rancher-webhook")
		if err != nil {
			return false, nil
		}
		if v := newRelease.Chart.Metadata.Version; v != exactVersion {
			w.T().Logf("%s version %s does not yet match expected %s", newRelease.Chart.Name(), v, exactVersion)
			return false, nil
		}
		return true, nil
	})
	w.Require().NoError(err)
}

func (w *SystemChartsVersionSuite) TestInstallFleet() {
	defer w.resetSettings()

	w.Require().NoError(w.uninstallApp("cattle-fleet-system", "fleet"))

	const minVersion = "102.0.0+up0.6.0"
	w.Require().NoError(w.updateSetting("fleet-min-version", minVersion))

	watcher, err := w.catalogClient.Apps("cattle-fleet-system").Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + "fleet",
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	w.Require().NoError(err)

	err = wait.WatchWait(watcher, func(event watch.Event) (ready bool, err error) {
		if event.Type == watch.Error {
			return false, fmt.Errorf("there was an error installing the fleet chart")
		} else if event.Type == watch.Added {
			return true, nil
		}
		return false, nil
	})
	w.Require().NoError(err)

	// Allow the new release to fully deploy. Otherwise, the client won't find it among current releases.
	var newRelease *release.Release
	err = kwait.Poll(10*time.Second, 2*time.Minute, func() (done bool, err error) {
		newRelease, err = w.fetchRelease("cattle-fleet-system", "fleet")
		if err != nil {
			return false, nil
		}
		return true, nil
	})
	w.Require().NoError(err)

	latest, err := w.catalogClient.GetLatestChartVersion("fleet", catalog.RancherChartRepo)
	w.Require().NoError(err)

	// Ensure Rancher deployed the latest version when the minimum version is below the latest.
	w.Assert().Equal(newRelease.Chart.Metadata.Version, latest)
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

	s.Value = value
	_, err = w.client.Steve.SteveType("management.cattle.io.setting").Update(existing, s)
	return err
}
