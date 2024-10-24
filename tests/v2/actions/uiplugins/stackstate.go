package uiplugins

import (
	"context"
	"fmt"

	"github.com/rancher/machine/libmachine/log"
	catalogv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/api/steve/catalog/types"
	"github.com/rancher/shepherd/pkg/wait"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	stackstateExtensionNamespace = "cattle-ui-plugin-system"
	stackstateExtensionsName     = "observability"
	uiPluginName                 = "rancher-ui-plugins"
	local                        = "local"
)

var (
	timeoutSeconds = int64(60 * 2)
)

// InstallStackstateUiPlugin is a helper function that installs the stackstate extension chart in the local cluster of rancher.
func InstallStackstateUiPlugin(client *rancher.Client, installExtensionOptions *ExtensionOptions) error {

	extensionInstallAction := newStackstateUiPluginInstallAction(installExtensionOptions)

	catalogClient, err := client.GetClusterCatalogClient(local)
	if err != nil {
		return err
	}

	// register uninstall stackstate extension as a cleanup function
	client.Session.RegisterCleanupFunc(func() error {
		defaultChartUninstallAction := newPluginUninstallAction()

		err := catalogClient.UninstallChart(stackstateExtensionsName, stackstateExtensionNamespace, defaultChartUninstallAction)
		if err != nil {
			return err
		}
		log.Info("Uninstalled observability extension successfully.")

		watchAppInterface, err := catalogClient.Apps(stackstateExtensionNamespace).Watch(context.TODO(), metav1.ListOptions{
			FieldSelector:  "metadata.name=" + stackstateExtensionsName,
			TimeoutSeconds: &timeoutSeconds,
		})
		if err != nil {
			return err
		}

		err = wait.WatchWait(watchAppInterface, func(event watch.Event) (ready bool, err error) {
			chart := event.Object.(*catalogv1.App)
			if event.Type == watch.Error {
				return false, fmt.Errorf("there was an error uninstalling stackstate extension")
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

		err = catalogClient.UninstallChart(stackstateExtensionsName, stackstateExtensionNamespace, defaultChartUninstallAction)
		return err
	})

	err = catalogClient.InstallChart(extensionInstallAction, uiPluginName)
	if err != nil {
		return err
	}

	watchAppInterface, err := catalogClient.Apps(stackstateExtensionNamespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + stackstateExtensionsName,
		TimeoutSeconds: &timeoutSeconds,
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

// newStackstateUiPluginInstallAction is a private helper function that returns chart install action with stackstate extension payload options.
func newStackstateUiPluginInstallAction(p *ExtensionOptions) *types.ChartInstallAction {

	chartInstall := newPluginsInstall(p.ChartName, p.Version, nil)
	chartInstalls := []types.ChartInstall{*chartInstall}

	chartInstallAction := &types.ChartInstallAction{
		Namespace: stackstateExtensionNamespace,
		Charts:    chartInstalls,
	}

	return chartInstallAction
}
