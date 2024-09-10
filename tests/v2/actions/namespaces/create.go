package namespaces

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rancher/rancher/tests/v2/actions/kubeapi/namespaces"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/pkg/api/scheme"
	"github.com/rancher/shepherd/pkg/wait"
	coreV1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeUnstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	NamespaceSteveType = "namespace"
)

// CreateNamespace is a helper function that uses the dynamic client to create a namespace on a project.
// It registers a delete function with a wait.WatchWait to ensure the namspace is deleted cleanly.
func CreateNamespace(client *rancher.Client, namespaceName, containerDefaultResourceLimit string, labels, annotations map[string]string, project *management.Project) (*v1.SteveAPIObject, error) {
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
	err = kwait.Poll(300*time.Millisecond, 3*time.Minute, func() (done bool, err error) {
		namespaceStatus := &coreV1.NamespaceStatus{}
		err = v1.ConvertToK8sType(resp.Status, namespaceStatus)
		if err != nil {
			return false, err
		}
		if namespaceStatus.Phase == "Active" {
			return true, nil
		}
		return false, nil
	})
	return resp, nil
}
