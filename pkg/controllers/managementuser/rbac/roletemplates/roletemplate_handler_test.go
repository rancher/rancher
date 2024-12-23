package roletemplates

import (
	"fmt"
	"reflect"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	crbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"go.uber.org/mock/gomock"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	sampleRule = rbacv1.PolicyRule{
		Verbs:     []string{"get", "list"},
		Resources: []string{"secrets"},
		APIGroups: []string{""},
	}
	samplePromotedRule = rbacv1.PolicyRule{
		Verbs:     []string{"get", "list"},
		Resources: []string{"persistentvolumes"},
		APIGroups: []string{""},
	}
	errDefault  = fmt.Errorf("error")
	errNotFound = apierrors.NewNotFound(schema.GroupResource{}, "")
)

func Test_clusterRolesForRoleTemplate(t *testing.T) {

	tests := []struct {
		name   string
		rt     *v3.RoleTemplate
		rules  []rbacv1.PolicyRule
		verify func(*testing.T, []*rbacv1.ClusterRole)
	}{
		{
			name: "roletemplates with cluster context creates rules and aggregating clusterroles",
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "myroletemplate",
				},
				Context: "cluster",
			},
			rules: []rbacv1.PolicyRule{sampleRule},
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
					t.Errorf("expected role to have 1 rules but got %d", len(roles))
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
			},
			rules: []rbacv1.PolicyRule{sampleRule},
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
			},
			rules: []rbacv1.PolicyRule{samplePromotedRule},
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
			got := clusterRolesForRoleTemplate(tt.rt, tt.rules)
			tt.verify(t, got)
		})
	}
}

func TestCollectRules(t *testing.T) {
	tests := []struct {
		name     string
		crClient crbacv1.ClusterRoleController
		getFunc  func() (*rbacv1.ClusterRole, error)
		rt       *v3.RoleTemplate
		want     []rbacv1.PolicyRule
		wantErr  bool
	}{
		{
			name: "no external rules",
			rt: &v3.RoleTemplate{
				External: false,
				Rules:    []rbacv1.PolicyRule{sampleRule},
			},
			want: []rbacv1.PolicyRule{sampleRule},
		},
		{
			name: "external rules",
			rt: &v3.RoleTemplate{
				External:      true,
				ExternalRules: []rbacv1.PolicyRule{sampleRule},
			},
			want: []rbacv1.PolicyRule{sampleRule},
		},
		{
			name: "external roletemplates prioritize external rules",
			rt: &v3.RoleTemplate{
				External:      true,
				ExternalRules: []rbacv1.PolicyRule{sampleRule},
				Rules:         []rbacv1.PolicyRule{samplePromotedRule},
			},
			getFunc: func() (*rbacv1.ClusterRole, error) {
				return &rbacv1.ClusterRole{
					Rules: []rbacv1.PolicyRule{samplePromotedRule},
				}, nil
			},
			want: []rbacv1.PolicyRule{sampleRule},
		},
		{
			name: "fetch external role rules",
			rt: &v3.RoleTemplate{
				External: true,
			},
			getFunc: func() (*rbacv1.ClusterRole, error) {
				return &rbacv1.ClusterRole{
					Rules: []rbacv1.PolicyRule{sampleRule},
				}, nil
			},
			want: []rbacv1.PolicyRule{sampleRule},
		},
		{
			name: "fetched external role rules prioritized",
			rt: &v3.RoleTemplate{
				External: true,
				Rules:    []rbacv1.PolicyRule{samplePromotedRule},
			},
			getFunc: func() (*rbacv1.ClusterRole, error) {
				return &rbacv1.ClusterRole{
					Rules: []rbacv1.PolicyRule{sampleRule},
				}, nil
			},
			want: []rbacv1.PolicyRule{sampleRule},
		},
		{
			name: "error fetching external role",
			rt: &v3.RoleTemplate{
				External: true,
			},
			getFunc: func() (*rbacv1.ClusterRole, error) {
				return nil, errDefault
			},
			wantErr: true,
		},
		{
			name: "not found error fetching external role",
			rt: &v3.RoleTemplate{
				External: true,
			},
			getFunc: func() (*rbacv1.ClusterRole, error) {
				return nil, errNotFound
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			crController := fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList](ctrl)
			if tt.getFunc != nil {
				crController.EXPECT().Get(gomock.Any(), gomock.Any()).Return(tt.getFunc()).AnyTimes()
			}
			rth := &roleTemplateHandler{
				crController: crController,
			}

			got, err := rth.collectRules(tt.rt)

			if (err != nil) != tt.wantErr {
				t.Errorf("roleTemplateHandler.collectRules() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("roleTemplateHandler.collectRules() = %v, want %v", got, tt.want)
			}
		})
	}
}
