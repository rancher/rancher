package uiplugins

import (
	"context"
	"fmt"

	"github.com/rancher/machine/libmachine/log"
	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/defaults"
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
	timeoutSeconds = int64(defaults.TwoMinuteTimeout)
)

// InstallObservabilityUiPlugin is a helper function that installs the observability extension chart in the local cluster of rancher.
func InstallObservabilityUiPlugin(client *rancher.Client, installExtensionOptions *ExtensionOptions) error {

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

		watchAppInterface, err := catalogClient.Apps(stackstateExtensionNamespace).Watch(context.TODO(), metav1.ListOptions{
			FieldSelector:  "metadata.name=" + stackstateExtensionsName,
			TimeoutSeconds: &timeoutSeconds,
		})
		if err != nil {
			return err
		}

		err = wait.WatchWait(watchAppInterface, func(event watch.Event) (ready bool, err error) {
			chart := event.Object.(*v1.App)
			if event.Type == watch.Error {
				return false, fmt.Errorf("there was an error uninstalling stackstate extension")
			} else if event.Type == watch.Deleted {
				log.Info("Uninstalled observability extension successfully.")
				return true, nil
			} else if chart == nil {
				return true, nil
			}
			return false, nil

		})

		return err

	})

	err = catalogClient.InstallChart(extensionInstallAction, uiPluginName)
	if err != nil {
		return err
	}

	watchAppInterface, err := catalogClient.Apps(stackstateExtensionNamespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + stackstateExtensionsName,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	if err != nil {
		return err
	}

	err = wait.WatchWait(watchAppInterface, func(event watch.Event) (ready bool, err error) {
		app := event.Object.(*v1.App)

		state := app.Status.Summary.State
		if state == string(v1.StatusDeployed) {
			return true, nil
		}
		return false, nil
	})

	return err
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

// CreateExtensionsRepo is a helper that utilizes the rancher client and add the ui extensions repo to the list if repositories in the local cluster.
func CreateExtensionsRepo(client *rancher.Client, rancherUiPluginsName, uiExtensionGitRepoURL, uiExtensionsRepoBranch string) error {
	log.Info("Adding ui extensions repo to rancher chart repositories in the local cluster.")

	clusterRepoObj := v1.ClusterRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name: rancherUiPluginsName,
		},
		Spec: v1.RepoSpec{
			GitRepo:   uiExtensionGitRepoURL,
			GitBranch: uiExtensionsRepoBranch,
		},
	}

	repoObject, err := client.Catalog.ClusterRepos().Create(context.TODO(), &clusterRepoObj, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	client.Session.RegisterCleanupFunc(func() error {
		err := client.Catalog.ClusterRepos().Delete(context.TODO(), repoObject.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}

		watchAppInterface, err := client.Catalog.ClusterRepos().Watch(context.TODO(), metav1.ListOptions{
			FieldSelector:  "metadata.name=" + repoObject.Name,
			TimeoutSeconds: &defaults.WatchTimeoutSeconds,
		})
		if err != nil {
			return err
		}

		err = wait.WatchWait(watchAppInterface, func(event watch.Event) (ready bool, err error) {
			if event.Type == watch.Error {
				return false, fmt.Errorf("there was an error deleting the cluster repo")
			} else if event.Type == watch.Deleted {
				log.Info("Removed extensions repo successfully.")
				return true, nil
			}
			return false, nil
		})

		return err
	})

	watchAppInterface, err := client.Catalog.ClusterRepos().Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + clusterRepoObj.Name,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})

	if err != nil {
		return err
	}

	err = wait.WatchWait(watchAppInterface, func(event watch.Event) (ready bool, err error) {
		repo := event.Object.(*v1.ClusterRepo)

		state := repo.Status.Conditions
		for _, condition := range state {
			if condition.Type == string(v1.RepoDownloaded) && condition.Status == "True" {
				return true, nil
			}
		}
		return false, nil
	})

	return err
}
