package catalog

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rancher/rancher/pkg/api/steve/catalog/types"
	scheme "github.com/rancher/rancher/pkg/generated/clientset/versioned/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ClusterRepoSteveResourceType = "catalog.cattle.io.clusterrepo"

	rancherChartsURL = "v1/catalog.cattle.io.clusterrepos/rancher-charts"
	rancherAppsURL   = "v1/catalog.cattle.io.apps/"
)

// GetListChartVersions is used to get the list of versions of `chartName`
func (c *Client) GetListChartVersions(chartName string) ([]string, error) {
	result, err := c.RESTClient().Get().
		AbsPath(rancherChartsURL).Param("link", "index").
		VersionedParams(&metav1.GetOptions{}, scheme.ParameterCodec).
		Do(context.TODO()).Raw()

	if err != nil {
		return nil, err
	}

	var mapResponse map[string]interface{}
	if err = json.Unmarshal(result, &mapResponse); err != nil {
		return nil, err
	}

	entries := mapResponse["entries"]
	specifiedChartEntries := entries.(map[string]interface{})[chartName].([]interface{})
	if len(specifiedChartEntries) < 1 {
		return nil, fmt.Errorf("failed to find chart %s from the chart repo", chartName)
	}

	versionsList := []string{}
	for _, entry := range specifiedChartEntries {
		entryMap := entry.(map[string]interface{})
		versionsList = append(versionsList, entryMap["version"].(string))
	}

	return versionsList, nil
}

// GetLatestChartVersion is used to get the lastest version of `chartName`
func (c *Client) GetLatestChartVersion(chartName string) (string, error) {
	versionsList, err := c.GetListChartVersions(chartName)
	if err != nil {
		return "", err
	}
	lastestVersion := versionsList[0]

	return lastestVersion, nil
}

// InstallChart installs the chart according to the parameter `chart`
func (c *Client) InstallChart(chart *types.ChartInstallAction) error {
	bodyContent, err := json.Marshal(chart)
	if err != nil {
		return err
	}

	result := c.RESTClient().Post().
		AbsPath(rancherChartsURL).Param("action", "install").
		VersionedParams(&metav1.CreateOptions{}, scheme.ParameterCodec).
		Body(bodyContent).
		Do(context.TODO())

	return result.Error()
}

// InstallChartFromRepo installs the chart according to the parameter `chart` and `repoName`
func (c *Client) InstallChartFromRepo(chart *types.ChartInstallAction, repoName string) error {
	bodyContent, err := json.Marshal(chart)
	if err != nil {
		return err
	}

	result := c.RESTClient().Post().
		AbsPath(fmt.Sprintf("v1/catalog.cattle.io.clusterrepos/%s", repoName)).Param("action", "install").
		VersionedParams(&metav1.CreateOptions{}, scheme.ParameterCodec).
		Body(bodyContent).
		Do(context.TODO())

	return result.Error()
}

// UpgradeChart upgrades the chart according to the parameter `chart`
func (c *Client) UpgradeChart(chart *types.ChartUpgradeAction) error {
	bodyContent, err := json.Marshal(chart)
	if err != nil {
		return err
	}

	result := c.RESTClient().Post().
		AbsPath(rancherChartsURL).Param("action", "upgrade").
		VersionedParams(&metav1.CreateOptions{}, scheme.ParameterCodec).
		Body(bodyContent).
		Do(context.TODO())

	return result.Error()
}

// UninstallChart uninstalls the chart according to `chartNamespace`, `chartName`, and `uninstallAction`
func (c *Client) UninstallChart(chartName, chartNamespace string, uninstallAction *types.ChartUninstallAction) error {
	bodyContent, err := json.Marshal(uninstallAction)
	if err != nil {
		return err
	}

	url := rancherAppsURL + chartNamespace
	result := c.RESTClient().Post().
		Name(chartName).
		AbsPath(url).Param("action", "uninstall").
		Body(bodyContent).
		VersionedParams(&metav1.CreateOptions{}, scheme.ParameterCodec).
		Do(context.TODO())

	return result.Error()
}
