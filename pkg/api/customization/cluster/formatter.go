package cluster

import (
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

type Formatter struct {
	KontainerDriverLister v3.KontainerDriverLister
}

func (f *Formatter) Formatter(request *types.APIContext, resource *types.RawResource) {
	if convert.ToBool(resource.Values["internal"]) {
		delete(resource.Links, "remove")
	}
	shellLink := request.URLBuilder.Link("shell", resource)
	shellLink = strings.Replace(shellLink, "http", "ws", 1)
	shellLink = strings.Replace(shellLink, "/shell", "?shell=true", 1)
	resource.Links["shell"] = shellLink
	resource.AddAction(request, v3.ClusterActionGenerateKubeconfig)
	resource.AddAction(request, v3.ClusterActionImportYaml)
	resource.AddAction(request, v3.ClusterActionExportYaml)
	if _, ok := resource.Values["rancherKubernetesEngineConfig"]; ok {
		resource.AddAction(request, v3.ClusterActionRotateCertificates)
		if _, ok := values.GetValue(resource.Values, "rancherKubernetesEngineConfig", "services", "etcd", "backupConfig"); ok {
			resource.AddAction(request, v3.ClusterActionBackupEtcd)
			resource.AddAction(request, v3.ClusterActionRestoreFromEtcdBackup)
		}
	}

	if err := request.AccessControl.CanDo(v3.ClusterGroupVersionKind.Group, v3.ClusterResource.Name, "update", request, resource.Values, request.Schema); err == nil {
		if convert.ToBool(resource.Values["enableClusterMonitoring"]) {
			resource.AddAction(request, v3.ClusterActionDisableMonitoring)
			resource.AddAction(request, v3.ClusterActionEditMonitoring)
		} else {
			resource.AddAction(request, v3.ClusterActionEnableMonitoring)
		}
	}

	if convert.ToBool(resource.Values["enableClusterMonitoring"]) {
		resource.AddAction(request, v3.ClusterActionViewMonitoring)
	}

	if gkeConfig, ok := resource.Values["googleKubernetesEngineConfig"]; ok {
		configMap, ok := gkeConfig.(map[string]interface{})
		if !ok {
			logrus.Errorf("could not convert gke config to map")
			return
		}

		setTrueIfNil(configMap, "enableStackdriverLogging")
		setTrueIfNil(configMap, "enableStackdriverMonitoring")
		setTrueIfNil(configMap, "enableHorizontalPodAutoscaling")
		setTrueIfNil(configMap, "enableHttpLoadBalancing")
		setTrueIfNil(configMap, "enableNetworkPolicyConfig")
	}

	if eksConfig, ok := resource.Values["amazonElasticContainerServiceConfig"]; ok {
		configMap, ok := eksConfig.(map[string]interface{})
		if !ok {
			logrus.Errorf("could not convert eks config to map")
			return
		}

		setTrueIfNil(configMap, "associateWorkerNodePublicIp")
		setIntIfNil(configMap, "nodeVolumeSize", 20)
	}
}

func setTrueIfNil(configMap map[string]interface{}, fieldName string) {
	if configMap[fieldName] == nil {
		configMap[fieldName] = true
	}
}

func setIntIfNil(configMap map[string]interface{}, fieldName string, replaceVal int) {
	if configMap[fieldName] == nil {
		configMap[fieldName] = replaceVal
	}
}
