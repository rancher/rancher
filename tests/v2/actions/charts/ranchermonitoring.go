package charts

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	catalogv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	kubenamespaces "github.com/rancher/rancher/tests/v2/actions/kubeapi/namespaces"
	"github.com/rancher/rancher/tests/v2/actions/namespaces"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/catalog"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/pkg/api/steve/catalog/types"
	"github.com/rancher/shepherd/pkg/wait"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	// Namespace that rancher monitoring chart is installed in
	RancherMonitoringNamespace = "cattle-monitoring-system"
	// Name of the rancher monitoring chart
	RancherMonitoringName = "rancher-monitoring"
	// Name of the rancher monitoring alert config secret
	RancherMonitoringAlertSecret = "alertmanager-rancher-monitoring-alertmanager"
	// Name of rancher monitoring crd chart
	RancherMonitoringCRDName = "rancher-monitoring-crd"
)

// InstallRancherMonitoringChart is a helper function that installs the rancher-monitoring chart.
func InstallRancherMonitoringChart(client *rancher.Client, installOptions *InstallOptions, rancherMonitoringOpts *RancherMonitoringOpts) error {
	serverSetting, err := client.Management.Setting.ByID(serverURLSettingID)
	if err != nil {
		return err
	}

	registrySetting, err := client.Management.Setting.ByID(defaultRegistrySettingID)
	if err != nil {
		return err
	}

	monitoringChartInstallActionPayload := &payloadOpts{
		InstallOptions:  *installOptions,
		Name:            RancherMonitoringName,
		Namespace:       RancherMonitoringNamespace,
		Host:            serverSetting.Value,
		DefaultRegistry: registrySetting.Value,
	}

	chartInstallAction, err := newMonitoringChartInstallAction(monitoringChartInstallActionPayload, rancherMonitoringOpts)
	if err != nil {
		return err
	}

	catalogClient, err := client.GetClusterCatalogClient(installOptions.Cluster.ID)
	if err != nil {
		return err
	}

	// Cleanup registration
	client.Session.RegisterCleanupFunc(func() error {
		// UninstallAction for when uninstalling the rancher-monitoring chart
		defaultChartUninstallAction := newChartUninstallAction()

		err = catalogClient.UninstallChart(RancherMonitoringName, RancherMonitoringNamespace, defaultChartUninstallAction)
		if err != nil {
			return err
		}

		watchAppInterface, err := catalogClient.Apps(RancherMonitoringNamespace).Watch(context.TODO(), metav1.ListOptions{
			FieldSelector:  "metadata.name=" + RancherMonitoringName,
			TimeoutSeconds: &defaults.WatchTimeoutSeconds,
		})
		if err != nil {
			return err
		}

		err = wait.WatchWait(watchAppInterface, func(event watch.Event) (ready bool, err error) {
			if event.Type == watch.Error {
				return false, fmt.Errorf("there was an error uninstalling rancher monitoring chart")
			} else if event.Type == watch.Deleted {
				return true, nil
			}
			return false, nil
		})
		if err != nil {
			return err
		}

		err = catalogClient.UninstallChart(RancherMonitoringCRDName, RancherMonitoringNamespace, defaultChartUninstallAction)
		if err != nil {
			return err
		}

		watchAppInterface, err = catalogClient.Apps(RancherMonitoringNamespace).Watch(context.TODO(), metav1.ListOptions{
			FieldSelector:  "metadata.name=" + RancherMonitoringCRDName,
			TimeoutSeconds: &defaults.WatchTimeoutSeconds,
		})
		if err != nil {
			return err
		}

		err = wait.WatchWait(watchAppInterface, func(event watch.Event) (ready bool, err error) {
			chart := event.Object.(*catalogv1.App)
			if event.Type == watch.Error {
				return false, fmt.Errorf("there was an error uninstalling rancher monitoring chart")
			} else if event.Type == watch.Deleted {
				return true, nil
			} else if chart == nil {
				return true, nil
			}
			return false, nil
		})
		if err != nil {
			return err
		}

		steveclient, err := client.Steve.ProxyDownstream(installOptions.Cluster.ID)
		if err != nil {
			return err
		}

		namespaceClient := steveclient.SteveType(namespaces.NamespaceSteveType)

		namespace, err := namespaceClient.ByID(RancherMonitoringNamespace)
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
			FieldSelector:  "metadata.name=" + RancherMonitoringNamespace,
			TimeoutSeconds: &defaults.WatchTimeoutSeconds,
		})
		if err != nil {
			return err
		}

		return wait.WatchWait(watchNamespaceInterface, func(event watch.Event) (ready bool, err error) {
			if event.Type == watch.Deleted {
				return true, nil
			}
			return false, nil
		})
	})

	err = catalogClient.InstallChart(chartInstallAction, catalog.RancherChartRepo)
	if err != nil {
		return err
	}

	// wait for chart to be full deployed
	watchAppInterface, err := catalogClient.Apps(RancherMonitoringNamespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + RancherMonitoringName,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	if err != nil {
		return err
	}

	err = wait.WatchWait(watchAppInterface, func(event watch.Event) (ready bool, err error) {
		app := event.Object.(*catalogv1.App)

		state := app.Status.Summary.State
		if state == string(catalogv1.StatusDeployed) {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return err
	}
	return nil
}

// newMonitoringChartInstallAction is a private helper function that returns chart install action with monitoring and payload options.
func newMonitoringChartInstallAction(p *payloadOpts, rancherMonitoringOpts *RancherMonitoringOpts) (*types.ChartInstallAction, error) {
	monitoringValues := map[string]interface{}{
		"prometheus": map[string]interface{}{
			"prometheusSpec": map[string]interface{}{
				"evaluationInterval": "1m",
				"retentionSize":      "50GiB",
				"scrapeInterval":     "1m",
			},
		},
	}

	opts, err := addMonitoringProviderPrefix(p.Cluster.Provider, rancherMonitoringOpts)
	if err != nil {
		return nil, err
	}

	for k, v := range opts {
		monitoringValues[k] = v
	}

	chartInstall := newChartInstall(p.Name, p.Version, p.Cluster.ID, p.Cluster.Name, p.Host, rancherChartsName, p.ProjectID, p.DefaultRegistry, monitoringValues)
	chartInstallCRD := newChartInstall(p.Name+"-crd", p.Version, p.Cluster.ID, p.Cluster.Name, p.Host, rancherChartsName, p.ProjectID, p.DefaultRegistry, nil)
	chartInstalls := []types.ChartInstall{*chartInstallCRD, *chartInstall}

	chartInstallAction := newChartInstallAction(p.Namespace, p.ProjectID, chartInstalls)

	return chartInstallAction, nil
}

// UpgradeMonitoringChart is a helper function that upgrades the rancher-monitoring chart.
func UpgradeRancherMonitoringChart(client *rancher.Client, installOptions *InstallOptions, rancherMonitoringOpts *RancherMonitoringOpts) error {
	serverSetting, err := client.Management.Setting.ByID(serverURLSettingID)
	if err != nil {
		return err
	}

	registrySetting, err := client.Management.Setting.ByID(defaultRegistrySettingID)
	if err != nil {
		return err
	}

	monitoringChartUpgradeActionPayload := &payloadOpts{
		InstallOptions:  *installOptions,
		Name:            RancherMonitoringName,
		Namespace:       RancherMonitoringNamespace,
		Host:            serverSetting.Value,
		DefaultRegistry: registrySetting.Value,
	}

	chartUpgradeAction, err := newMonitoringChartUpgradeAction(monitoringChartUpgradeActionPayload, rancherMonitoringOpts)
	if err != nil {
		return err
	}

	catalogClient, err := client.GetClusterCatalogClient(installOptions.Cluster.ID)
	if err != nil {
		return err
	}

	err = catalogClient.UpgradeChart(chartUpgradeAction, catalog.RancherChartRepo)
	if err != nil {
		return err
	}

	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	if err != nil {
		return err
	}
	adminCatalogClient, err := adminClient.GetClusterCatalogClient(installOptions.Cluster.ID)
	if err != nil {
		return err
	}

	// wait for chart to be in status pending upgrade
	watchAppInterface, err := adminCatalogClient.Apps(RancherMonitoringNamespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + RancherMonitoringName,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	if err != nil {
		return err
	}

	err = wait.WatchWait(watchAppInterface, func(event watch.Event) (ready bool, err error) {
		app := event.Object.(*catalogv1.App)

		state := app.Status.Summary.State
		if state == string(catalogv1.StatusPendingUpgrade) {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return err
	}

	// wait for chart to be full deployed
	watchAppInterface, err = adminCatalogClient.Apps(RancherMonitoringNamespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + RancherMonitoringName,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	if err != nil {
		return err
	}

	err = wait.WatchWait(watchAppInterface, func(event watch.Event) (ready bool, err error) {
		app := event.Object.(*catalogv1.App)

		state := app.Status.Summary.State
		if state == string(catalogv1.StatusDeployed) {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return err
	}

	return nil
}

// newMonitoringChartUpgradeAction is a private helper function that returns chart upgrade action with monitoring and payload options.
func newMonitoringChartUpgradeAction(p *payloadOpts, rancherMonitoringOpts *RancherMonitoringOpts) (*types.ChartUpgradeAction, error) {
	monitoringValues := map[string]interface{}{
		"prometheus": map[string]interface{}{
			"prometheusSpec": map[string]interface{}{
				"evaluationInterval": "1m",
				"retentionSize":      "50GiB",
				"scrapeInterval":     "1m",
			},
		},
	}

	opts, err := addMonitoringProviderPrefix(p.Cluster.Provider, rancherMonitoringOpts)
	if err != nil {
		return nil, err
	}

	for k, v := range opts {
		monitoringValues[k] = v
	}

	chartUpgrade := newChartUpgrade(p.Name, p.Name, p.Version, p.Cluster.ID, p.Cluster.Name, p.Host, p.DefaultRegistry, monitoringValues)
	chartUpgradeCRD := newChartUpgrade(p.Name+"-crd", p.Name+"-crd", p.Version, p.Cluster.ID, p.Cluster.Name, p.Host, p.DefaultRegistry, monitoringValues)
	chartUpgrades := []types.ChartUpgrade{*chartUpgradeCRD, *chartUpgrade}

	chartUpgradeAction := newChartUpgradeAction(p.Namespace, chartUpgrades)

	return chartUpgradeAction, nil
}

// addProvider prefix is a private helper function that adds kubernetes provider to the monitoring opts payload keys. ex) maps "scheduler" to "rke2Scheduler"
func addMonitoringProviderPrefix(provider clusters.KubernetesProvider, opts *RancherMonitoringOpts) (map[string]any, error) {
	optsBytes, err := json.Marshal(opts)
	if err != nil {
		return nil, err
	}

	optsMap := map[string]any{}
	err = json.Unmarshal(optsBytes, &optsMap)
	if err != nil {
		return nil, err
	}

	newOptsMap := map[string]any{}
	ingressKey := "ingressNginx"

	for k, v := range optsMap {
		// ingressNginx on RKE1 doesn't have prefix
		if k == ingressKey && provider == clusters.KubernetesProviderRKE {
			newOptsMap[k] = map[string]any{
				"enabled": v,
			}

			continue
		}

		newKey := fmt.Sprintf("%v%v%v", provider, strings.ToUpper(string(k[0])), k[1:])
		newOptsMap[newKey] = map[string]any{
			"enabled": v,
		}
	}

	return newOptsMap, nil
}
