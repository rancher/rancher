package roletemplates

import (
	"fmt"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
				if got, want := len(roles[2].Rules), 1; got != want {
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
			got, err := rth.clusterRolesForRoleTemplate(tt.rt)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			tt.verify(t, got)
		})
	}
}

func Test_extractPromotedRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                 string
		rules                []rbacv1.PolicyRule
		wantPromotedRules    []rbacv1.PolicyRule
		wantNonPromotedRules []rbacv1.PolicyRule
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
			wantNonPromotedRules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get"},
					Resources: []string{"pods"},
					APIGroups: []string{""},
				},
			},
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
			wantNonPromotedRules: []rbacv1.PolicyRule{},
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
			wantNonPromotedRules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get"},
					Resources: []string{"nodes"},
					APIGroups: []string{"cattle.io"},
				},
			},
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
			wantNonPromotedRules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get"},
					Resources: []string{"nodes"},
					APIGroups: []string{"*"},
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
			wantNonPromotedRules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get"},
					Resources: []string{"*"},
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
			wantNonPromotedRules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get"},
					Resources: []string{"pods"},
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
			wantNonPromotedRules: []rbacv1.PolicyRule{},
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
			wantNonPromotedRules: []rbacv1.PolicyRule{},
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
			wantNonPromotedRules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get"},
					Resources: []string{"*"},
					APIGroups: []string{"*"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			promoted, nonPromoted := extractPromotedRules(tt.rules)
			assert.ElementsMatch(t, promoted, tt.wantPromotedRules)
			assert.ElementsMatch(t, nonPromoted, tt.wantNonPromotedRules)
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
				aggregationLabel: "test-rt",
			},
		},
	}
	clusterRoleWithNoAggregationLabel = rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{},
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
						Labels: map[string]string{aggregationLabel: "wrong-rt"},
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
						aggregationLabel: "test-rt",
						"other-label":    "value",
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

func Test_removeLabelFromExternalRole(t *testing.T) {
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
			name: "external role has no label",
			rt:   externalRoleTemplate.DeepCopy(),
			getFunc: func() (*rbacv1.ClusterRole, error) {
				return clusterRoleWithNoAggregationLabel.DeepCopy(), nil
			},
		},
		{
			name: "external role has nil label map",
			rt:   externalRoleTemplate.DeepCopy(),
			getFunc: func() (*rbacv1.ClusterRole, error) {
				return &rbacv1.ClusterRole{}, nil
			},
		},
		{
			name: "external role has label removed",
			rt:   externalRoleTemplate.DeepCopy(),
			getFunc: func() (*rbacv1.ClusterRole, error) {
				return clusterRoleWithAggregationLabel.DeepCopy(), nil
			},
			updateFunc: func() (*rbacv1.ClusterRole, error) { return nil, nil },
			updatedCR:  clusterRoleWithNoAggregationLabel.DeepCopy(),
		},
		{
			name: "external role has label removed but keeps other labels",
			rt:   externalRoleTemplate.DeepCopy(),
			getFunc: func() (*rbacv1.ClusterRole, error) {
				return &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							aggregationLabel: "test-rt",
							"other-label":    "value",
						},
					},
				}, nil
			},
			updateFunc: func() (*rbacv1.ClusterRole, error) { return nil, nil },
			updatedCR: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"other-label": "value"},
				},
			},
		},
		{
			name: "error updating cluster role",
			rt:   externalRoleTemplate.DeepCopy(),
			getFunc: func() (*rbacv1.ClusterRole, error) {
				return clusterRoleWithAggregationLabel.DeepCopy(), nil
			},
			updateFunc: func() (*rbacv1.ClusterRole, error) { return nil, fmt.Errorf("error") },
			updatedCR:  clusterRoleWithNoAggregationLabel.DeepCopy(),
			wantErr:    true,
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
			if err := rth.removeLabelFromExternalRole(tt.rt); (err != nil) != tt.wantErr {
				t.Errorf("roleTemplateHandler.removeLabelFromExternalRole() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
