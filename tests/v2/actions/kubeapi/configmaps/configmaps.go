package configmaps

import (
	"context"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/unstructured"
	"github.com/rancher/shepherd/pkg/api/scheme"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ConfigMapGroupVersionResource is the required Group Version Resource for accessing config maps in a cluster,
// using the dynamic client.
var ConfigMapGroupVersionResource = schema.GroupVersionResource{
	Group:    "",
	Version:  "v1",
	Resource: "configmaps",
}

// NewConfigmapTemplate is a constructor that creates a configmap template
func NewConfigmapTemplate(configmapName, namespace string, annotations, labels, data map[string]string) coreV1.ConfigMap {
	if annotations == nil {
		annotations = make(map[string]string)
	}
	if labels == nil {
		labels = make(map[string]string)
	}
	if data == nil {
		data = make(map[string]string)
	}

	return coreV1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        configmapName,
			Namespace:   namespace,
			Annotations: annotations,
			Labels:      labels,
		},
		Data: data,
	}
}

// CreateConfigMap is a helper function that uses the dynamic client to create a config map on a namespace for a specific cluster.
// It registers a delete fuction.
func CreateConfigMap(client *rancher.Client, clusterName, configMapName, description, namespace string, data, labels, annotations map[string]string) (*coreV1.ConfigMap, error) {
	// ConfigMap object for a namespace in a cluster
	annotations["field.cattle.io/description"] = description
	configMap := &coreV1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        configMapName,
			Annotations: annotations,
			Namespace:   namespace,
			Labels:      labels,
		},
		Data: data,
	}

	dynamicClient, err := client.GetDownStreamClusterClient(clusterName)
	if err != nil {
		return nil, err
	}

	configMapResource := dynamicClient.Resource(ConfigMapGroupVersionResource).Namespace(namespace)

	unstructuredResp, err := configMapResource.Create(context.TODO(), unstructured.MustToUnstructured(configMap), metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	newConfig := &coreV1.ConfigMap{}
	err = scheme.Scheme.Convert(unstructuredResp, newConfig, unstructuredResp.GroupVersionKind())
	if err != nil {
		return nil, err
	}
	return newConfig, nil
}
