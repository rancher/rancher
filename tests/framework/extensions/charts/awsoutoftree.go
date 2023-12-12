package charts

import (
	"time"

	"github.com/rancher/rancher/pkg/api/steve/catalog/types"

	appv1 "k8s.io/api/apps/v1"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	steveV1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	repoType                     = "catalog.cattle.io.clusterrepo"
	appsType                     = "catalog.cattle.io.apps"
	awsUpstreamCloudProviderRepo = "https://github.com/kubernetes/cloud-provider-aws.git"
	masterBranch                 = "master"
	AwsUpstreamChartName         = "aws-cloud-controller-manager"
	kubeSystemNamespace          = "kube-system"
)

// InstallAWSOutOfTreeChart installs the CSI chart for aws cloud provider in a given cluster.
func InstallAWSOutOfTreeChart(client *rancher.Client, installOptions *InstallOptions, repoName, clusterID string) error {
	serverSetting, err := client.Management.Setting.ByID(serverURLSettingID)
	if err != nil {
		return err
	}

	registrySetting, err := client.Management.Setting.ByID(defaultRegistrySettingID)
	if err != nil {
		return err
	}

	awsChartInstallActionPayload := &payloadOpts{
		InstallOptions:  *installOptions,
		Name:            AwsUpstreamChartName,
		Namespace:       kubeSystemNamespace,
		Host:            serverSetting.Value,
		DefaultRegistry: registrySetting.Value,
	}

	chartInstallAction := awsChartInstallAction(awsChartInstallActionPayload, repoName, kubeSystemNamespace, installOptions.ProjectID)

	catalogClient, err := client.GetClusterCatalogClient(installOptions.ClusterID)
	if err != nil {
		return err
	}

	err = catalogClient.InstallChart(chartInstallAction, repoName)
	if err != nil {
		return err
	}

	err = VerifyChartInstall(catalogClient, kubeSystemNamespace, AwsUpstreamChartName)
	if err != nil {
		return err
	}

	steveclient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return err
	}

	chartNodeSelector := map[string]string{
		"node-role.kubernetes.io/controlplane": "true",
	}
	err = updateHelmNodeSelectors(steveclient, kubeSystemNamespace, AwsUpstreamChartName, chartNodeSelector)

	return err
}

// awsChartInstallAction is a helper function that returns a chartInstallAction for aws out-of-tree chart.
func awsChartInstallAction(awsChartInstallActionPayload *payloadOpts, repoName, chartNamespace, chartProject string) *types.ChartInstallAction {
	chartValues := map[string]interface{}{
		"args": []interface{}{
			"--use-service-account-credentials=true",
			"--configure-cloud-routes=false",
			"--v=2",
			"--cloud-provider=aws",
		},
		// note: order of []interface{} must match the chart's order. A union is taken in the order given (not a pure replacement of the object)
		"clusterRoleRules": []interface{}{
			map[string]interface{}{
				"apiGroups": []interface{}{""},
				"resources": []interface{}{
					"events",
				},
				"verbs": []interface{}{
					"patch",
					"create",
					"update",
				},
			},
			map[string]interface{}{
				"apiGroups": []interface{}{""},
				"resources": []interface{}{
					"nodes",
				},
				"verbs": []interface{}{
					"*",
				},
			},
			map[string]interface{}{
				"apiGroups": []interface{}{""},
				"resources": []interface{}{
					"nodes/status",
				},
				"verbs": []interface{}{
					"patch",
				},
			},
			map[string]interface{}{
				"apiGroups": []interface{}{""},
				"resources": []interface{}{
					"services",
				},
				"verbs": []interface{}{
					"list",
					"patch",
					"update",
					"watch",
				},
			},
			map[string]interface{}{
				"apiGroups": []interface{}{""},
				"resources": []interface{}{
					"services/status",
				},
				"verbs": []interface{}{
					"list",
					"patch",
					"update",
					"watch",
				},
			},
			map[string]interface{}{
				"apiGroups": []interface{}{""},
				"resources": []interface{}{
					"serviceaccounts",
				},
				"verbs": []interface{}{
					"get",
					"create",
				},
			},
			map[string]interface{}{
				"apiGroups": []interface{}{""},
				"resources": []interface{}{
					"persistentvolumes",
				},
				"verbs": []interface{}{
					"get",
					"list",
					"update",
					"watch",
				},
			},
			map[string]interface{}{
				"apiGroups": []interface{}{""},
				"resources": []interface{}{
					"endpoints",
				},
				"verbs": []interface{}{
					"get",
					"create",
					"list",
					"watch",
					"update",
				},
			},
			map[string]interface{}{
				"apiGroups": []interface{}{
					"coordination.k8s.io",
				},
				"resources": []interface{}{
					"leases",
				},
				"verbs": []interface{}{
					"get",
					"create",
					"list",
					"watch",
					"update",
				},
			},
			map[string]interface{}{
				"apiGroups": []interface{}{""},
				"resources": []interface{}{
					"serviceaccounts/token",
				},
				"verbs": []interface{}{
					"create",
				},
			},
		},
		"nodeSelector": map[string]interface{}{
			"node-role.kubernetes.io/controlplane": "true",
		},
		"tolerations": []interface{}{
			map[string]interface{}{
				"effect": "NoSchedule",
				"value":  "true",
				"key":    "node-role.kubernetes.io/controlplane",
			},
			map[string]interface{}{
				"effect": "NoSchedule",
				"value":  "true",
				"key":    "node.cloudprovider.kubernetes.io/uninitialized",
			},
			map[string]interface{}{
				"effect": "NoSchedule",
				"value":  "true",
				"key":    "node-role.kubernetes.io/master",
			},
		},
	}

	chartInstall := newChartInstall(
		awsChartInstallActionPayload.Name,
		awsChartInstallActionPayload.InstallOptions.Version,
		awsChartInstallActionPayload.InstallOptions.ClusterID,
		awsChartInstallActionPayload.InstallOptions.ClusterName,
		awsChartInstallActionPayload.Host,
		repoName,
		chartProject,
		awsChartInstallActionPayload.DefaultRegistry,
		chartValues)
	chartInstalls := []types.ChartInstall{*chartInstall}

	return newChartInstallAction(chartNamespace, awsChartInstallActionPayload.ProjectID, chartInstalls)
}

// updateHelmNodeSelectors is a function that updates the newNodeSelector for a given Daemonset's nodeSelector. This is required due to an
// upstream bug in helm charts, where you can't override the nodeSelector during a deployment of an upstream chart.
func updateHelmNodeSelectors(client *steveV1.Client, daemonsetNamespace, daemonsetName string, newNodeSelector map[string]string) error {
	err := kwait.Poll(1*time.Second, 1*time.Minute, func() (done bool, err error) {
		_, err = client.SteveType(pods.DaemonsetSteveType).ByID(daemonsetNamespace + "/" + daemonsetName)
		if err != nil {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return err
	}

	steveDaemonset, err := client.SteveType(pods.DaemonsetSteveType).ByID(daemonsetNamespace + "/" + daemonsetName)
	if err != nil {
		return err
	}

	daemonsetObject := new(appv1.DaemonSet)
	err = steveV1.ConvertToK8sType(steveDaemonset, &daemonsetObject)
	if err != nil {
		return err
	}

	daemonsetObject.Spec.Template.Spec.NodeSelector = newNodeSelector

	_, err = client.SteveType(pods.DaemonsetSteveType).Update(steveDaemonset, daemonsetObject)
	return err
}
