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
//
// In a nested Rancher setup, the same physical cluster can be managed as a downstream cluster by one
// Rancher install while also being the local cluster of another. A ClusterRole created by a different
// install (identified by the clusterRoleOwnerInstallUUID annotation) is not in this install's
// RoleTemplate management plane and must be left alone, even if the owning RoleTemplate can't be
// found locally.
func (c *crHandler) sync(key string, obj *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
	if key == "" || obj == nil {
		return nil, nil
	}

	owner, ok := obj.Annotations[clusterRoleOwner]
	if !ok {
		return obj, nil
	}

	installUUID := settings.InstallUUID.Get()
	if ownerInstallUUID, ok := obj.Annotations[clusterRoleOwnerInstallUUID]; ok && ownerInstallUUID != installUUID {
		logrus.Tracef("[cluster-clusterrole-sync] installUUID=%s cluster=%s: clusterRole %q is owned by roleTemplate %q from a different Rancher install (installUUID=%s), skipping",
			installUUID, c.clusterName, obj.Name, owner, ownerInstallUUID)
		return obj, nil
	}

	_, err := c.roleTemplateLister.Get(owner)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Tracef("[cluster-clusterrole-sync] installUUID=%s cluster=%s: roleTemplate %q owning clusterRole %q not found locally, deleting clusterRole",
				installUUID, c.clusterName, owner, obj.Name)
			return obj, c.clusterRoles.Delete(obj.Name, &metav1.DeleteOptions{})
		}
		return obj, err
	}

	return obj, nil
}
