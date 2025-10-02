package rbac

import (
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	wrbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type crHandler struct {
	clusterRoles       wrbacv1.ClusterRoleClient
	roleTemplateLister mgmtv3.RoleTemplateCache
}

func newClusterRoleHandler(r *manager) *crHandler {
	return &crHandler{
		clusterRoles:       r.clusterRoles,
		roleTemplateLister: r.rtLister,
	}
}

// sync validates that a clusterRole's parent roleTemplate still exists in management
// and will remove the clusterRole if the roleTemplate no longer exists.
func (c *crHandler) sync(key string, obj *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
	if key == "" || obj == nil {
		return nil, nil
	}

	if owner, ok := obj.Annotations[clusterRoleOwner]; ok {
		_, err := c.roleTemplateLister.Get(owner)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return obj, c.clusterRoles.Delete(obj.Name, &metav1.DeleteOptions{})
			}
			return obj, err
		}
	}

	return obj, nil
}
