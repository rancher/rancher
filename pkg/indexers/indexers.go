package indexers

import (
	"github.com/rancher/rancher/pkg/wrangler"
	v1 "k8s.io/api/rbac/v1"
)

func RegisterIndexers(config *wrangler.Context) {
	config.Mgmt.Cluster().Cache().AddIndexer(ClusterByPSPTKey, clusterByPSPT)
	config.Mgmt.Cluster().Cache().AddIndexer(ClusterByGenericEngineConfigKey, clusterByKontainerDriver)

	config.RBAC.ClusterRoleBinding().Cache().AddIndexer(RBByRoleAndSubjectIndex, rbByClusterRoleAndSubject)
	config.RBAC.ClusterRoleBinding().Cache().AddIndexer(MembershipBindingOwnerIndex, func(obj *v1.ClusterRoleBinding) ([]string, error) {
		return indexByMembershipBindingOwner(obj)
	})

	config.RBAC.RoleBinding().Cache().AddIndexer(RBByOwnerIndex, rbByOwner)
	config.RBAC.RoleBinding().Cache().AddIndexer(RBByRoleAndSubjectIndex, rbByRoleAndSubject)
	config.RBAC.RoleBinding().Cache().AddIndexer(MembershipBindingOwnerIndex, func(obj *v1.RoleBinding) ([]string, error) {
		return indexByMembershipBindingOwner(obj)
	})
}
