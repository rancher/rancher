package roletemplates

import (
	"fmt"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
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
)

func Test_clusterRolesForRoleTemplate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                       string
		rt                         *v3.RoleTemplate
		setupClusterRoleController func(*fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList])
		verify                     func(*testing.T, []*rbacv1.ClusterRole)
		wantErr                    bool
	}{
		{
			name: "roletemplates with cluster context creates rules and aggregating clusterroles",
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "myroletemplate",
				},
				Context: "cluster",
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
				Rules:   []rbacv1.PolicyRule{sampleRule, samplePromotedRule},
			},
			verify: func(t *testing.T, roles []*rbacv1.ClusterRole) {
				if got, want := len(roles), 4; got != want {
					t.Errorf("expected %d roles but got %d", want, got)
				}
				for i, want := range []string{"myroletemplate-promoted", "myroletemplate-promoted-aggregator", "myroletemplate", "myroletemplate-aggregator"} {
					if got := roles[i].Name; got != want {
						t.Errorf("role[%d] have incorrect name, got %q, want %q", i, got, want)
					}
				}
				if got, want := len(roles[2].Rules), 2; got != want {
					t.Errorf("expected role to have %d rules but got %d", want, got)
				}
				if got := roles[3].AggregationRule; got == nil || len(got.ClusterRoleSelectors) == 0 {
					t.Errorf("expected aggregation rule not to be empty")
				}
			},
		},
		{
			name: "roletemplates with project context and an inherited roletemplate with promoted rules creates an extra clusterrole for global resources",
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
				for i, want := range []string{"myroletemplate-promoted-aggregator", "myroletemplate", "myroletemplate-aggregator"} {
					if got := roles[i].Name; got != want {
						t.Errorf("role[%d] have incorrect name, got %q, want %q", i, got, want)
					}
				}
				if got := roles[0].AggregationRule; got == nil || len(got.ClusterRoleSelectors) == 0 {
					t.Errorf("expected promoted aggregation rule not to be empty")
				}
				if got := roles[2].AggregationRule; got == nil || len(got.ClusterRoleSelectors) == 0 {
					t.Errorf("expected aggregation rule not to be empty")
				}
			},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().Get("some-roletemplate-promoted-aggregator", metav1.GetOptions{}).Return(&rbacv1.ClusterRole{}, nil)
			},
		},
		{
			name: "error getting inherited roletemplate clusterroles",
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "myroletemplate",
				},
				Context:           "project",
				RoleTemplateNames: []string{"some-roletemplate"},
			},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().Get("some-roletemplate-promoted-aggregator", metav1.GetOptions{}).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "roletemplates with project context and an inherited roletemplate with no promoted rules",
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "myroletemplate",
				},
				Context:           "project",
				RoleTemplateNames: []string{"some-roletemplate"},
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
				if got := roles[1].AggregationRule; got == nil || len(got.ClusterRoleSelectors) == 0 {
					t.Errorf("expected aggregation rule not to be empty")
				}
			},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().Get("some-roletemplate-promoted-aggregator", metav1.GetOptions{}).Return(nil, errNotFound)
			},
		},
		{
			name: "external roletemplates only create a single aggregated cluster role",
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "myroletemplate",
				},
				Context:  "project",
				External: true,
			},
			verify: func(t *testing.T, roles []*rbacv1.ClusterRole) {
				if got, want := len(roles), 1; got != want {
					t.Errorf("expected %d roles but got %d", want, got)
				}
				for i, want := range []string{"myroletemplate-aggregator"} {
					if got := roles[i].Name; got != want {
						t.Errorf("role[%d] have incorrect name, got %q, want %q", i, got, want)
					}
				}
				if got := roles[0].AggregationRule; got == nil || len(got.ClusterRoleSelectors) == 0 {
					t.Errorf("expected aggregation rule not to be empty")
				}
			},
		},
		{
			name: "external roletemplates with project context and promoted rules",
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "myroletemplate",
				},
				Context:  "project",
				External: true,
				Rules:    []rbacv1.PolicyRule{samplePromotedRule},
			},
			verify: func(t *testing.T, roles []*rbacv1.ClusterRole) {
				if got, want := len(roles), 3; got != want {
					t.Errorf("expected %d roles but got %d", want, got)
				}
				for i, want := range []string{"myroletemplate-promoted", "myroletemplate-promoted-aggregator", "myroletemplate-aggregator"} {
					if got := roles[i].Name; got != want {
						t.Errorf("role[%d] have incorrect name, got %q, want %q", i, got, want)
					}
				}
				if got := roles[1].AggregationRule; got == nil || len(got.ClusterRoleSelectors) == 0 {
					t.Errorf("expected aggregation rule not to be empty")
				}
				if got := roles[2].AggregationRule; got == nil || len(got.ClusterRoleSelectors) == 0 {
					t.Errorf("expected aggregation rule not to be empty")
				}
			},
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			crController := fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList](ctrl)
			if tt.setupClusterRoleController != nil {
				tt.setupClusterRoleController(crController)
			}
			rth := &roleTemplateHandler{
				crController: crController,
			}
			roleTemplateCopy := tt.rt.DeepCopy()
			got, err := rth.clusterRolesForRoleTemplate(tt.rt)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			tt.verify(t, got)

			// Ensure the rules have not been modified within clusterRolesForRoleTemplate even though it extracts promoted rules.
			// Otherwise, when the controller runs for a different downstream cluster, it won't have the promoted rules.
			assert.Equal(t, tt.rt.Rules, roleTemplateCopy.Rules)
		})
	}
}

func Test_ExtractPromotedRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		rules             []rbacv1.PolicyRule
		wantPromotedRules []rbacv1.PolicyRule
	}{
		{
			name: "no promoted rules",
			rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get"},
					Resources: []string{"pods"},
					APIGroups: []string{""},
				},
			},
			wantPromotedRules: []rbacv1.PolicyRule{},
		},
		{
			name: "promoted rules",
			rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get"},
					Resources: []string{"nodes"},
					APIGroups: []string{""},
				},
			},
			wantPromotedRules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get"},
					Resources: []string{"nodes"},
					APIGroups: []string{""},
				},
			},
		},
		{
			name: "same resource name wrong apigroup",
			rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get"},
					Resources: []string{"nodes"},
					APIGroups: []string{"cattle.io"},
				},
			},
			wantPromotedRules: []rbacv1.PolicyRule{},
		},
		{
			name: "wildcard apigroup converted to promoted apigroup",
			rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get"},
					Resources: []string{"nodes"},
					APIGroups: []string{"*"},
				},
			},
			wantPromotedRules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get"},
					Resources: []string{"nodes"},
					APIGroups: []string{""},
				},
			},
		},
		{
			name: "wildcard resource converted to promoted resources",
			rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get"},
					Resources: []string{"*"},
					APIGroups: []string{""},
				},
			},
			wantPromotedRules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get"},
					Resources: []string{"nodes"},
					APIGroups: []string{""},
				},
				{
					Verbs:     []string{"get"},
					Resources: []string{"persistentvolumes"},
					APIGroups: []string{""},
				},
			},
		},
		{
			name: "filter out non promoted rules",
			rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get"},
					Resources: []string{"nodes"},
					APIGroups: []string{""},
				},
				{
					Verbs:     []string{"get"},
					Resources: []string{"pods"},
					APIGroups: []string{""},
				},
			},
			wantPromotedRules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get"},
					Resources: []string{"nodes"},
					APIGroups: []string{""},
				},
			},
		},
		{
			name: "only provide local cluster",
			rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get"},
					Resources: []string{"clusters"},
					APIGroups: []string{"management.cattle.io"},
				},
			},
			wantPromotedRules: []rbacv1.PolicyRule{
				{
					Verbs:         []string{"get"},
					Resources:     []string{"clusters"},
					APIGroups:     []string{"management.cattle.io"},
					ResourceNames: []string{"local"},
				},
			},
		},
		{
			name: "all promoted rules",
			rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get"},
					Resources: []string{"navlinks"},
					APIGroups: []string{"ui.cattle.io"},
				},
				{
					Verbs:     []string{"get"},
					Resources: []string{"nodes"},
					APIGroups: []string{""},
				},
				{
					Verbs:     []string{"get"},
					Resources: []string{"persistentvolumes"},
					APIGroups: []string{""},
				},
				{
					Verbs:     []string{"get"},
					Resources: []string{"storageclasses"},
					APIGroups: []string{"storage.k8s.io"},
				},
				{
					Verbs:     []string{"get"},
					Resources: []string{"apiservices"},
					APIGroups: []string{"apiregistration.k8s.io"},
				},
				{
					Verbs:     []string{"get"},
					Resources: []string{"clusterrepos"},
					APIGroups: []string{"catalog.cattle.io"},
				},
				{
					Verbs:     []string{"get"},
					Resources: []string{"clusters"},
					APIGroups: []string{"management.cattle.io"},
				},
			},
			wantPromotedRules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get"},
					Resources: []string{"navlinks"},
					APIGroups: []string{"ui.cattle.io"},
				},
				{
					Verbs:     []string{"get"},
					Resources: []string{"nodes"},
					APIGroups: []string{""},
				},
				{
					Verbs:     []string{"get"},
					Resources: []string{"persistentvolumes"},
					APIGroups: []string{""},
				},
				{
					Verbs:     []string{"get"},
					Resources: []string{"storageclasses"},
					APIGroups: []string{"storage.k8s.io"},
				},
				{
					Verbs:     []string{"get"},
					Resources: []string{"apiservices"},
					APIGroups: []string{"apiregistration.k8s.io"},
				},
				{
					Verbs:     []string{"get"},
					Resources: []string{"clusterrepos"},
					APIGroups: []string{"catalog.cattle.io"},
				},
				{
					Verbs:         []string{"get"},
					Resources:     []string{"clusters"},
					APIGroups:     []string{"management.cattle.io"},
					ResourceNames: []string{"local"},
				},
			},
		},
		{
			name: "star star gives all promoted rules",
			rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get"},
					Resources: []string{"*"},
					APIGroups: []string{"*"},
				},
			},
			wantPromotedRules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get"},
					Resources: []string{"navlinks"},
					APIGroups: []string{"ui.cattle.io"},
				},
				{
					Verbs:     []string{"get"},
					Resources: []string{"nodes"},
					APIGroups: []string{""},
				},
				{
					Verbs:     []string{"get"},
					Resources: []string{"persistentvolumes"},
					APIGroups: []string{""},
				},
				{
					Verbs:     []string{"get"},
					Resources: []string{"storageclasses"},
					APIGroups: []string{"storage.k8s.io"},
				},
				{
					Verbs:     []string{"get"},
					Resources: []string{"apiservices"},
					APIGroups: []string{"apiregistration.k8s.io"},
				},
				{
					Verbs:     []string{"get"},
					Resources: []string{"clusterrepos"},
					APIGroups: []string{"catalog.cattle.io"},
				},
				{
					Verbs:         []string{"get"},
					Resources:     []string{"clusters"},
					APIGroups:     []string{"management.cattle.io"},
					ResourceNames: []string{"local"},
				},
			},
		},
		{
			name: "promoted rule with resource names preserved",
			rules: []rbacv1.PolicyRule{
				{
					Verbs:         []string{"get"},
					Resources:     []string{"nodes"},
					APIGroups:     []string{""},
					ResourceNames: []string{"node-1", "node-2"},
				},
			},
			wantPromotedRules: []rbacv1.PolicyRule{
				{
					Verbs:         []string{"get"},
					Resources:     []string{"nodes"},
					APIGroups:     []string{""},
					ResourceNames: []string{"node-1", "node-2"},
				},
			},
		},
		{
			name: "wildcard resource with specific promoted apigroup",
			rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"list"},
					Resources: []string{"*"},
					APIGroups: []string{"storage.k8s.io"},
				},
			},
			wantPromotedRules: []rbacv1.PolicyRule{
				{Verbs: []string{"list"}, Resources: []string{"storageclasses"}, APIGroups: []string{"storage.k8s.io"}},
			},
		},
		{
			name: "only local cluster resource is part of the promoted rules",
			rules: []rbacv1.PolicyRule{
				{
					Verbs:         []string{"get"},
					Resources:     []string{"clusters"},
					APIGroups:     []string{"management.cattle.io"},
					ResourceNames: []string{"local", "cluster-b"},
				},
			},
			wantPromotedRules: []rbacv1.PolicyRule{
				{
					Verbs:         []string{"get"},
					Resources:     []string{"clusters"},
					APIGroups:     []string{"management.cattle.io"},
					ResourceNames: []string{"local"},
				},
			},
		},
		{
			name: "clusters resource with ResourceNames containing multiple ResourceNames only keeps 'local'",
			rules: []rbacv1.PolicyRule{
				{
					Verbs:         []string{"get"},
					Resources:     []string{"clusters"},
					APIGroups:     []string{"management.cattle.io"},
					ResourceNames: []string{"other-cluster", "local"},
				},
			},
			wantPromotedRules: []rbacv1.PolicyRule{
				{
					Verbs:         []string{"get"},
					Resources:     []string{"clusters"},
					APIGroups:     []string{"management.cattle.io"},
					ResourceNames: []string{"local"},
				},
			},
		},
		{
			name: "clusters resource without ResourceName of 'local' is ignored",
			rules: []rbacv1.PolicyRule{
				{
					Verbs:         []string{"get"},
					Resources:     []string{"clusters"},
					APIGroups:     []string{"management.cattle.io"},
					ResourceNames: []string{"other-cluster-id"},
				},
			},
			wantPromotedRules: []rbacv1.PolicyRule{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			promoted := ExtractPromotedRules(tt.rules)
			assert.ElementsMatch(t, promoted, tt.wantPromotedRules)
		})
	}
}

var (
	externalRoleTemplate = v3.RoleTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "test-rt"},
		External:   true,
	}
	clusterRoleWithAggregationLabel = rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				rbac.AggregationLabel: "test-rt",
			},
		},
	}
)

func Test_addLabelToExternalRole(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		rt         *v3.RoleTemplate
		getFunc    func() (*rbacv1.ClusterRole, error)
		updateFunc func() (*rbacv1.ClusterRole, error)
		updatedCR  *rbacv1.ClusterRole
		wantErr    bool
	}{
		{
			name: "no op if there is no external role",
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-rt"},
				External:   false,
			},
		},
		{
			name:    "error getting external role",
			rt:      externalRoleTemplate.DeepCopy(),
			getFunc: func() (*rbacv1.ClusterRole, error) { return nil, fmt.Errorf("error") },
			wantErr: true,
		},
		{
			name: "external role already has label",
			rt:   externalRoleTemplate.DeepCopy(),
			getFunc: func() (*rbacv1.ClusterRole, error) {
				return clusterRoleWithAggregationLabel.DeepCopy(), nil
			},
		},
		{
			name: "external role has nil label map",
			rt:   externalRoleTemplate.DeepCopy(),
			getFunc: func() (*rbacv1.ClusterRole, error) {
				return &rbacv1.ClusterRole{}, nil
			},
			updateFunc: func() (*rbacv1.ClusterRole, error) {
				return nil, nil
			},
			updatedCR: clusterRoleWithAggregationLabel.DeepCopy(),
		},
		{
			name: "external role has wrong label",
			rt:   externalRoleTemplate.DeepCopy(),
			getFunc: func() (*rbacv1.ClusterRole, error) {
				return &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{rbac.AggregationLabel: "wrong-rt"},
					},
				}, nil
			},
			updateFunc: func() (*rbacv1.ClusterRole, error) {
				return nil, nil
			},
			updatedCR: clusterRoleWithAggregationLabel.DeepCopy(),
		},
		{
			name: "external role missing label",
			rt:   externalRoleTemplate.DeepCopy(),
			getFunc: func() (*rbacv1.ClusterRole, error) {
				return &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"other-label": "value"},
					},
				}, nil
			},
			updateFunc: func() (*rbacv1.ClusterRole, error) {
				return nil, nil
			},
			updatedCR: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						rbac.AggregationLabel: "test-rt",
						"other-label":         "value",
					},
				},
			},
		},
		{
			name: "error updating cluster role",
			rt:   externalRoleTemplate.DeepCopy(),
			getFunc: func() (*rbacv1.ClusterRole, error) {
				return &rbacv1.ClusterRole{}, nil
			},
			updateFunc: func() (*rbacv1.ClusterRole, error) {
				return nil, fmt.Errorf("error")
			},
			updatedCR: clusterRoleWithAggregationLabel.DeepCopy(),
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			crController := fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList](ctrl)
			if tt.getFunc != nil {
				crController.EXPECT().Get(tt.rt.Name, gomock.Any()).Return(tt.getFunc())
			}
			if tt.updateFunc != nil {
				crController.EXPECT().Update(tt.updatedCR).Return(tt.updateFunc())
			}

			rth := &roleTemplateHandler{
				crController: crController,
			}
			if err := rth.addLabelToExternalRole(tt.rt); (err != nil) != tt.wantErr {
				t.Errorf("roleTemplateHandler.addLabelToExternalRole() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_ensureOnlyDesiredClusterRolesExist(t *testing.T) {
	t.Parallel()

	testRT := v3.RoleTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-rt",
		},
		Context: "cluster",
	}

	desiredCR1 := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-rt",
			Labels: map[string]string{
				rbac.ClusterRoleOwnerLabel: "test-rt",
			},
		},
		Rules: []rbacv1.PolicyRule{sampleRule},
	}

	desiredCR2 := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-rt-aggregator",
			Labels: map[string]string{
				rbac.ClusterRoleOwnerLabel: "test-rt",
			},
		},
		AggregationRule: &rbacv1.AggregationRule{
			ClusterRoleSelectors: []metav1.LabelSelector{
				{
					MatchLabels: map[string]string{
						rbac.AggregationLabel: "test-rt",
					},
				},
			},
		},
	}

	unwantedCR := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "old-cluster-role",
			Labels: map[string]string{
				rbac.ClusterRoleOwnerLabel: "test-rt",
			},
		},
	}

	outdatedCR := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-rt",
			Labels: map[string]string{
				rbac.ClusterRoleOwnerLabel: "test-rt",
				"old-label":                "old-value",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get"},
				Resources: []string{"configmaps"},
				APIGroups: []string{""},
			},
		},
	}

	tests := []struct {
		name                       string
		rt                         *v3.RoleTemplate
		desiredCRs                 []*rbacv1.ClusterRole
		setupClusterRoleController func(*fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList])
		wantErr                    bool
	}{
		{
			name:       "create cluster roles when they don't exist",
			rt:         testRT.DeepCopy(),
			desiredCRs: []*rbacv1.ClusterRole{desiredCR1.DeepCopy(), desiredCR2.DeepCopy()},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				// List returns empty list (no existing cluster roles)
				m.EXPECT().List(metav1.ListOptions{LabelSelector: rbac.GetClusterRoleOwnerLabel("test-rt")}).Return(&rbacv1.ClusterRoleList{}, nil)

				// Expect Get calls for each desired cluster role (they don't exist)
				notFoundErr := apierrors.NewNotFound(schema.GroupResource{Group: "rbac.authorization.k8s.io", Resource: "clusterroles"}, "test-rt")
				m.EXPECT().Get("test-rt", metav1.GetOptions{}).Return(nil, notFoundErr)
				m.EXPECT().Create(gomock.Any()).Return(desiredCR1.DeepCopy(), nil)

				notFoundErr2 := apierrors.NewNotFound(schema.GroupResource{Group: "rbac.authorization.k8s.io", Resource: "clusterroles"}, "test-rt-aggregator")
				m.EXPECT().Get("test-rt-aggregator", metav1.GetOptions{}).Return(nil, notFoundErr2)
				m.EXPECT().Create(gomock.Any()).Return(desiredCR2.DeepCopy(), nil)
			},
		},
		{
			name:       "update cluster roles with wrong rules and labels",
			rt:         testRT.DeepCopy(),
			desiredCRs: []*rbacv1.ClusterRole{desiredCR1.DeepCopy()},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().List(metav1.ListOptions{LabelSelector: rbac.GetClusterRoleOwnerLabel("test-rt")}).Return(&rbacv1.ClusterRoleList{}, nil)

				// Get returns outdated cluster role
				m.EXPECT().Get("test-rt", metav1.GetOptions{}).Return(outdatedCR.DeepCopy(), nil)
				// Expect update to fix the cluster role
				m.EXPECT().Update(gomock.Any()).Return(desiredCR1.DeepCopy(), nil)
			},
		},
		{
			name:       "delete unwanted cluster roles",
			rt:         testRT.DeepCopy(),
			desiredCRs: []*rbacv1.ClusterRole{desiredCR1.DeepCopy()},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				// List returns an unwanted cluster role
				m.EXPECT().List(metav1.ListOptions{LabelSelector: rbac.GetClusterRoleOwnerLabel("test-rt")}).Return(&rbacv1.ClusterRoleList{
					Items: []rbacv1.ClusterRole{*unwantedCR.DeepCopy()},
				}, nil)
				// Expect delete of unwanted cluster role
				m.EXPECT().Delete("old-cluster-role", &metav1.DeleteOptions{}).Return(nil)

				// Get returns the desired cluster role already exists
				m.EXPECT().Get("test-rt", metav1.GetOptions{}).Return(desiredCR1.DeepCopy(), nil)
			},
		},
		{
			name:       "delete unwanted and create desired cluster roles",
			rt:         testRT.DeepCopy(),
			desiredCRs: []*rbacv1.ClusterRole{desiredCR1.DeepCopy(), desiredCR2.DeepCopy()},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				// List returns unwanted cluster role
				m.EXPECT().List(metav1.ListOptions{LabelSelector: rbac.GetClusterRoleOwnerLabel("test-rt")}).Return(&rbacv1.ClusterRoleList{
					Items: []rbacv1.ClusterRole{*unwantedCR.DeepCopy()},
				}, nil)
				// Expect delete
				m.EXPECT().Delete("old-cluster-role", &metav1.DeleteOptions{}).Return(nil)

				// Create new desired cluster roles
				notFoundErr := apierrors.NewNotFound(schema.GroupResource{Group: "rbac.authorization.k8s.io", Resource: "clusterroles"}, "test-rt")
				m.EXPECT().Get("test-rt", metav1.GetOptions{}).Return(nil, notFoundErr)
				m.EXPECT().Create(gomock.Any()).Return(desiredCR1.DeepCopy(), nil)

				notFoundErr2 := apierrors.NewNotFound(schema.GroupResource{Group: "rbac.authorization.k8s.io", Resource: "clusterroles"}, "test-rt-aggregator")
				m.EXPECT().Get("test-rt-aggregator", metav1.GetOptions{}).Return(nil, notFoundErr2)
				m.EXPECT().Create(gomock.Any()).Return(desiredCR2.DeepCopy(), nil)
			},
		},
		{
			name:       "error listing existing cluster roles",
			rt:         testRT.DeepCopy(),
			desiredCRs: []*rbacv1.ClusterRole{desiredCR1.DeepCopy()},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().List(metav1.ListOptions{LabelSelector: rbac.GetClusterRoleOwnerLabel("test-rt")}).Return(nil, fmt.Errorf("list error"))
			},
			wantErr: true,
		},
		{
			name:       "error deleting unwanted cluster role",
			rt:         testRT.DeepCopy(),
			desiredCRs: []*rbacv1.ClusterRole{desiredCR1.DeepCopy()},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				// List returns unwanted cluster role
				m.EXPECT().List(metav1.ListOptions{LabelSelector: rbac.GetClusterRoleOwnerLabel("test-rt")}).Return(&rbacv1.ClusterRoleList{
					Items: []rbacv1.ClusterRole{*unwantedCR.DeepCopy()},
				}, nil)
				// Delete fails
				m.EXPECT().Delete("old-cluster-role", &metav1.DeleteOptions{}).Return(fmt.Errorf("delete error"))
			},
			wantErr: true,
		},
		{
			name:       "error creating cluster role",
			rt:         testRT.DeepCopy(),
			desiredCRs: []*rbacv1.ClusterRole{desiredCR1.DeepCopy()},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().List(metav1.ListOptions{LabelSelector: rbac.GetClusterRoleOwnerLabel("test-rt")}).Return(&rbacv1.ClusterRoleList{}, nil)

				notFoundErr := apierrors.NewNotFound(schema.GroupResource{Group: "rbac.authorization.k8s.io", Resource: "clusterroles"}, "test-rt")
				m.EXPECT().Get("test-rt", metav1.GetOptions{}).Return(nil, notFoundErr)
				m.EXPECT().Create(gomock.Any()).Return(nil, fmt.Errorf("create error"))
			},
			wantErr: true,
		},
		{
			name:       "error updating cluster role",
			rt:         testRT.DeepCopy(),
			desiredCRs: []*rbacv1.ClusterRole{desiredCR1.DeepCopy()},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().List(metav1.ListOptions{LabelSelector: rbac.GetClusterRoleOwnerLabel("test-rt")}).Return(&rbacv1.ClusterRoleList{}, nil)

				// Get returns outdated cluster role, triggering an update
				m.EXPECT().Get("test-rt", metav1.GetOptions{}).Return(outdatedCR.DeepCopy(), nil)
				m.EXPECT().Update(gomock.Any()).Return(nil, fmt.Errorf("update error"))
			},
			wantErr: true,
		},
		{
			name:       "no desired cluster roles - delete all existing",
			rt:         testRT.DeepCopy(),
			desiredCRs: []*rbacv1.ClusterRole{},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				// List returns existing cluster roles
				m.EXPECT().List(metav1.ListOptions{LabelSelector: rbac.GetClusterRoleOwnerLabel("test-rt")}).Return(&rbacv1.ClusterRoleList{
					Items: []rbacv1.ClusterRole{*unwantedCR.DeepCopy(), *desiredCR1.DeepCopy()},
				}, nil)
				// Expect both to be deleted
				m.EXPECT().Delete("old-cluster-role", &metav1.DeleteOptions{}).Return(nil)
				m.EXPECT().Delete("test-rt", &metav1.DeleteOptions{}).Return(nil)
			},
		},
	}

	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			crController := fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList](ctrl)
			if tt.setupClusterRoleController != nil {
				tt.setupClusterRoleController(crController)
			}

			rth := &roleTemplateHandler{
				crController: crController,
			}

			err := rth.ensureOnlyDesiredClusterRolesExist(tt.rt, tt.desiredCRs)
			if (err != nil) != tt.wantErr {
				t.Errorf("roleTemplateHandler.ensureOnlyDesiredClusterRolesExist() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
