package clusteregistrationtokens

import (
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/controllers/management/clusterprovisioner"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
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
	resource.AddAction(request, "generateKubeconfig")
	resource.AddAction(request, "importYaml")
	resource.AddAction(request, "exportYaml")

	f.transposeGenericConfigToDynamicField(resource.Values)

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
	}
}

func (f *Formatter) transposeGenericConfigToDynamicField(data map[string]interface{}) {
	if genericEngineConfig, ok := data["genericEngineConfig"]; ok {
		if configMap, ok := genericEngineConfig.(map[string]interface{}); ok {
			drivers, err := f.KontainerDriverLister.List("", labels.Everything())
			if err != nil {
				logrus.Warnf("failed to get kontainer drivers in formatter: %v", err)
				return
			}

			var driver *v3.KontainerDriver
			if driverName, ok := configMap[clusterprovisioner.DriverNameField]; ok {
				for _, candidate := range drivers {
					if driverName == candidate.Name {
						driver = candidate
						break
					}
				}

				if driver == nil {
					logrus.Warnf("got unknown driver in formatter: %v", data[clusterprovisioner.DriverNameField])
					return
				}

				var driverNameField string
				if driver.Spec.BuiltIn {
					driverNameField = driver.Status.DisplayName + "Config"
				} else {
					driverNameField = driver.Status.DisplayName + "EngineConfig"
				}

				data[driverNameField] = data["genericEngineConfig"]
				delete(data, "genericEngineConfig")
			}
		} else {
			logrus.Warnf("failed to convert generic engine config to map[string]string")
		}
	}
}

func setTrueIfNil(configMap map[string]interface{}, fieldName string) {
	if configMap[fieldName] == nil {
		configMap[fieldName] = true
	}
}
