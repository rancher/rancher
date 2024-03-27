package configmaps

import (
	v1 "github.com/rancher/shepherd/clients/rancher/v1"

	"github.com/rancher/shepherd/extensions/kubeapi/configmaps"
)

const (
	namespace = "default"
	labelKey  = "label1"
	labelVal  = "autoLabel"
	dataKey   = "foo"
	dataVal   = "bar"
	annoKey   = "anno1"
	annoVal   = "automated annotation"
	descKey   = "field.cattle.io/description"
	descVal   = "automated configmap description"
	cmName    = "steve-configmap"
)

func createConfigmap(client v1.SteveClient, name, namespace string, annotations, labels, data map[string]string) (v1.SteveAPIObject, error) {
	configmapTemplate := configmaps.NewConfigmapTemplate(name, namespace, annotations, labels, data)
	configMapObj, err := client.Create(configmapTemplate)

	return *configMapObj, err
}

func updateConfigmapAnnotations(configMapClient v1.SteveClient, configmapObj v1.SteveAPIObject, newAnnotations map[string]string) (*v1.SteveAPIObject, error) {
	newConfigmap := configmapObj
	newConfigmap.ObjectMeta.Annotations = newAnnotations
	updatedConfigMapObj, err := configMapClient.Update(&configmapObj, newConfigmap)

	return updatedConfigMapObj, err
}

func getConfigMapLabelsAndAnnotations(actualResources map[string]string) map[string]string {
	expectedResources := map[string]string{}

	for resource := range actualResources {
		if _, found := actualResources[resource]; found {
			expectedResources[resource] = actualResources[resource]
		}
	}

	return expectedResources
}
