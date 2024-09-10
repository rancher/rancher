package charts

import (
	"context"

	"github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/shepherd/clients/rancher"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/extensions/wait"
	"github.com/rancher/shepherd/pkg/api/steve/catalog/types"
	"github.com/rancher/shepherd/pkg/namegenerator"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	fleetNamespace = "fleet-default"
	localCluster   = "local"
)

// InstallTemplateChart installs a template from a repo.
func InstallTemplateChart(client *rancher.Client, repoName, templateName, clusterName, k8sVersion string, credentials *v1.SteveAPIObject) error {
	latestVersion, err := client.Catalog.GetLatestChartVersion(templateName, repoName)
	if err != nil {
		return err
	}

	project, err := projects.GetProjectByName(client, localCluster, "System")
	if err != nil {
		return err
	}

	installOptions := &InstallOptions{
		Cluster: &clusters.ClusterMeta{
			ID: localCluster,
		},
		Version:   latestVersion,
		ProjectID: project.ID,
	}

	serverSetting, err := client.Management.Setting.ByID(serverURLSettingID)
	if err != nil {
		return err
	}

	registrySetting, err := client.Management.Setting.ByID(defaultRegistrySettingID)
	if err != nil {
		return err
	}

	chartInstallActionPayload := &payloadOpts{
		InstallOptions:  *installOptions,
		Name:            templateName,
		Namespace:       fleetNamespace,
		Host:            serverSetting.Value,
		DefaultRegistry: registrySetting.Value,
	}

	chartValues, err := client.Catalog.GetChartValues(repoName, templateName, installOptions.Version)
	if err != nil {
		return err
	}

	chartInstallAction := TemplateInstallAction(chartInstallActionPayload, repoName, clusterName, credentials.Namespace+":"+credentials.Name, k8sVersion, fleetNamespace, chartValues)

	catalogClient, err := client.GetClusterCatalogClient(installOptions.Cluster.ID)
	if err != nil {
		return err
	}

	err = client.Catalog.InstallChart(chartInstallAction, repoName)
	if err != nil {
		return err
	}

	client.Session.RegisterCleanupFunc(func() error {
		err := client.Catalog.UninstallChart(templateName, fleetNamespace, newChartUninstallAction())
		if err != nil {
			return err
		}

		watchAppInterface, err := catalogClient.Apps(fleetNamespace).Watch(context.TODO(), metav1.ListOptions{
			FieldSelector:  "metadata.name=" + templateName,
			TimeoutSeconds: &defaults.WatchTimeoutSeconds,
		})
		if err != nil {
			return err
		}

		err = wait.ResourceDelete(watchAppInterface)
		if err != nil {
			return err
		}

		return nil
	})

	err = charts.WaitChartInstall(catalogClient, fleetNamespace, templateName)
	if err != nil {
		return err
	}
	return err
}

// TemplateInstallAction creates the payload used when installing a template chart
func TemplateInstallAction(InstallActionPayload *payloadOpts, repoName, clusterName, cloudCredential, k8sVersion, namespace string, chartValues map[string]any) *types.ChartInstallAction {
	chartValues["cloudCredentialSecretName"] = cloudCredential
	chartValues["kubernetesVersion"] = k8sVersion
	chartValues["cluster"].(map[string]any)["name"] = clusterName

	for index := range chartValues["nodepools"].(map[string]any) {
		chartValues["nodepools"].(map[string]any)[index].(map[string]any)["name"] = namegenerator.AppendRandomString("nodepool")
	}

	chartInstall := newChartInstall(
		InstallActionPayload.Name,
		InstallActionPayload.InstallOptions.Version,
		InstallActionPayload.InstallOptions.Cluster.ID,
		InstallActionPayload.InstallOptions.Cluster.Name,
		InstallActionPayload.Host,
		repoName,
		InstallActionPayload.InstallOptions.ProjectID,
		InstallActionPayload.DefaultRegistry,
		chartValues)
	chartInstalls := []types.ChartInstall{*chartInstall}

	return newChartInstallAction(namespace, InstallActionPayload.ProjectID, chartInstalls)
}
