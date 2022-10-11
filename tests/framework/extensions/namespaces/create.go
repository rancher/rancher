package namespaces

import (
	"context"
	"fmt"
	"strings"

	"github.com/rancher/rancher/pkg/api/scheme"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/kubeapi/namespaces"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	coreV1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeUnstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	NamespaceSteveType = "namespace"
)

// CreateNamespace is a helper function that uses the dynamic client to create a namespace on a project.
// It registers a delete function with a wait.WatchWait to ensure the namspace is deleted cleanly.
func CreateNamespace(client *rancher.Client, namespaceName, containerDefaultResourceLimit string, labels, annotations map[string]string, project *management.Project) (*v1.SteveAPIObject, error) {
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

	steveClient, err := client.Steve.ProxyDownstream(project.ClusterID)
	if err != nil {
		return nil, err
	}

	nameSpaceClient := steveClient.SteveType(NamespaceSteveType)

	resp, err := nameSpaceClient.Create(namespace)
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
		steveClient, err = client.Steve.ProxyDownstream(project.ClusterID)
		if err != nil {
			return err
		}

		nameSpaceClient = steveClient.SteveType(NamespaceSteveType)
		err := nameSpaceClient.Delete(resp)
		if errors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}

		adminNamespaceResource := adminDynamicClient.Resource(namespaces.NamespaceGroupVersionResource).Namespace("")
		watchInterface, err := adminNamespaceResource.Watch(context.TODO(), metav1.ListOptions{
			FieldSelector:  "metadata.name=" + resp.Name,
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

	return resp, nil
}
