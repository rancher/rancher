package namespaces

import (
	"context"
	"fmt"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/api/scheme"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// NamespaceGroupVersionResource is the required Group Version Resource for accessing namespaces in a cluster,
// using the dynamic client.
var NamespaceGroupVersionResource = schema.GroupVersionResource{
	Group:    "",
	Version:  "v1",
	Resource: "namespaces",
}

// ContainerDefaultResourceLimit sets the container default resource limit in a string
// limitsCPU and requestsCPU in form of "3m"
// limitsMemory and requestsMemory in the form of "3Mi"
func ContainerDefaultResourceLimit(limitsCPU, limitsMemory, requestsCPU, requestsMemory string) string {
	containerDefaultResourceLimit := fmt.Sprintf("{\"limitsCpu\": \"%s\", \"limitsMemory\":\"%s\",\"requestsCpu\":\"%s\",\"requestsMemory\":\"%s\"}",
		limitsCPU, limitsMemory, requestsCPU, requestsMemory)
	return containerDefaultResourceLimit
}

// GetNamespaceByName is a helper function that returns the namespace by name in a specific cluster, uses ListNamespaces to get the namespace.
func GetNamespaceByName(client *rancher.Client, clusterID, namespaceName string) (*corev1.Namespace, error) {
	namespace := new(corev1.Namespace)

	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return nil, err
	}

	namespaceResource := dynamicClient.Resource(NamespaceGroupVersionResource).Namespace("")
	unstructuredNamespace, err := namespaceResource.Get(context.TODO(), namespaceName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if err = scheme.Scheme.Convert(unstructuredNamespace, namespace, unstructuredNamespace.GroupVersionKind()); err != nil {
		return nil, err
	}

	return namespace, nil
}
