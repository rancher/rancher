package helm

import (
	typescorev1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	typesrbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/rancher/types/config"
)

type serviceAccountInjector struct {
	crLister      typesrbacv1.ClusterRoleLister
	crbLister     typesrbacv1.ClusterRoleBindingLister
	rbLister      typesrbacv1.RoleBindingLister
	nsLister      typescorev1.NamespaceLister
	clusterLister v3.ClusterLister
	projectLister v3.ProjectLister
	workload      *config.UserContext
}
