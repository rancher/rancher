package dashboard

import (
	"github.com/rancher/wrangler/v2/pkg/apply"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func addUnauthenticatedRoles(apply apply.Apply) error {
	return apply.
		WithDynamicLookup().
		WithSetID("cattle-unauthenticated").
		ApplyObjects(
			&rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cattle-unauthenticated",
				},
				Rules: []rbacv1.PolicyRule{
					{
						Verbs:         []string{"get"},
						APIGroups:     []string{"management.cattle.io"},
						Resources:     []string{"settings"},
						ResourceNames: []string{"first-login", "ui-pl", "ui-banners", "ui-brand", "ui-favicon", "ui-login-background-light", "ui-login-background-dark"},
					},
				},
			},
			&rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cattle-unauthenticated",
				},
				Subjects: []rbacv1.Subject{{
					Kind:     "Group",
					APIGroup: rbacv1.GroupName,
					Name:     "system:unauthenticated",
				}},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.GroupName,
					Kind:     "ClusterRole",
					Name:     "cattle-unauthenticated",
				},
			},
		)
}
