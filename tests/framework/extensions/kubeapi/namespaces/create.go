package namespaces

import (
	"context"
	"fmt"
	"strings"

	"github.com/rancher/rancher/pkg/api/scheme"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/defaults"
	"github.com/rancher/rancher/tests/framework/extensions/unstructured"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	coreV1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeUnstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
)

// CreateNamespace is a helper function that uses the dynamic client to create a namespace on a project.
// It registers a delete function with a wait.WatchWait to ensure the namspace is deleted cleanly.
func CreateNamespace(client *rancher.Client, namespaceName, containerDefaultResourceLimit string, labels, annotations map[string]string, project *management.Project) (*coreV1.Namespace, error) {
	// Namespace object for a project name space
	if annotations == nil {
		annotations = make(map[string]string)
	}
	if containerDefaultResourceLimit != "" {
		annotations["field.cattle.io/containerDefaultResourceLimit"] = containerDefaultResourceLimit
	}
	if project != nil {
		annotations["field.cattle.io/projectId"] = project.ID
	}
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

	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	if err != nil {
		return nil, err
	}

	adminDynamicClient, err := adminClient.GetDownStreamClusterClient(project.ClusterID)
	if err != nil {
		return nil, err
	}

	namespaceResource := dynamicClient.Resource(NamespaceGroupVersionResource).Namespace("")

	unstructuredResp, err := namespaceResource.Create(context.TODO(), unstructured.MustToUnstructured(namespace), metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	clusterRoleResource := adminDynamicClient.Resource(rbacv1.SchemeGroupVersion.WithResource("clusterroles"))
	projectID := strings.Split(project.ID, ":")[1]

	clusterRoleWatch, err := clusterRoleResource.Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + fmt.Sprintf("%s-namespaces-edit", projectID),
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})

	if err != nil {
		return nil, err
	}

	err = wait.WatchWait(clusterRoleWatch, func(event watch.Event) (ready bool, err error) {
		clusterRole := &rbacv1.ClusterRole{}
		err = scheme.Scheme.Convert(event.Object.(*kubeUnstructured.Unstructured), clusterRole, event.Object.(*kubeUnstructured.Unstructured).GroupVersionKind())

		if err != nil {
			return false, err
		}

		for _, rule := range clusterRole.Rules {
			for _, resourceName := range rule.ResourceNames {
				if resourceName == namespaceName {
					return true, nil
				}
			}
		}
		return false, nil
	})

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

		adminNamespaceResource := adminDynamicClient.Resource(NamespaceGroupVersionResource).Namespace("")
		watchInterface, err := adminNamespaceResource.Watch(context.TODO(), metav1.ListOptions{
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
