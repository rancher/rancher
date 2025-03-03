package charts

import (
	"context"
	"fmt"

	catalogv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	kubenamespaces "github.com/rancher/rancher/tests/v2/actions/kubeapi/namespaces"
	"github.com/rancher/rancher/tests/v2/actions/namespaces"
	"github.com/rancher/rancher/tests/v2/actions/observability"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/pkg/api/steve/catalog/types"
	"github.com/rancher/shepherd/pkg/wait"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	// Public constants
	StackstateExtensionNamespace = "cattle-ui-plugin-system"
	StackstateExtensionsName     = "observability"
	UIPluginName                 = "rancher-ui-plugins"
	StackstateK8sAgent           = "stackstate-k8s-agent"
	StackstateNamespace          = "stackstate"
	StackstateCRD                = "observability.rancher.io.configuration"
	RancherPartnerChartRepo      = "rancher-partner-charts"
)

var (
	timeoutSeconds = int64(defaults.TwoMinuteTimeout)
)

// InstallStackstateAgentChart is a private helper function that returns chart install action with stack state agent and payload options.
func InstallStackstateAgentChart(client *rancher.Client, installOptions *InstallOptions, stackstateConfigs *observability.StackStateConfig, systemProjectID string) error {
	serverSetting, err := client.Management.Setting.ByID(serverURLSettingID)
	if err != nil {
		log.Info("Error getting server setting.")
		return err
	}

	stackstateAgentChartInstallActionPayload := &payloadOpts{
		InstallOptions: *installOptions,
		Name:           StackstateK8sAgent,
		Namespace:      StackstateNamespace,
		Host:           serverSetting.Value,
	}

	chartInstallAction := newStackstateAgentChartInstallAction(stackstateAgentChartInstallActionPayload, stackstateConfigs, systemProjectID)

	catalogClient, err := client.GetClusterCatalogClient(installOptions.Cluster.ID)
	if err != nil {
		log.Info("Error getting catalogClient")
		return err
	}

	// register uninstall stackstate agent as a cleanup function
	client.Session.RegisterCleanupFunc(func() error {
		defaultChartUninstallAction := newChartUninstallAction()

		err := catalogClient.UninstallChart(StackstateK8sAgent, StackstateNamespace, defaultChartUninstallAction)
		if err != nil {
			return err
		}

		watchAppInterface, err := catalogClient.Apps(StackstateNamespace).Watch(context.TODO(), metav1.ListOptions{
			FieldSelector:  "metadata.name=" + StackstateK8sAgent,
			TimeoutSeconds: &timeoutSeconds,
		})
		if err != nil {
			return err
		}

		err = wait.WatchWait(watchAppInterface, func(event watch.Event) (ready bool, err error) {
			chart := event.Object.(*catalogv1.App)
			if event.Type == watch.Error {
				return false, fmt.Errorf("there was an error uninstalling stackstate agent chart")
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

		namespace, err := namespaceClient.ByID(StackstateNamespace)
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
			FieldSelector:  "metadata.name=" + StackstateNamespace,
			TimeoutSeconds: &timeoutSeconds,
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

	err = catalogClient.InstallChart(chartInstallAction, RancherPartnerChartRepo)
	if err != nil {
		log.Info("Errored installing the chart")
		return err
	}

	// wait for chart to be fully deployed
	watchAppInterface, err := catalogClient.Apps(StackstateNamespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + StackstateK8sAgent,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	if err != nil {
		log.Info("Unable to obtain the installed app ")
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
		log.Info("Unable to obtain the status of the installed app ")
		return err
	}
	return nil
}

// newStackstateAgentChartInstallAction is a helper function that returns an array of newChartInstallActions for installing the stackstate agent charts
func newStackstateAgentChartInstallAction(p *payloadOpts, stackstateConfigs *observability.StackStateConfig, systemProjectID string) *types.ChartInstallAction {
	stackstateValues := map[string]interface{}{
		"stackstate": map[string]interface{}{
			"cluster": map[string]interface{}{
				"name": p.Cluster.Name,
			},
			"apiKey": stackstateConfigs.ClusterApiKey,
			"url":    stackstateConfigs.Url,
		},
	}

	chartInstall := newChartInstall(p.Name, p.Version, p.Cluster.ID, p.Cluster.Name, p.Host, rancherPartnerCharts, systemProjectID, p.DefaultRegistry, stackstateValues)

	chartInstalls := []types.ChartInstall{*chartInstall}
	chartInstallAction := newChartInstallAction(p.Namespace, p.ProjectID, chartInstalls)

	return chartInstallAction
}

// UpgradeStackstateAgentChart is a helper function that upgrades the stackstate agent chart.
func UpgradeStackstateAgentChart(client *rancher.Client, installOptions *InstallOptions, stackstateConfigs *observability.StackStateConfig, systemProjectID string) error {
	serverSetting, err := client.Management.Setting.ByID(serverURLSettingID)
	if err != nil {
		return err
	}

	stackstateAgentChartUpgradeActionPayload := &payloadOpts{
		InstallOptions: *installOptions,
		Name:           StackstateK8sAgent,
		Namespace:      StackstateNamespace,
		Host:           serverSetting.Value,
	}

	chartUpgradeAction := newStackstateAgentChartUpgradeAction(stackstateAgentChartUpgradeActionPayload, stackstateConfigs)

	catalogClient, err := client.GetClusterCatalogClient(installOptions.Cluster.ID)
	if err != nil {
		return err
	}

	err = catalogClient.UpgradeChart(chartUpgradeAction, RancherPartnerChartRepo)
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
	watchAppInterface, err := adminCatalogClient.Apps(StackstateNamespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + StackstateK8sAgent,
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

	watchAppInterface, err = adminCatalogClient.Apps(StackstateNamespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + StackstateK8sAgent,
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

// newStackstateAgentChartUpgradeAction is a private helper function that returns chart upgrade action.
func newStackstateAgentChartUpgradeAction(p *payloadOpts, stackstateConfigs *observability.StackStateConfig) *types.ChartUpgradeAction {

	stackstateValues := map[string]interface{}{
		"stackstate": map[string]interface{}{
			"cluster": map[string]interface{}{
				"name": p.Cluster.Name,
			},
			"apiKey": stackstateConfigs.ClusterApiKey,
			"url":    stackstateConfigs.Url,
		},
	}

	chartUpgrade := newChartUpgrade(p.Name, p.Name, p.Version, p.Cluster.ID, p.Cluster.Name, p.Host, p.DefaultRegistry, stackstateValues)
	chartUpgrades := []types.ChartUpgrade{*chartUpgrade}
	chartUpgradeAction := newChartUpgradeAction(p.Namespace, chartUpgrades)

	return chartUpgradeAction
}
