package charts

import (
	"context"
	"fmt"
	"time"

	bv1 "github.com/rancher/backup-restore-operator/pkg/apis/resources.cattle.io/v1"
	catalogv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	kubenamespaces "github.com/rancher/rancher/tests/v2/actions/kubeapi/namespaces"
	"github.com/rancher/rancher/tests/v2/actions/namespaces"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/catalog"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/pkg/api/steve/catalog/types"
	"github.com/rancher/shepherd/pkg/wait"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	backupChartNamespace = "cattle-resources-system"
	backupChartName      = "rancher-backup"
)

var chartInstallAction *types.ChartInstallAction

// InstallRancherBackupChart is a helper function that installs the Rancher Backups chart.
func InstallRancherBackupChart(client *rancher.Client, installOptions *InstallOptions, rancherBackupOpts *RancherBackupOpts, withStorage bool) error {
	serverSetting, err := client.Management.Setting.ByID(serverURLSettingID)
	if err != nil {
		return err
	}

	backupChartInstallActionPayload := &payloadOpts{
		InstallOptions: *installOptions,
		Name:           backupChartName,
		Namespace:      backupChartNamespace,
		Host:           serverSetting.Value,
	}

	chartInstallAction = newBackupChartInstallAction(backupChartInstallActionPayload, withStorage, rancherBackupOpts)

	catalogClient, err := client.GetClusterCatalogClient(installOptions.Cluster.ID)
	if err != nil {
		return err
	}

	client.Session.RegisterCleanupFunc(func() error {
		defaultChartUninstallAction := newChartUninstallAction()

		err = catalogClient.UninstallChart(backupChartName, backupChartNamespace, defaultChartUninstallAction)
		if err != nil {
			return err
		}

		watchAppInterface, err := catalogClient.Apps(backupChartNamespace).Watch(context.TODO(), metav1.ListOptions{
			FieldSelector:  metadataName + backupChartName,
			TimeoutSeconds: &defaults.WatchTimeoutSeconds,
		})
		if err != nil {
			return err
		}

		err = wait.WatchWait(watchAppInterface, func(event watch.Event) (ready bool, err error) {
			if event.Type == watch.Error {
				return false, fmt.Errorf("there was an error uninstalling rancher backup chart")
			} else if event.Type == watch.Deleted {
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

		namespace, err := namespaceClient.ByID(backupChartNamespace)
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
			FieldSelector:  metadataName + backupChartNamespace,
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

	watchAppInterface, err := catalogClient.Apps(backupChartNamespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  metadataName + backupChartName,
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

// newBackupChartInstallAction is a private helper function that returns chart install action with backup and payload options.
func newBackupChartInstallAction(p *payloadOpts, withStorage bool, rancherBackupOpts *RancherBackupOpts) *types.ChartInstallAction {
	// If BRO is installed without any storage options selected, then only the basic chart install options are sent
	backupValues := map[string]interface{}{}
	if withStorage {
		backupValues = map[string]any{
			"s3": map[string]any{
				"bucketName":                rancherBackupOpts.BucketName,
				"credentialSecretName":      rancherBackupOpts.CredentialSecretName,
				"credentialSecretNamespace": rancherBackupOpts.CredentialSecretNamespace,
				"enabled":                   rancherBackupOpts.Enabled,
				"endpoint":                  rancherBackupOpts.Endpoint,
				"folder":                    rancherBackupOpts.Folder,
				"region":                    rancherBackupOpts.Region,
			},
		}
	}
	chartInstall := newChartInstall(p.Name, p.InstallOptions.Version, p.InstallOptions.Cluster.ID, p.InstallOptions.Cluster.Name, p.Host, rancherChartsName, p.ProjectID, p.DefaultRegistry, backupValues)
	chartInstallCRD := newChartInstall(p.Name+"-crd", p.InstallOptions.Version, p.InstallOptions.Cluster.ID, p.InstallOptions.Cluster.Name, p.Host, rancherChartsName, p.ProjectID, p.DefaultRegistry, nil)
	chartInstalls := []types.ChartInstall{*chartInstallCRD, *chartInstall}

	chartInstallAction := newChartInstallAction(p.Namespace, p.ProjectID, chartInstalls)

	return chartInstallAction
}

func VerifyBackupCompleted(client *rancher.Client, steveType string, backup *v1.SteveAPIObject) (ready bool, err error) {
	err = kwait.Poll(2*time.Second, 3*time.Minute, func() (done bool, err error) {
		backupObj, err := client.Steve.SteveType(steveType).ByID(backup.ID)
		backupStatus := &bv1.BackupStatus{}
		convertErr := v1.ConvertToK8sType(backupObj.Status, backupStatus)
		if convertErr != nil {
			return false, err
		}
		for _, condition := range backupStatus.Conditions {
			if condition.Type == "Ready" && condition.Status == corev1.ConditionTrue {
				ready = true
				logrus.Infof("Backup is completed!")
			}
		}
		return ready, nil
	})
	if err != nil {
		return ready, err
	}

	return ready, nil
}

func VerifyRestoreCompleted(client *rancher.Client, steveType string, restore *v1.SteveAPIObject) (ready bool, err error) {
	err = kwait.Poll(2*time.Second, 20*time.Minute, func() (done bool, err error) {
		restoreObj, err := client.Steve.SteveType(steveType).ByID(restore.ID)
		if err != nil {
			return false, nil
		}
		restoreStatus := &bv1.RestoreStatus{}
		convertErr := v1.ConvertToK8sType(restoreObj.Status, restoreStatus)
		if convertErr != nil {
			return false, err
		}
		for _, condition := range restoreStatus.Conditions {
			if condition.Type == "Ready" && condition.Status == corev1.ConditionTrue {
				ready = true
				logrus.Infof("Restore is completed!")
			}
		}
		return ready, nil
	})
	if err != nil {
		return ready, err
	}

	return ready, nil
}
