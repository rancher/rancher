package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	stevev1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/clients/rancher/catalog"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
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
	_ = w.updateSetting("rancher-webhook-version", w.latestWebhookVersion)
	_ = w.updateSetting("rancher-webhook-min-version", "")
	_ = w.updateSetting("system-feature-chart-refresh-seconds", "3600")
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

	host := hostName(w.client.RestConfig.Host)
	w.restClientGetter = &genericclioptions.ConfigFlags{
		CAFile:      &w.client.RestConfig.TLSClientConfig.CAFile,
		APIServer:   &host,
		Insecure:    &w.client.RestConfig.TLSClientConfig.Insecure,
		BearerToken: &w.client.RestConfig.BearerToken,
	}

	w.latestWebhookVersion, err = w.catalogClient.GetLatestChartVersion("rancher-webhook")
	require.NoError(w.T(), err)

	require.NoError(w.T(), w.updateSetting("rancher-webhook-version", w.latestWebhookVersion))
	require.NoError(w.T(), w.updateSetting("rancher-webhook-min-version", ""))
	require.NoError(w.T(), w.updateSetting("system-feature-chart-refresh-seconds", "10"))
}

func (w *SystemChartsVersionSuite) resetSettings() {
	w.T().Helper()
	require.NoError(w.T(), w.updateSetting("rancher-webhook-version", w.latestWebhookVersion))
	require.NoError(w.T(), w.updateSetting("rancher-webhook-min-version", ""))
	require.NoError(w.T(), w.updateSetting("system-feature-chart-refresh-seconds", "10"))
}

func TestSystemChartsVersionSuite(t *testing.T) {
	suite.Run(t, new(SystemChartsVersionSuite))
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
	err = kwait.Poll(10*time.Second, time.Minute, func() (done bool, err error) {
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

// TODO (maxsokolovsky) remove once the rancher-webhook-min-version setting is fully removed.
func (w *SystemChartsVersionSuite) TestInstallWebhookMinVersion() {
	defer w.resetSettings()

	const minVersion = "2.0.3+up0.3.3"
	w.Require().NoError(w.uninstallApp("cattle-system", "rancher-webhook"))
	w.Require().NoError(w.updateSetting("rancher-webhook-min-version", minVersion))

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
	err = kwait.Poll(10*time.Second, time.Minute, func() (done bool, err error) {
		newRelease, err = w.fetchRelease("cattle-system", "rancher-webhook")
		if err != nil {
			return false, nil
		}
		if v := newRelease.Chart.Metadata.Version; v != w.latestWebhookVersion {
			w.T().Logf("%s version %s does not yet match expected %s", newRelease.Chart.Name(), v, w.latestWebhookVersion)
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

	latest, err := w.catalogClient.GetLatestChartVersion("fleet")
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

func hostName(host string) string {
	const prefix = "https://"
	if strings.HasPrefix(host, prefix) {
		return host
	}
	return prefix + host
}
