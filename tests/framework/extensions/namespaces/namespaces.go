package namespaces

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/pkg/api/scheme"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/unstructured"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
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

// CreateNamespace is a helper function that uses the dynamic client to create a namespace on a project.
// It registers a delete fuction with a wait.WatchWait to ensure the namspace is deleted cleanly.
func CreateNamespace(client *rancher.Client, namespaceName, containerDefaultResourceLimit string, labels, annotations map[string]string, project *management.Project) (*coreV1.Namespace, error) {
	// Namespace object for a project name space
	annotations["field.cattle.io/containerDefaultResourceLimit"] = containerDefaultResourceLimit
	annotations["field.cattle.io/projectId"] = project.ID
	namespace := &coreV1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        namespaceName,
			Annotations: annotations,
			Labels:      labels,
		},
	}

	dynamicClient, err := client.GetDownStreamClusterClient(project.ClusterID)
	if err != nil {
		return nil, err
	}

	namespaceResource := dynamicClient.Resource(NamespaceGroupVersionResource).Namespace("")

	unstructuredResp, err := namespaceResource.Create(context.TODO(), unstructured.MustToUnstructured(namespace), metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	client.Session.RegisterCleanupFunc(func() error {
		err := namespaceResource.Delete(context.TODO(), unstructuredResp.GetName(), metav1.DeleteOptions{})
		if errors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}

		watchInterface, err := namespaceResource.Watch(context.TODO(), metav1.ListOptions{
			FieldSelector:  "metadata.name=" + unstructuredResp.GetName(),
			TimeoutSeconds: &defaults.WatchTimeoutSeconds,
		})

		if err != nil {
			return err
		}

		return wait.WatchWait(watchInterface, func(event watch.Event) (ready bool, err error) {
			if event.Type == watch.Deleted {
				return true, nil
			}
			return false, nil
		})
	})

	newNamespace := &coreV1.Namespace{}
	err = scheme.Scheme.Convert(unstructuredResp, newNamespace, unstructuredResp.GroupVersionKind())
	if err != nil {
		return nil, err
	}
	return newNamespace, nil
}
