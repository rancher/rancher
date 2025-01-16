package charts

import (
	"context"
	"fmt"
	catalogv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	rv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	kubenamespaces "github.com/rancher/rancher/tests/v2/actions/kubeapi/namespaces"
	"github.com/rancher/rancher/tests/v2/actions/namespaces"
	"github.com/rancher/rancher/tests/v2/actions/observability"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/catalog"
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
	StackStateChartRepo          = "suse-observability"
	StackStateChartURL           = "https://charts.rancher.com/server-charts/prime/suse-observability"
)

var (
	timeoutSeconds = int64(defaults.TwoMinuteTimeout)
)

func InstallStackStateChart(client *rancher.Client, installOptions *InstallOptions, stackstateConfigs *observability.StackStateConfig, systemProjectID string) error {

	// Get server URL for chart configuration
	serverSetting, err := client.Management.Setting.ByID(serverURLSettingID)
	if err != nil {
		log.Info("Error getting server setting.")
		return err
	}

	// Create payload options
	stackstateChartInstallActionPayload := &payloadOpts{
		InstallOptions: *installOptions,
		Name:           StackStateChartRepo,
		Namespace:      StackstateNamespace,
		Host:           serverSetting.Value,
	}

	chartInstallAction := newStackStateChartInstallAction(stackstateChartInstallActionPayload, stackstateConfigs, systemProjectID)

	catalogClient, err := client.GetClusterCatalogClient(installOptions.Cluster.ID)
	if err != nil {
		log.Info("Error getting catalogClient")
		return err
	}

	// Create suse-observability chart repo
	//err = CreateClusterRepo(client, catalogClient, StackStateChartRepo, StackStateChartURL)
	//if err != nil {
	//	log.Info("Error adding StackState Chart Repo")
	//	return err
	//}
	//TODO: Create cleanup function

	err = catalogClient.InstallChart(chartInstallAction, StackStateChartRepo)
	if err != nil {
		log.Info("Error installing the StackState chart")
		return err
	}

	watchAppInterface, err := catalogClient.Apps(StackstateNamespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector: "metadata.name=" + "suse-observability",
		//TODO: Change to WatchTimeoutSeconds
		TimeoutSeconds: (*int64)(&defaults.TwoMinuteTimeout),
	})
	if err != nil {
		log.Info("StackState App failed to install")
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

func newStackStateChartInstallAction(p *payloadOpts, stackstateConfigs *observability.StackStateConfig, systemProjectID string) *types.ChartInstallAction {
	stackstatechartValues := map[string]interface{}{
		"stackstate": map[string]interface{}{
			"cluster": map[string]interface{}{
				"name": p.Cluster.Name,
			},
			"apiKey": stackstateConfigs.ClusterApiKey,
			"url":    stackstateConfigs.Url,
		},
	}

	chartInstall := newChartInstall(p.Name, p.Version, p.Cluster.ID, p.Cluster.Name, p.Host, stackStateChart, systemProjectID, p.DefaultRegistry, stackstatechartValues)

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

// CreateClusterRepo creates a new ClusterRepo resource in the Kubernetes cluster using the provided catalog client.
// It takes the client, repository name, and repository URL as arguments and returns an error if the operation fails.
func CreateClusterRepo(client *rancher.Client, catalogClient *catalog.Client, name, url string) error {
	ctx := context.Background()
	repo := buildClusterRepo(name, url)
	_, err := catalogClient.ClusterRepos().Create(ctx, repo, metav1.CreateOptions{})

	client.Session.RegisterCleanupFunc(func() error {

		var propagation = metav1.DeletePropagationForeground
		err := catalogClient.ClusterRepos().Delete(context.Background(), "suse-observability", metav1.DeleteOptions{PropagationPolicy: &propagation})
		if err != nil {
			return err
		}

		return err
	})
	return err
}

// buildClusterRepo creates and returns a new ClusterRepo object with the provided name and URL.
func buildClusterRepo(name, url string) *rv1.ClusterRepo {
	return &rv1.ClusterRepo{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       rv1.RepoSpec{URL: url},
	}
}
