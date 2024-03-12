package charts

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/pkg/api/steve/catalog/types"
	catalogv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/defaults"
	kubenamespaces "github.com/rancher/rancher/tests/framework/extensions/kubeapi/namespaces"
	"github.com/rancher/rancher/tests/framework/extensions/namespaces"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	// Namespace that rancher cpi chart is installed in
	VsphereCpiNamespace = "kube-system"
	// Name of the rancher vpshere cpi chart
	VsphereCpiName = "rancher-vsphere-cpi"
)

type CpiConfig struct {
	Datacenters string `json:"datacenters" yaml:"datacenters"`
	Host        string `json:"host" yaml:"host"`
	Username    string `json:"username" yaml:"username"`
	Password    string `json:"password" yaml:"password"`
}

// InstallVsphereCpiChart is a helper function that installs the rancher-monitoring chart.
func InstallVsphereCpiChart(client *rancher.Client, installOptions *InstallOptions, cpiConfig *CpiConfig) error {
	serverSetting, err := client.Management.Setting.ByID(serverURLSettingID)
	if err != nil {
		return err
	}

	registrySetting, err := client.Management.Setting.ByID(defaultRegistrySettingID)
	if err != nil {
		return err
	}

	cpiChartInstallActionPayload := &payloadOpts{
		InstallOptions:  *installOptions,
		Name:            VsphereCpiName,
		Namespace:       VsphereCpiNamespace,
		Host:            serverSetting.Value,
		DefaultRegistry: registrySetting.Value,
	}

	chartInstallAction := newCpiChartInstallAction(cpiChartInstallActionPayload, cpiConfig)

	catalogClient, err := client.GetClusterCatalogClient(installOptions.ClusterID)
	if err != nil {
		return err
	}

	client.Session.RegisterCleanupFunc(func() error {
		defaultChartUninstallAction := newChartUninstallAction()

		err = catalogClient.UninstallChart(VsphereCpiName, VsphereCpiNamespace, defaultChartUninstallAction)
		if err != nil {
			return err
		}

		watchAppInterface, err := catalogClient.Apps(VsphereCpiNamespace).Watch(context.TODO(), metav1.ListOptions{
			FieldSelector:  "metadata.name=" + VsphereCpiName,
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

		err = catalogClient.UninstallChart(RancherMonitoringCRDName, VsphereCpiNamespace, defaultChartUninstallAction)
		if err != nil {
			return err
		}

		watchAppInterface, err = catalogClient.Apps(VsphereCpiNamespace).Watch(context.TODO(), metav1.ListOptions{
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

		steveclient, err := client.Steve.ProxyDownstream(installOptions.ClusterID)
		if err != nil {
			return err
		}

		namespaceClient := steveclient.SteveType(namespaces.NamespaceSteveType)

		namespace, err := namespaceClient.ByID(VsphereCpiNamespace)
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
		adminDynamicClient, err := adminClient.GetDownStreamClusterClient(installOptions.ClusterID)
		if err != nil {
			return err
		}
		adminNamespaceResource := adminDynamicClient.Resource(kubenamespaces.NamespaceGroupVersionResource).Namespace("")

		watchNamespaceInterface, err := adminNamespaceResource.Watch(context.TODO(), metav1.ListOptions{
			FieldSelector:  "metadata.name=" + VsphereCpiNamespace,
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

	err = catalogClient.InstallChart(chartInstallAction)
	if err != nil {
		return err
	}

	// wait for chart to be full deployed
	watchAppInterface, err := catalogClient.Apps(VsphereCpiNamespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + VsphereCpiName,
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
func newCpiChartInstallAction(p *payloadOpts, cpiConfig *CpiConfig) *types.ChartInstallAction {

	cpiValues := map[string]interface{}{
		"vCenter": map[string]interface{}{
			"datacenters": cpiConfig.Datacenters, // Update with datacenters if needed
			"host":        cpiConfig.Host,        // Update with vSphere host
			"password":    cpiConfig.Password,    // Update with your vSphere password
			"username":    cpiConfig.Username,    // Update with your vSphere username
		},
	}
	chartInstall := newChartInstall(p.Name, p.InstallOptions.Version, p.InstallOptions.ClusterID, p.InstallOptions.ClusterName, p.Host, p.DefaultRegistry, cpiValues)
	chartInstalls := []types.ChartInstall{*chartInstall}

	chartInstallAction := newChartInstallAction(p.Namespace, p.ProjectID, chartInstalls)

	return chartInstallAction
}
