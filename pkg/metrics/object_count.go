package metrics

import (
	"context"

	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	managementv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	projectv3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	rbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type clusterObjectCounter interface {
	ConfigMaps(context.Context) (int64, error)
	Namespaces(context.Context) (int64, error)
	Nodes(context.Context) (int64, error)
	Secrets(context.Context) (int64, error)
	ClusterRoleBindings(context.Context) (int64, error)
	RoleBindings(context.Context) (int64, error)
	AppRevisions(context.Context) (int64, error)
	CatalogTemplateVersions(context.Context) (int64, error)
	Projects(context.Context) (int64, error)
}

type clusterCountClient struct {
	// There are *complete* caches only for some kinds, use that when available (see pkg/controllers/managementuser/controllers.go).

	core interface {
		corev1.NamespacesGetter
	}
	rbac interface {
		rbacv1.ClusterRoleBindingsGetter
		rbacv1.RoleBindingsGetter
	}

	// Otherwise, use a client directly.

	dynamic dynamic.Interface
}

func (h *metricsHandler) getClusterClient(clusterID string) (clusterObjectCounter, error) {
	cluster, err := h.clusterManager.UserContext(clusterID)
	if err != nil {
		return nil, err
	}
	return &clusterCountClient{
		dynamic: dynamic.New(cluster.UnversionedClient),
		core:    cluster.Core,
		rbac:    cluster.RBAC,
	}, nil
}

func (c clusterCountClient) countAllObjects(ctx context.Context, gvr schema.GroupVersionResource) (int64, error) {
	// Starting on Kubernetes 1.16, ListMeta includes a RemainingItemCount field, with an estimation of the number of items that couldn't fit in the request
	// Use that to obtain the number of objects without having to perform all the calls.
	list, err := c.dynamic.Resource(gvr).List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return 0, nil
		}
		return 0, err
	}
	n := int64(len(list.Items))
	if remaining := list.GetRemainingItemCount(); remaining != nil {
		n += *remaining
	}
	return n, nil
}

func (c clusterCountClient) Namespaces(_ context.Context) (int64, error) {
	list, err := c.core.Namespaces(metav1.NamespaceAll).Controller().Lister().List(metav1.NamespaceAll, labels.Everything())
	return int64(len(list)), err
}

func (c clusterCountClient) ClusterRoleBindings(_ context.Context) (int64, error) {
	list, err := c.rbac.ClusterRoleBindings(metav1.NamespaceAll).Controller().Lister().List(metav1.NamespaceAll, labels.Everything())
	return int64(len(list)), err
}

func (c clusterCountClient) RoleBindings(_ context.Context) (int64, error) {
	list, err := c.rbac.RoleBindings(metav1.NamespaceAll).Controller().Lister().List(metav1.NamespaceAll, labels.Everything())
	return int64(len(list)), err
}

func (c clusterCountClient) ConfigMaps(ctx context.Context) (int64, error) {
	return c.countAllObjects(ctx, corev1.ConfigMapGroupVersionResource)
}

func (c clusterCountClient) Nodes(ctx context.Context) (int64, error) {
	return c.countAllObjects(ctx, corev1.NodeGroupVersionResource)
}

func (c clusterCountClient) Secrets(ctx context.Context) (int64, error) {
	return c.countAllObjects(ctx, corev1.SecretGroupVersionResource)
}

func (c clusterCountClient) AppRevisions(ctx context.Context) (int64, error) {
	return c.countAllObjects(ctx, projectv3.AppRevisionGroupVersionResource)
}

func (c clusterCountClient) CatalogTemplateVersions(ctx context.Context) (int64, error) {
	return c.countAllObjects(ctx, managementv3.CatalogTemplateVersionGroupVersionResource)
}

func (c clusterCountClient) Projects(ctx context.Context) (int64, error) {
	return c.countAllObjects(ctx, managementv3.ProjectGroupVersionResource)
}
