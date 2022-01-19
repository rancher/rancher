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
	rancherChartsURL = "v1/catalog.cattle.io.clusterrepos/rancher-charts"
	rancherAppsURL   = "v1/catalog.cattle.io.apps/"
)

// GetLatestChartVersion is used to get the lastest version of `chartName`
func (c *Client) GetLatestChartVersion(chartName string) (string, error) {
	result, err := c.RESTClient().Get().
		AbsPath(rancherChartsURL).Param("link", "index").
		VersionedParams(&metav1.GetOptions{}, scheme.ParameterCodec).
		Do(context.TODO()).Raw()

	if err != nil {
		return "", err
	}

	var mapResponse map[string]interface{}
	if err = json.Unmarshal(result, &mapResponse); err != nil {
		return "", err
	}

	entries := mapResponse["entries"]
	specifiedChartEntries := entries.(map[string]interface{})[chartName].([]interface{})
	if len(specifiedChartEntries) < 1 {
		return "", fmt.Errorf("failed to find chart %s from the chart repo", chartName)
	}

	lastestVersion := specifiedChartEntries[0].(map[string]interface{})["version"].(string)
	return lastestVersion, nil
}

// InstallChart installs the chart according to the paratmeter `chart`
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

// UninstallChart uninstalls the chart according to `chartNamespace` and `chartName`
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
