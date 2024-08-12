package charts

import (
	"context"

	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/rke1/nodetemplates"
	rancherv1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/pkg/api/steve/catalog/types"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	r1vsphere "github.com/rancher/rancher/tests/v2/actions/rke1/nodetemplates/vsphere"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/catalog"
)

const (
	systemProject       = "System"
	vsphereCPIchartName = "rancher-vsphere-cpi"
	manualCPIchartName  = "vsphere-cpi"
	vsphereCSIchartName = "rancher-vsphere-csi"
	manualCSIchartName  = "vsphere-csi"

	vcenter      = "vCenter"
	storageclass = "storageClass"

	datacenters  = "datacenters"
	host         = "host"
	password     = "password"
	username     = "username"
	port         = "port"
	clusterid    = "clusterId"
	datastoreurl = "datastoreURL"
)

// InstallVsphereOutOfTreeCharts installs the CPI and CSI chart for vsphere cloud provider in a given cluster.
func InstallVsphereOutOfTreeCharts(client *rancher.Client, repoName, clusterName string, isLatestVersion bool) error {
	serverSetting, err := client.Management.Setting.ByID(serverURLSettingID)
	if err != nil {
		return err
	}

	cluster, err := clusters.NewClusterMeta(client, clusterName)
	if err != nil {
		return err
	}

	registrySetting, err := client.Management.Setting.ByID(defaultRegistrySettingID)
	if err != nil {
		return err
	}

	project, err := projects.GetProjectByName(client, cluster.ID, systemProject)
	if err != nil {
		return err
	}

	catalogClient, err := client.GetClusterCatalogClient(cluster.ID)
	if err != nil {
		return err
	}

	var cpiVersion string
	var csiVersion string

	cpiVersions, err := catalogClient.GetListChartVersions(vsphereCPIchartName, catalog.RancherChartRepo)
	if err != nil {
		return err
	}

	cpiVersion = cpiVersions[0]

	csiVersions, err := catalogClient.GetListChartVersions(vsphereCSIchartName, catalog.RancherChartRepo)
	if err != nil {
		return err
	}

	csiVersion = csiVersions[0]

	if !isLatestVersion {
		cpiVersion = cpiVersions[len(cpiVersions)-1]
		csiVersion = csiVersions[len(csiVersions)-1]
	}

	installCPIOptions := &InstallOptions{
		Cluster:   cluster,
		Version:   cpiVersion,
		ProjectID: project.ID,
	}

	chartInstallActionPayload := &payloadOpts{
		InstallOptions:  *installCPIOptions,
		Name:            vsphereCPIchartName,
		Namespace:       kubeSystemNamespace,
		Host:            serverSetting.Value,
		DefaultRegistry: registrySetting.Value,
	}

	vsphereTemplateConfig := r1vsphere.GetVsphereNodeTemplate()

	chartInstallAction, err := vsphereCPIChartInstallAction(catalogClient,
		chartInstallActionPayload, vsphereTemplateConfig, installCPIOptions, repoName, kubeSystemNamespace)
	if err != nil {
		return err
	}

	logrus.Infof("executing chart install")
	err = catalogClient.InstallChart(chartInstallAction, repoName)
	if err != nil {
		return err
	}

	logrus.Infof("verifying chart install")
	err = charts.WaitChartInstall(catalogClient, kubeSystemNamespace, vsphereCPIchartName)
	if err != nil {
		return err
	}

	installCSIOptions := &InstallOptions{
		Cluster:   cluster,
		Version:   csiVersion,
		ProjectID: project.ID,
	}

	chartInstallActionPayload = &payloadOpts{
		InstallOptions:  *installCSIOptions,
		Name:            vsphereCSIchartName,
		Namespace:       kubeSystemNamespace,
		Host:            serverSetting.Value,
		DefaultRegistry: registrySetting.Value,
	}

	chartInstallAction, err = vsphereCSIChartInstallAction(catalogClient, chartInstallActionPayload,
		vsphereTemplateConfig, installCSIOptions, repoName, kubeSystemNamespace)
	if err != nil {
		return err
	}

	logrus.Infof("executing chart install")
	err = catalogClient.InstallChart(chartInstallAction, repoName)
	if err != nil {
		return err
	}

	return err
}

// vsphereCPIChartInstallAction is a helper function that returns a chartInstallAction for vsphere out-of-tree chart.
func vsphereCPIChartInstallAction(client *catalog.Client, chartInstallActionPayload *payloadOpts, vsphereTemplateConfig *nodetemplates.VmwareVsphereNodeTemplateConfig, installOptions *InstallOptions, repoName, chartNamespace string) (*types.ChartInstallAction, error) {
	chartValues, err := client.GetChartValues(repoName, vsphereCPIchartName, installOptions.Version)
	if err != nil {
		return nil, err
	}

	chartValues[vcenter].(map[string]interface{})[datacenters] = vsphereTemplateConfig.Datacenter
	chartValues[vcenter].(map[string]interface{})[host] = vsphereTemplateConfig.Vcenter
	chartValues[vcenter].(map[string]interface{})[password] = vsphereTemplateConfig.Password
	chartValues[vcenter].(map[string]interface{})[username] = vsphereTemplateConfig.Username
	chartValues[vcenter].(map[string]interface{})[port] = vsphereTemplateConfig.VcenterPort

	chartInstall := newChartInstall(
		chartInstallActionPayload.Name,
		chartInstallActionPayload.Version,
		chartInstallActionPayload.Cluster.ID,
		chartInstallActionPayload.Cluster.Name,
		chartInstallActionPayload.Host,
		repoName,
		installOptions.ProjectID,
		chartInstallActionPayload.DefaultRegistry,
		chartValues)
	chartInstalls := []types.ChartInstall{*chartInstall}

	return newChartInstallAction(chartNamespace, chartInstallActionPayload.ProjectID, chartInstalls), nil
}

// vsphereCSIChartInstallAction is a helper function that returns a chartInstallAction for vsphere out-of-tree chart.
func vsphereCSIChartInstallAction(client *catalog.Client, chartInstallActionPayload *payloadOpts, vsphereTemplateConfig *nodetemplates.VmwareVsphereNodeTemplateConfig, installOptions *InstallOptions, repoName, chartNamespace string) (*types.ChartInstallAction, error) {
	chartValues, err := client.GetChartValues(repoName, vsphereCSIchartName, installOptions.Version)
	if err != nil {
		return nil, err
	}

	chartValues[vcenter].(map[string]interface{})[datacenters] = vsphereTemplateConfig.Datacenter
	chartValues[vcenter].(map[string]interface{})[host] = vsphereTemplateConfig.Vcenter
	chartValues[vcenter].(map[string]interface{})[password] = vsphereTemplateConfig.Password
	chartValues[vcenter].(map[string]interface{})[username] = vsphereTemplateConfig.Username
	chartValues[vcenter].(map[string]interface{})[port] = vsphereTemplateConfig.VcenterPort
	chartValues[vcenter].(map[string]interface{})[clusterid] = installOptions.Cluster.ID

	chartValues[storageclass].(map[string]interface{})[datastoreurl] = vsphereTemplateConfig.DatastoreURL

	chartInstall := newChartInstall(
		chartInstallActionPayload.Name,
		chartInstallActionPayload.Version,
		chartInstallActionPayload.Cluster.ID,
		chartInstallActionPayload.Cluster.Name,
		chartInstallActionPayload.Host,
		repoName,
		installOptions.ProjectID,
		chartInstallActionPayload.DefaultRegistry,
		chartValues)
	chartInstalls := []types.ChartInstall{*chartInstall}

	return newChartInstallAction(chartNamespace, chartInstallActionPayload.ProjectID, chartInstalls), nil
}

// UpgradeVsphereOutOfTreeCharts upgrades the CPI and CSI chart for vsphere cloud provider in a given cluster to the latest version.
func UpgradeVsphereOutOfTreeCharts(client *rancher.Client, repoName, clusterName string) error {
	serverSetting, err := client.Management.Setting.ByID(serverURLSettingID)
	if err != nil {
		return err
	}

	cluster, err := clusters.NewClusterMeta(client, clusterName)
	if err != nil {
		return err
	}

	registrySetting, err := client.Management.Setting.ByID(defaultRegistrySettingID)
	if err != nil {
		return err
	}

	project, err := projects.GetProjectByName(client, cluster.ID, systemProject)
	if err != nil {
		return err
	}

	catalogClient, err := client.GetClusterCatalogClient(cluster.ID)
	if err != nil {
		return err
	}

	cpiChartName := vsphereCPIchartName
	cpiVersion, err := catalogClient.GetLatestChartVersion(cpiChartName, catalog.RancherChartRepo)
	if err != nil {
		return err
	}

	csiChartName := vsphereCSIchartName
	csiVersion, err := catalogClient.GetLatestChartVersion(csiChartName, catalog.RancherChartRepo)
	if err != nil {
		return err
	}

	cpiChartObject, err := getExistingChartInstall(catalogClient, cpiChartName, kubeSystemNamespace)
	if err != nil {
		cpiChartName = manualCPIchartName
		cpiChartObject, err = getExistingChartInstall(catalogClient, cpiChartName, kubeSystemNamespace)
		if err != nil {
			return err
		}
	}

	cpiSpec := new(v1.ReleaseSpec)
	err = rancherv1.ConvertToK8sType(cpiChartObject.Spec, &cpiSpec)
	if err != nil {
		return err
	}

	csiChartObject, err := getExistingChartInstall(catalogClient, csiChartName, kubeSystemNamespace)
	if err != nil {
		csiChartName = manualCSIchartName
		csiChartObject, err = getExistingChartInstall(catalogClient, csiChartName, kubeSystemNamespace)
		if err != nil {
			return err
		}
	}

	csiSpec := new(v1.ReleaseSpec)
	err = rancherv1.ConvertToK8sType(csiChartObject.Spec, &csiSpec)
	if err != nil {
		return err
	}

	if cpiVersion != cpiSpec.Chart.Metadata.Version {
		installCPIOptions := &InstallOptions{
			Cluster:   cluster,
			Version:   cpiVersion,
			ProjectID: project.ID,
		}

		chartInstallActionPayload := &payloadOpts{
			InstallOptions:  *installCPIOptions,
			Name:            cpiChartName,
			Namespace:       kubeSystemNamespace,
			Host:            serverSetting.Value,
			DefaultRegistry: registrySetting.Value,
		}

		vsphereConfig := r1vsphere.GetVsphereNodeTemplate()

		chartUpgradeAction, err := vsphereCPIChartUpgradeAction(catalogClient,
			chartInstallActionPayload, vsphereConfig, repoName, kubeSystemNamespace)
		if err != nil {
			return err
		}

		err = catalogClient.UpgradeChart(chartUpgradeAction, repoName)
		if err != nil {
			return err
		}

		err = charts.WaitChartUpgrade(catalogClient, kubeSystemNamespace, cpiChartName, cpiVersion)
		if err != nil {
			return err
		}
	} else {
		logrus.Infof(
			"No chart version to upgrade to, already on the latest CPI version: %s vs installed %s",
			cpiVersion,
			cpiSpec.Chart.Metadata.Version)
	}

	if csiVersion != csiSpec.Chart.Metadata.Version {
		installCSIOptions := &InstallOptions{
			Cluster:   cluster,
			Version:   csiVersion,
			ProjectID: project.ID,
		}

		chartInstallActionPayload := &payloadOpts{
			InstallOptions:  *installCSIOptions,
			Name:            csiChartName,
			Namespace:       kubeSystemNamespace,
			Host:            serverSetting.Value,
			DefaultRegistry: registrySetting.Value,
		}

		vsphereConfig := r1vsphere.GetVsphereNodeTemplate()

		chartUpgradeAction, err := vsphereCSIChartUpgradeAction(catalogClient, chartInstallActionPayload,
			vsphereConfig, repoName, kubeSystemNamespace)
		if err != nil {
			return err
		}

		logrus.Infof("executing chart upgrade")
		err = catalogClient.UpgradeChart(chartUpgradeAction, repoName)
		if err != nil {
			return err
		}

		logrus.Infof("verifying chart upgrade")
		err = charts.WaitChartUpgrade(catalogClient, kubeSystemNamespace, csiChartName, csiVersion)
		if err != nil {
			return err
		}

	} else {
		logrus.Infof(
			"No chart version to upgrade to, already on the latest CSI version: %s vs installed %s",
			csiVersion,
			csiSpec.Chart.Metadata.Version)
	}

	return err
}

// vsphereCPIChartUpgradeAction is a helper function that returns a chartUpgradeAction for vsphere out-of-tree chart.
func vsphereCPIChartUpgradeAction(client *catalog.Client, chartUpgradeActionPayload *payloadOpts, vsphereTemplateConfig *nodetemplates.VmwareVsphereNodeTemplateConfig, repoName, chartNamespace string) (*types.ChartUpgradeAction, error) {
	chartValues, err := client.GetChartValues(repoName, chartUpgradeActionPayload.Name, chartUpgradeActionPayload.InstallOptions.Version)
	if err != nil {
		return nil, err
	}

	chartValues[vcenter].(map[string]interface{})[datacenters] = vsphereTemplateConfig.Datacenter
	chartValues[vcenter].(map[string]interface{})[host] = vsphereTemplateConfig.Vcenter
	chartValues[vcenter].(map[string]interface{})[password] = vsphereTemplateConfig.Password
	chartValues[vcenter].(map[string]interface{})[username] = vsphereTemplateConfig.Username
	chartValues[vcenter].(map[string]interface{})[port] = vsphereTemplateConfig.VcenterPort

	chartUpgrade := newChartUpgrade(
		vsphereCPIchartName,
		chartUpgradeActionPayload.Name,
		chartUpgradeActionPayload.Version,
		chartUpgradeActionPayload.Cluster.ID,
		chartUpgradeActionPayload.Cluster.Name,
		repoName,
		chartUpgradeActionPayload.DefaultRegistry,
		chartValues)
	chartUpgrades := []types.ChartUpgrade{*chartUpgrade}

	return newChartUpgradeAction(chartNamespace, chartUpgrades), nil
}

// vsphereCSIChartUpgradeAction is a helper function that returns a chartUpgradeAction for vsphere out-of-tree chart.
func vsphereCSIChartUpgradeAction(client *catalog.Client, chartUpgradeActionPayload *payloadOpts, vsphereTemplateConfig *nodetemplates.VmwareVsphereNodeTemplateConfig, repoName, chartNamespace string) (*types.ChartUpgradeAction, error) {
	chartValues, err := client.GetChartValues(repoName, vsphereCSIchartName, chartUpgradeActionPayload.InstallOptions.Version)
	if err != nil {
		return nil, err
	}

	chartValues[vcenter].(map[string]interface{})[datacenters] = vsphereTemplateConfig.Datacenter
	chartValues[vcenter].(map[string]interface{})[host] = vsphereTemplateConfig.Vcenter
	chartValues[vcenter].(map[string]interface{})[password] = vsphereTemplateConfig.Password
	chartValues[vcenter].(map[string]interface{})[username] = vsphereTemplateConfig.Username
	chartValues[vcenter].(map[string]interface{})[port] = vsphereTemplateConfig.VcenterPort
	chartValues[vcenter].(map[string]interface{})[clusterid] = chartUpgradeActionPayload.InstallOptions.Cluster.ID

	chartValues[storageclass].(map[string]interface{})[datastoreurl] = vsphereTemplateConfig.DatastoreURL

	chartUpgrade := newChartUpgrade(
		vsphereCSIchartName,
		chartUpgradeActionPayload.Name,
		chartUpgradeActionPayload.Version,
		chartUpgradeActionPayload.Cluster.ID,
		chartUpgradeActionPayload.Cluster.Name,
		repoName,
		chartUpgradeActionPayload.DefaultRegistry,
		chartValues)
	chartUpgrades := []types.ChartUpgrade{*chartUpgrade}

	return newChartUpgradeAction(chartNamespace, chartUpgrades), nil
}

// getExistingChartInstall returns the App object of an installed chart.
func getExistingChartInstall(client *catalog.Client, chartName, chartNamespace string) (*v1.App, error) {
	appObject, err := client.Apps(chartNamespace).Get(context.TODO(), chartName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return appObject, nil
}
