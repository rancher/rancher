package rbac

import (
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	wranglerName "github.com/rancher/wrangler/v3/pkg/name"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InheritedRoleName returns the name of the Role created in a downstream namespace for a
// GlobalRole's InheritedNamespacedRules. It hashes the raw GlobalRole name joined with the
// namespace, so it must not be given a name that was already passed through SafeConcatName:
// for names long enough to be truncated that produces a different hash.
func InheritedRoleName(globalRoleName, namespace string) string {
	return wranglerName.SafeConcatName(globalRoleName, namespace)
}

// BuildInheritedRole returns the Role a GlobalRole's InheritedNamespacedRules defines for a
// namespace of a downstream cluster. The GlobalRole controller on the leader replica and the
// namespace handler on the replica owning the cluster both create this object, so its shape
// has to come from one place for the two writers to converge.
func BuildInheritedRole(gr *v3.GlobalRole, namespace string) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      InheritedRoleName(gr.Name, namespace),
			Namespace: namespace,
			Labels: map[string]string{
				GrOwnerLabel: wranglerName.SafeConcatName(gr.Name),
			},
		},
		Rules: gr.InheritedNamespacedRules[namespace],
	}
}

// BuildInheritedRoleBinding returns the RoleBinding a GlobalRoleBinding defines for a namespace
// listed in its GlobalRole's InheritedNamespacedRules. Shared by the leader-side
// GlobalRoleBinding controller and the owner-side namespace handler, like BuildInheritedRole.
func BuildInheritedRoleBinding(grb *v3.GlobalRoleBinding, namespace string) *rbacv1.RoleBinding {
	grbName := wranglerName.SafeConcatName(grb.Name)
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      wranglerName.SafeConcatName(grbName, namespace),
			Namespace: namespace,
			Labels: map[string]string{
				GrbOwnerLabel: grbName,
			},
		},
		Subjects: []rbacv1.Subject{GetGRBSubject(grb)},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     InheritedRoleName(grb.GlobalRoleName, namespace),
		},
	}
}

// AreRolesSame compares the Rules and Labels of two Roles.
func AreRolesSame(currentRole, wantedRole *rbacv1.Role) bool {
	return equality.Semantic.DeepEqual(wantedRole.Rules, currentRole.Rules) &&
		equality.Semantic.DeepEqual(wantedRole.Labels, currentRole.Labels)
}
