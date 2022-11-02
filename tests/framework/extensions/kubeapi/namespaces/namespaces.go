package namespaces

import (
	"fmt"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
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
	var namespace *corev1.Namespace

	namespaceList, err := ListNamespaces(client, clusterID, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for i, ns := range namespaceList.Items {
		if namespaceName == ns.Name {
			namespace = &namespaceList.Items[i]
			break
		}
	}

	return namespace, nil
}
