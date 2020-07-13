package rbac

import (
	"github.com/rancher/rancher/pkg/rbac"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	rbacv1 "github.com/rancher/rancher/pkg/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/types/config"
	v12 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

const (
	grbByUserAndRoleIndex = "authz.cluster.cattle.io/grb-by-user-and-role"
)

func newGlobalRoleBindingHandler(workload *config.UserContext) v3.GlobalRoleBindingHandlerFunc {
	informer := workload.Management.Management.GlobalRoleBindings("").Controller().Informer()
	indexers := map[string]cache.IndexFunc{
		grbByUserAndRoleIndex: grbByUserAndRole,
	}
	informer.AddIndexers(indexers)

	h := &grbHandler{
		grbIndexer:          informer.GetIndexer(),
		clusterRoleBindings: workload.RBAC.ClusterRoleBindings(""),
		crbLister:           workload.RBAC.ClusterRoleBindings("").Controller().Lister(),
	}
	return h.sync
}

// grbHandler ensures the global admins have full access to every cluster. If a globalRoleBinding is created that uses
// the admin role, then the user in that binding gets a clusterRoleBinding in every user cluster to the cluster-admin role
type grbHandler struct {
	clusterRoleBindings rbacv1.ClusterRoleBindingInterface
	crbLister           rbacv1.ClusterRoleBindingLister
	grbIndexer          cache.Indexer
}

func (c *grbHandler) sync(key string, obj *v3.GlobalRoleBinding) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil || obj.GlobalRoleName != "admin" {
		return obj, nil
	}

	bindingName := rbac.GrbCRBName(obj)
	b, err := c.crbLister.Get("", bindingName)
	if err != nil && !apierrors.IsNotFound(err) {
		return obj, err
	}

	if b != nil {
		// binding exists, nothing to do
		return obj, nil
	}

	_, err = c.clusterRoleBindings.Create(&v12.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: bindingName,
		},
		Subjects: []v12.Subject{rbac.GetGRBSubject(obj)},
		RoleRef: v12.RoleRef{
			Name: "cluster-admin",
			Kind: "ClusterRole",
		},
	})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return obj, err
		}
	}
	return obj, nil
}

func grbByUserAndRole(obj interface{}) ([]string, error) {
	grb, ok := obj.(*v3.GlobalRoleBinding)
	if !ok {
		return []string{}, nil
	}

	return []string{rbac.GetGRBTargetKey(grb) + "-" + grb.GlobalRoleName}, nil
}
