package roletemplates

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_clusterRolesForRoleTemplate(t *testing.T) {
	sampleRule := rbacv1.PolicyRule{
		Verbs:     []string{"get", "list"},
		Resources: []string{"secrets"},
		APIGroups: []string{""},
	}
	sampleExternalRule := rbacv1.PolicyRule{
		Verbs:     []string{rbacv1.VerbAll},
		Resources: []string{"configmaps"},
		APIGroups: []string{""},
	}
	samplePromotedRule := rbacv1.PolicyRule{
		Verbs:     []string{"get", "list"},
		Resources: []string{"persistentvolumes"},
		APIGroups: []string{""},
	}
	tests := []struct {
		name   string
		rt     *v3.RoleTemplate
		verify func(*testing.T, []*rbacv1.ClusterRole)
	}{
		{
			name: "roletemplates with cluster context creates rules and aggregating clusterroles",
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "myroletemplate",
				},
				Context:       "cluster",
				Rules:         []rbacv1.PolicyRule{sampleRule},
				ExternalRules: []rbacv1.PolicyRule{sampleExternalRule},
			},
			verify: func(t *testing.T, roles []*rbacv1.ClusterRole) {
				if got, want := len(roles), 2; got != want {
					t.Errorf("expected %d roles but got %d", want, got)
				}
				for i, want := range []string{"myroletemplate", "myroletemplate-aggregator"} {
					if got := roles[i].Name; got != want {
						t.Errorf("role[%d] have incorrect name, got %q, want %q", i, got, want)
					}
				}
				if got, want := len(roles[0].Rules), 2; got != want {
					t.Errorf("expected role to have 2 rules but got %d", len(roles))
				}
				if got := roles[1].AggregationRule; got == nil || len(got.ClusterRoleSelectors) == 0 {
					t.Errorf("expected aggregation rule not to be empty")
				}
			},
		},
		{
			name: "roletemplates with project context creates rules and aggregating clusterroles",
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "myroletemplate",
				},
				Context: "project",
				Rules:   []rbacv1.PolicyRule{sampleRule},
			},
			verify: func(t *testing.T, roles []*rbacv1.ClusterRole) {
				if got, want := len(roles), 2; got != want {
					t.Errorf("expected %d roles but got %d", want, got)
				}
				for i, want := range []string{"myroletemplate", "myroletemplate-aggregator"} {
					if got := roles[i].Name; got != want {
						t.Errorf("role[%d] have incorrect name, got %q, want %q", i, got, want)
					}
				}
				if got, want := len(roles[0].Rules), 1; got != want {
					t.Errorf("expected role to have %d rules but got %d", want, got)
				}
				if got := roles[1].AggregationRule; got == nil || len(got.ClusterRoleSelectors) == 0 {
					t.Errorf("expected aggregation rule not to be empty")
				}
			},
		},
		{
			name: "roletemplates with project context and promoted rules create extra clusterroles for global resources",
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "myroletemplate",
				},
				Context: "project",
				Rules:   []rbacv1.PolicyRule{samplePromotedRule},
			},
			verify: func(t *testing.T, roles []*rbacv1.ClusterRole) {
				if got, want := len(roles), 4; got != want {
					t.Errorf("expected %d roles but got %d", want, got)
				}
				for i, want := range []string{"myroletemplate", "myroletemplate-aggregator", "myroletemplate-promoted", "myroletemplate-promoted-aggregator"} {
					if got := roles[i].Name; got != want {
						t.Errorf("role[%d] have incorrect name, got %q, want %q", i, got, want)
					}
				}
				if got, want := len(roles[2].Rules), 1; got != want {
					t.Errorf("expected role to have %d rules but got %d", want, got)
				}
				if got := roles[3].AggregationRule; got == nil || len(got.ClusterRoleSelectors) == 0 {
					t.Errorf("expected aggregation rule not to be empty")
				}
			},
		},
		{
			name: "roletemplates with project context and role template names an extra clusterroles for global resources",
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "myroletemplate",
				},
				Context:           "project",
				RoleTemplateNames: []string{"some-roletemplate"},
			},
			verify: func(t *testing.T, roles []*rbacv1.ClusterRole) {
				if got, want := len(roles), 3; got != want {
					t.Errorf("expected %d roles but got %d", want, got)
				}
				for i, want := range []string{"myroletemplate", "myroletemplate-aggregator", "myroletemplate-promoted-aggregator"} {
					if got := roles[i].Name; got != want {
						t.Errorf("role[%d] have incorrect name, got %q, want %q", i, got, want)
					}
				}
				if got := roles[2].AggregationRule; got == nil || len(got.ClusterRoleSelectors) == 0 {
					t.Errorf("expected aggregation rule not to be empty")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clusterRolesForRoleTemplate(tt.rt)
			tt.verify(t, got)
		})
	}
}
