package rbac

import (
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	wrbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type crHandler struct {
	clusterRoles       wrbacv1.ClusterRoleClient
	roleTemplateLister mgmtv3.RoleTemplateCache
	clusterName        string
}

func newClusterRoleHandler(r *manager) *crHandler {
	return &crHandler{
		clusterRoles:       r.clusterRoles,
		roleTemplateLister: r.rtLister,
		clusterName:        r.clusterName,
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
				logrus.Tracef("[cluster-clusterrole-sync] installUUID=%s cluster=%s: roleTemplate %q owning clusterRole %q not found locally, deleting clusterRole",
					settings.InstallUUID.Get(), c.clusterName, owner, obj.Name)
				return obj, c.clusterRoles.Delete(obj.Name, &metav1.DeleteOptions{})
			}
			return obj, err
		}
	}

	return obj, nil
}
