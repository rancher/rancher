package namespaces

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

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

// NewProjectNamespace creates an *unstructured.Unstructured object to be used by the GetDownStreamK8Client to create a project namespace
func NewProjectNamespace(namespaceName, projectID, containerDefaultResourceLimit string, labels, annotations map[string]string) *unstructured.Unstructured {
	annotations["field.cattle.io/containerDefaultResourceLimit"] = containerDefaultResourceLimit
	annotations["field.cattle.io/projectId"] = projectID

	namespace := &unstructured.Unstructured{}
	namespace.SetAPIVersion("v1")
	namespace.SetKind("Namespace")
	namespace.SetName(namespaceName)
	namespace.SetLabels(labels)
	namespace.SetAnnotations(annotations)
	namespace.Object["type"] = "namespace"

	return namespace
}
