package rbac

import (
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	typesrbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type crHandler struct {
	clusterRoles       typesrbacv1.ClusterRoleInterface
	roleTemplateLister v3.RoleTemplateLister
}

func newClusterRoleHandler(r *manager) *crHandler {
	return &crHandler{
		clusterRoles:       r.clusterRoles,
		roleTemplateLister: r.rtLister,
	}
}

// sync validates that a clusterRole's parent roleTemplate still exists in management
// and will remove the clusterRole if the roleTemplate no longer exists.
func (c *crHandler) sync(key string, obj *rbacv1.ClusterRole) (runtime.Object, error) {
	if key == "" || obj == nil {
		return nil, nil
	}

	if owner, ok := obj.Annotations[clusterRoleOwner]; ok {
		_, err := c.roleTemplateLister.Get("", owner)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return obj, c.clusterRoles.Delete(obj.Name, &metav1.DeleteOptions{})
			}
			return obj, err
		}
	}

	return obj, nil
}
