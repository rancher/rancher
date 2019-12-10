package agentauth

import (
	"context"

	managementv3 "github.com/rancher/rancher/pkg/wrangler/generated/controllers/management.cattle.io/v3"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	rbac "github.com/rancher/wrangler-api/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/name"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func Register(ctx context.Context, cluster managementv3.ClusterController, rbac rbac.Interface, apply apply.Apply) error {
	h := handler{}
	managementv3.RegisterClusterGeneratingHandler(ctx,
		cluster,
		apply.WithCacheTypes(
			cluster,
			rbac.ClusterRole(),
			rbac.ClusterRoleBinding(),
			rbac.Role(),
			rbac.RoleBinding()),
		"",
		"agent-auth",
		h.generate,
		&generic.GeneratingHandlerOptions{
			AllowClusterScoped: true,
		})
	return nil
}

type handler struct{}

func (h *handler) generate(cluster *v3.Cluster, status v3.ClusterStatus) ([]runtime.Object, v3.ClusterStatus, error) {
	return []runtime.Object{
		// ACCESS TO ALL
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: name.SafeConcatName("agent-cluster-admin", cluster.Name),
			},
			Subjects: []rbacv1.Subject{
				{
					Kind: "User",
					Name: "system:cluster:" + cluster.Name,
				},
			},
			RoleRef: rbacv1.RoleRef{
				Kind: "ClusterRole",
				Name: "cluster-admin",
			},
		},

		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: name.SafeConcatName("agent-view", cluster.Name),
			},
			Rules: []rbacv1.PolicyRule{
				{
					Verbs:         []string{"get"},
					APIGroups:     []string{"management.cattle.io"},
					Resources:     []string{"clusters"},
					ResourceNames: []string{cluster.Name},
				},
			},
		},
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: name.SafeConcatName("agent-view-binding", cluster.Name),
			},
			Subjects: []rbacv1.Subject{
				{
					Kind: "User",
					Name: "system:cluster:" + cluster.Name,
				},
			},
			RoleRef: rbacv1.RoleRef{
				Kind: "ClusterRole",
				Name: name.SafeConcatName("agent-view", cluster.Name),
			},
		},
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "agent-view",
				Namespace: cluster.Name,
			},
			Rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"update", "get", "list", "watch"},
					APIGroups: []string{"management.cattle.io"},
					Resources: []string{rbacv1.ResourceAll},
				},
			},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "agent-view-binding",
				Namespace: cluster.Name,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind: "User",
					Name: "system:cluster:" + cluster.Name,
				},
			},
			RoleRef: rbacv1.RoleRef{
				Kind: "Role",
				Name: "agent-view",
			},
		},
	}, status, nil
}
