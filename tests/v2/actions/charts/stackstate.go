package charts

import (
	"context"

	catalogv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/pkg/api/steve/catalog/types"
	"github.com/rancher/shepherd/pkg/wait"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	// Namespace the plugin needs to be installed in
	StackstateExtensionNamespace = "cattle-ui-plugin-system"
	// Name of the stack state extensions plugin
	StackstateChartName = "observability"
	UIPluginName   = "rancher-ui-plugins"
)

// InstallRancherIstioChart is a helper function that installs the rancher-istio chart.
func InstallStackstateExtension(client *rancher.Client, installExtensionOptions *ExtensionOptions) error {

	extensionInstallAction := newStackstateExtensionsInstallAction(installExtensionOptions)

	catalogClient, err := client.GetClusterCatalogClient("local")
	if err != nil {
		return err
	}

	err = catalogClient.InstallChart(extensionInstallAction, UIPluginName)
	if err != nil {
		return err
	}

	// wait for chart to be full deployed
	watchAppInterface, err := catalogClient.Apps(StackstateExtensionNamespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + StackstateChartName,
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

// newIstioChartInstallAction is a private helper function that returns chart install action with istio and payload options.
func newStackstateExtensionsInstallAction(p *ExtensionOptions) *types.ChartInstallAction {

	chartInstall := newExtensionsInstall(p.ChartName, p.Version, nil)
	chartInstalls := []types.ChartInstall{*chartInstall}

	chartInstallAction := newExtensionsInstallAction(StackstateExtensionNamespace, chartInstalls)

	return chartInstallAction
}
