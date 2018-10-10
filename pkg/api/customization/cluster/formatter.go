package clusteregistrationtokens

import (
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/types/client/management/v3"
	"github.com/sirupsen/logrus"
)

func Formatter(request *types.APIContext, resource *types.RawResource) {
	if convert.ToBool(resource.Values["internal"]) {
		delete(resource.Links, "remove")
	}
	shellLink := request.URLBuilder.Link("shell", resource)
	shellLink = strings.Replace(shellLink, "http", "ws", 1)
	shellLink = strings.Replace(shellLink, "/shell", "?shell=true", 1)
	resource.Links["shell"] = shellLink
	resource.AddAction(request, "generateKubeconfig")
	resource.AddAction(request, "importYaml")
	resource.AddAction(request, "exportYaml")

	if gkeConfig, ok := resource.Values[client.ClusterSpecFieldGoogleKubernetesEngineConfig]; ok {
		configMap, ok := gkeConfig.(map[string]interface{})
		if !ok {
			logrus.Errorf("could not convert gke config to map")
			return
		}

		setTrueIfNil(configMap, client.GoogleKubernetesEngineConfigFieldEnableStackdriverLogging)
		setTrueIfNil(configMap, client.GoogleKubernetesEngineConfigFieldEnableStackdriverMonitoring)
		setTrueIfNil(configMap, client.GoogleKubernetesEngineConfigFieldEnableHorizontalPodAutoscaling)
		setTrueIfNil(configMap, client.GoogleKubernetesEngineConfigFieldEnableHTTPLoadBalancing)
		setTrueIfNil(configMap, client.GoogleKubernetesEngineConfigFieldEnableNetworkPolicyConfig)
	}

	if eksConfig, ok := resource.Values[client.ClusterSpecFieldAmazonElasticContainerServiceConfig]; ok {
		configMap, ok := eksConfig.(map[string]interface{})
		if !ok {
			logrus.Errorf("could not convert aks config to map")
			return
		}

		setTrueIfNil(configMap, client.AmazonElasticContainerServiceConfigFieldAssociateWorkerNodePublicIP)
	}
}

func setTrueIfNil(configMap map[string]interface{}, fieldName string) {
	if configMap[fieldName] == nil {
		configMap[fieldName] = true
	}
}
