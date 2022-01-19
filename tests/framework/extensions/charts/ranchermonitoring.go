package charts

import (
	"context"
	"fmt"
	"time"

	"github.com/rancher/rancher/pkg/api/steve/catalog/types"
	catalogv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/namespaces"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	RancherMonitoringNamespace    = "cattle-monitoring-system"
	RancherMonitoringName         = "rancher-monitoring"
	RancherMonitoringCRDName      = "rancher-monitoring-crd"
	RancherMonitoringConfigSecret = "alertmanager-rancher-monitoring-alertmanager"
)

//InstallRancherMonitoringChart is a helper function that installs the rancher-montitoring chart.
func InstallRancherMonitoringChart(client *rancher.Client, projectID, clusterName, version string) error {
	chartInstallCRD := types.ChartInstall{
		Annotations: map[string]string{
			"catalog.cattle.io/ui-source-repo":      "rancher-charts",
			"catalog.cattle.io/ui-source-repo-type": "cluster",
		},
		ChartName:   RancherMonitoringCRDName,
		ReleaseName: RancherMonitoringCRDName,
		Version:     version,
		Values: v3.MapStringInterface{
			"global": map[string]interface{}{
				"cattle": map[string]string{
					"clusterId":             clusterName,
					"clusterName":           clusterName,
					"rkePathPrefix":         "",
					"rkeWindowsPathPrefix":  "",
					"systemDefaultRegistry": "",
					"url":                   client.RancherConfig.Host,
				},
				"systemDefaultRegistry": "",
			},
		},
	}
	chartInstall := types.ChartInstall{
		Annotations: map[string]string{
			"catalog.cattle.io/ui-source-repo":      "rancher-charts",
			"catalog.cattle.io/ui-source-repo-type": "cluster",
		},
		ChartName:   RancherMonitoringName,
		ReleaseName: RancherMonitoringName,
		Version:     version,
		Values: v3.MapStringInterface{
			"alertmanager": map[string]interface{}{
				"alertmanagerSpec": map[string]interface{}{
					"configSecret":      RancherMonitoringConfigSecret,
					"useExistingSecret": true,
				},
			},
			"global": map[string]interface{}{
				"cattle": map[string]string{
					"clusterId":             clusterName,
					"clusterName":           clusterName,
					"rkePathPrefix":         "",
					"rkeWindowsPathPrefix":  "",
					"systemDefaultRegistry": "",
					"url":                   client.RancherConfig.Host,
				},
				"systemDefaultRegistry": "",
			},
			"ingressNgnix": map[string]interface{}{
				"enabled": true,
			},
			"prometheus": map[string]interface{}{
				"prometheusSpec": map[string]interface{}{
					"evaluationInterval": "1m",
					"retentionSize":      "50GiB",
					"scrapeInterval":     "1m",
				},
			},
			"rkeControllerManager": map[string]interface{}{
				"enabled": true,
			},
			"rkeEtcd": map[string]interface{}{
				"enabled": true,
			},
			"rkeProxy": map[string]interface{}{
				"enabled": true,
			},
			"rkeScheduler": map[string]interface{}{
				"enabled": true,
			},
		},
	}
	chartInstallAction := &types.ChartInstallAction{
		DisableHooks:             false,
		Timeout:                  &metav1.Duration{Duration: 600 * time.Second},
		Wait:                     true,
		Namespace:                RancherMonitoringNamespace,
		ProjectID:                projectID,
		DisableOpenAPIValidation: false,
		Charts:                   []types.ChartInstall{chartInstallCRD, chartInstall},
	}

	// Cleanup registration
	client.Session.RegisterCleanupFunc(func() error {
		//UninstallAction for when uninstalling the rancher-monitoring chart
		uninstallAction := &types.ChartUninstallAction{
			Description:  "",
			DryRun:       false,
			KeepHistory:  false,
			DisableHooks: false,
			Timeout:      nil,
		}

		err := client.Catalog.UninstallChart(RancherMonitoringName, RancherMonitoringNamespace, uninstallAction)
		if err != nil {
			return err
		}

		watchAppInterface, err := client.Catalog.Apps(RancherMonitoringNamespace).Watch(context.TODO(), metav1.ListOptions{
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

		dynamicClient, err := client.GetDownStreamClusterClient(clusterName)
		if err != nil {
			return err
		}

		namespaceResource := dynamicClient.Resource(namespaces.NamespaceGroupVersionResource).Namespace("")

		err = namespaceResource.Delete(context.TODO(), RancherMonitoringNamespace, metav1.DeleteOptions{})
		if errors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}

		watchNamespaceInterface, err := namespaceResource.Watch(context.TODO(), metav1.ListOptions{
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

	err := client.Catalog.InstallChart(chartInstallAction)
	if err != nil {
		return err
	}

	//wait for chart to be full deployed
	watchAppInterface, err := client.Catalog.Apps(RancherMonitoringNamespace).Watch(context.TODO(), metav1.ListOptions{
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
