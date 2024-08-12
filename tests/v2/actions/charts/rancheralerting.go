package charts

import (
	"context"
	"fmt"

	catalogv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	kubenamespaces "github.com/rancher/rancher/tests/v2/actions/kubeapi/namespaces"
	"github.com/rancher/rancher/tests/v2/actions/namespaces"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/catalog"
	"github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/pkg/api/steve/catalog/types"
	"github.com/rancher/shepherd/pkg/wait"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	// Namespace that rancher alerting drivers chart is installed
	RancherAlertingNamespace = RancherMonitoringNamespace
	// Name of the rancher alerting drivers chart
	RancherAlertingName = "rancher-alerting-drivers"
)

// InstallRancherALertingChart is a helper function that installs the rancher-alerting-drivers chart.
func InstallRancherAlertingChart(client *rancher.Client, installOptions *InstallOptions, rancherAlertingOpts *RancherAlertingOpts) error {
	serverSetting, err := client.Management.Setting.ByID(serverURLSettingID)
	if err != nil {
		return err
	}

	registrySetting, err := client.Management.Setting.ByID(defaultRegistrySettingID)
	if err != nil {
		return err
	}

	alertingChartInstallActionPayload := &payloadOpts{
		InstallOptions:  *installOptions,
		Name:            RancherAlertingName,
		Namespace:       RancherAlertingNamespace,
		Host:            serverSetting.Value,
		DefaultRegistry: registrySetting.Value,
	}

	chartInstallAction := newAlertingChartInstallAction(alertingChartInstallActionPayload, rancherAlertingOpts)
	if err != nil {
		return err
	}

	catalogClient, err := client.GetClusterCatalogClient(installOptions.Cluster.ID)
	if err != nil {
		return err
	}

	// Cleanup registration
	client.Session.RegisterCleanupFunc(func() error {
		// UninstallAction for when uninstalling the rancher-alerting-drivers chart
		defaultChartUninstallAction := newChartUninstallAction()

		err = catalogClient.UninstallChart(RancherAlertingName, RancherAlertingNamespace, defaultChartUninstallAction)
		if err != nil {
			return err
		}

		watchAppInterface, err := catalogClient.Apps(RancherAlertingNamespace).Watch(context.TODO(), metav1.ListOptions{
			FieldSelector:  "metadata.name=" + RancherAlertingName,
			TimeoutSeconds: &defaults.WatchTimeoutSeconds,
		})
		if err != nil {
			return err
		}

		err = wait.WatchWait(watchAppInterface, func(event watch.Event) (ready bool, err error) {
			if event.Type == watch.Error {
				return false, fmt.Errorf("there was an error uninstalling rancher alert drivers chart")
			} else if event.Type == watch.Deleted {
				return true, nil
			}
			return false, nil
		})
		if err != nil {
			return err
		}

		monitoringChart, err := charts.GetChartStatus(client, installOptions.Cluster.ID, RancherMonitoringNamespace, RancherMonitoringName)
		if err != nil {
			return err
		}

		// prevent hitting delete twice for the monitoring namespace while CRDs are being deleted
		if !monitoringChart.IsAlreadyInstalled {
			steveclient, err := client.Steve.ProxyDownstream(installOptions.Cluster.ID)
			if err != nil {
				return err
			}

			namespaceClient := steveclient.SteveType(namespaces.NamespaceSteveType)

			namespace, err := namespaceClient.ByID(RancherAlertingNamespace)
			if err != nil {
				return err
			}

			err = namespaceClient.Delete(namespace)
			if err != nil {
				return err
			}

			adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
			if err != nil {
				return err
			}
			adminDynamicClient, err := adminClient.GetDownStreamClusterClient(installOptions.Cluster.ID)
			if err != nil {
				return err
			}
			adminNamespaceResource := adminDynamicClient.Resource(kubenamespaces.NamespaceGroupVersionResource).Namespace("")

			watchNamespaceInterface, err := adminNamespaceResource.Watch(context.TODO(), metav1.ListOptions{
				FieldSelector:  "metadata.name=" + RancherAlertingNamespace,
				TimeoutSeconds: &defaults.WatchTimeoutSeconds,
			})
			if err != nil {
				return err
			}

			err = wait.WatchWait(watchNamespaceInterface, func(event watch.Event) (ready bool, err error) {
				if event.Type == watch.Deleted {
					return true, nil
				}
				return false, nil
			})
			if err != nil {
				return err
			}
		}

		return nil
	})

	err = catalogClient.InstallChart(chartInstallAction, catalog.RancherChartRepo)
	if err != nil {
		return err
	}

	// wait for chart to be full deployed
	watchAppInterface, err := catalogClient.Apps(RancherAlertingNamespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + RancherAlertingName,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	if err != nil {
		return err
	}

	return wait.WatchWait(watchAppInterface, func(event watch.Event) (ready bool, err error) {
		app := event.Object.(*catalogv1.App)

		state := app.Status.Summary.State
		if state == string(catalogv1.StatusDeployed) {
			return true, nil
		}
		return false, nil
	})
}

func newAlertingChartInstallAction(p *payloadOpts, opts *RancherAlertingOpts) *types.ChartInstallAction {
	alertingValues := map[string]interface{}{
		"prom2teams": map[string]interface{}{
			"enabled": opts.Teams,
		},
		"sachet": map[string]interface{}{
			"enabled": opts.SMS,
		},
	}

	chartInstall := newChartInstall(p.Name, p.Version, p.Cluster.ID, p.Cluster.Name, p.Host, rancherChartsName, p.ProjectID, p.DefaultRegistry, alertingValues)
	chartInstalls := []types.ChartInstall{*chartInstall}

	return newChartInstallAction(p.Namespace, p.ProjectID, chartInstalls)
}
