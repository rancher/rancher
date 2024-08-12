package services

import (
	"context"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/unstructured"
	"github.com/rancher/shepherd/pkg/api/scheme"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ServiceGroupVersionResource is the required Group Version Resource for accessing services in a cluster,
// using the dynamic client.
var ServiceGroupVersionResource = schema.GroupVersionResource{
	Group:    "",
	Version:  "v1",
	Resource: "services",
}

// CreateService is a helper function that uses the dynamic client to create a service in a namespace for a specific cluster.
func CreateService(client *rancher.Client, clusterName, serviceName, namespace string, spec corev1.ServiceSpec) (*corev1.Service, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterName)
	if err != nil {
		return nil, err
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
		},
		Spec: spec,
	}

	serviceResource := dynamicClient.Resource(ServiceGroupVersionResource).Namespace(namespace)

	unstructuredResp, err := serviceResource.Create(context.TODO(), unstructured.MustToUnstructured(service), metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	newService := &corev1.Service{}
	err = scheme.Scheme.Convert(unstructuredResp, newService, unstructuredResp.GroupVersionKind())
	if err != nil {
		return nil, err
	}

	return newService, nil
}
