package configmaps

import (
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"

	"github.com/rancher/rancher/tests/framework/extensions/kubeapi/storage"
	"github.com/rancher/rancher/tests/framework/pkg/namegenerator"
)

var(
	configmapName = namegenerator.AppendRandomString(("steve-configmap"))
	annotations = map[string]string{"anno1": "automated annotation", "field.cattle.io/description": "automated configmap description"}
	updatedAnnotations = map[string]string{"anno2": "new automated annotation", "field.cattle.io/description": "new automated configmap description"}
	labels = map[string]string{"label1": "autoLabel"}
	data = map[string]string{"foo": "bar"}
  )

  const namespace string = "default"

// createConfigmap is a helper function that creates a configmap and returns the JSON/k8s api object
func createConfigmap(client v1.SteveClient, name string, namespace string, annotations map[string]string, labels map[string]string, data map[string]string) v1.SteveAPIObject {
	// Uses configmapTemplate to create a new configmap
	configmapTemplate := storage.NewConfigmapTemplate(name, namespace, annotations, labels, data)
	configMapObj, _ := client.Create(configmapTemplate)

	return *configMapObj
}

// updateConfigmap is a helper function to update an existing configmap and return the updated JSON/k8s api configmap object
func updateConfigmapAnnotations(configMapClient v1.SteveClient, configmapObj v1.SteveAPIObject, newAnnotations map[string]string) (*v1.SteveAPIObject, error) {
	// Update action
	newConfigmap := configmapObj
	newConfigmap.ObjectMeta.Annotations = newAnnotations
	updatedConfigMapObj, err := configMapClient.Update(&configmapObj, newConfigmap)
	return updatedConfigMapObj, err
}

// deleteConfigmap is a helper function to delete an existing configmap
func deleteConfigmap(configMapClient v1.SteveClient, configmapObj v1.SteveAPIObject) error {
	// delete
    configmapByID, _ := configMapClient.ByID(configmapObj.ID)
    delErr := configMapClient.Delete(configmapByID)
	return delErr
}