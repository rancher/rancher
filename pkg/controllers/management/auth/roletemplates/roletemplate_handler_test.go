package roletemplates

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	getRoleTemplates = rbacv1.PolicyRule{
		APIGroups: []string{"management.cattle.io"},
		Verbs:     []string{"get"},
		Resources: []string{"roletemplates"},
	}
	getPRTBS = rbacv1.PolicyRule{
		APIGroups: []string{"management.cattle.io"},
		Verbs:     []string{"get"},
		Resources: []string{"projectroletemplatebindings"},
	}
	getCRTBs = rbacv1.PolicyRule{
		APIGroups: []string{"management.cattle.io"},
		Verbs:     []string{"get"},
		Resources: []string{"clusterroletemplatebindings"},
	}
	listOptions = metav1.ListOptions{
		LabelSelector: "authz.cluster.cattle.io/clusterrole-owner=test-rt",
	}
	terminatingRoleTemplate = v3.RoleTemplate{
		ObjectMeta: metav1.ObjectMeta{
			DeletionTimestamp: &metav1.Time{Time: time.Now()},
		},
	}
	defaultRoleTemplate = v3.RoleTemplate{
		TypeMeta: metav1.TypeMeta{
			Kind:       "roletemplate",
			APIVersion: "management.cattle.io",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-rt",
			UID:  "UID123",
		},
	}
	defaultOwnerReference = metav1.OwnerReference{
		Name:       "test-rt",
		Kind:       "roletemplate",
		APIVersion: "management.cattle.io",
		UID:        "UID123",
	}
	projectManagementClusterRole = rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-rt-project-mgmt",
			OwnerReferences: []metav1.OwnerReference{defaultOwnerReference},
			Labels: map[string]string{
				"management.cattle.io/aggregates":           "test-rt-project-mgmt",
				"authz.cluster.cattle.io/clusterrole-owner": "test-rt",
			},
		},
		Rules: []rbacv1.PolicyRule{getPRTBS},
	}
	projectManagementAggregatingClusterRole = rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-rt-project-mgmt-aggregator",
			OwnerReferences: []metav1.OwnerReference{defaultOwnerReference},
			Labels: map[string]string{
				"management.cattle.io/aggregates":           "test-rt-project-mgmt-aggregator",
				"authz.cluster.cattle.io/clusterrole-owner": "test-rt",
			},
		},
		AggregationRule: &rbacv1.AggregationRule{
			ClusterRoleSelectors: []metav1.LabelSelector{
				{
					MatchLabels: map[string]string{"management.cattle.io/aggregates": "test-rt-project-mgmt"},
				},
			},
		},
	}
	clusterManagementClusterRole = rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-rt-cluster-mgmt",
			OwnerReferences: []metav1.OwnerReference{defaultOwnerReference},
			Labels: map[string]string{
				"management.cattle.io/aggregates":           "test-rt-cluster-mgmt",
				"authz.cluster.cattle.io/clusterrole-owner": "test-rt",
			},
		},
		Rules: []rbacv1.PolicyRule{getCRTBs},
	}
	clusterManagementAggregatingClusterRole = rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-rt-cluster-mgmt-aggregator",
			OwnerReferences: []metav1.OwnerReference{defaultOwnerReference},
			Labels: map[string]string{
				"management.cattle.io/aggregates":           "test-rt-cluster-mgmt-aggregator",
				"authz.cluster.cattle.io/clusterrole-owner": "test-rt",
			},
		},
		AggregationRule: &rbacv1.AggregationRule{
			ClusterRoleSelectors: []metav1.LabelSelector{
				{
					MatchLabels: map[string]string{"management.cattle.io/aggregates": "test-rt-cluster-mgmt"},
				},
			},
		},
	}
)

func Test_OnChange(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                       string
		setupClusterRoleController func(*fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList])
		rt                         *v3.RoleTemplate
		wantErr                    bool
	}{
		{
			name:    "exit early when roletemplate is nil",
			rt:      nil,
			wantErr: false,
		},
		{
			name:    "exit early when roletemplate is terminating",
			rt:      terminatingRoleTemplate.DeepCopy(),
			wantErr: false,
		},
		{
			name: "project RT with no management plane privileges doesn't create CRs",
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-rt",
				},
				Context: "project",
				Rules:   []rbacv1.PolicyRule{getRoleTemplates},
			},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().List(listOptions).Return(&rbacv1.ClusterRoleList{}, nil)
			},
		},
		{
			name: "project RT with management plane privileges creates CRs",
			rt: &v3.RoleTemplate{
				ObjectMeta: defaultRoleTemplate.ObjectMeta,
				Context:    "project",
				TypeMeta:   defaultRoleTemplate.TypeMeta,
				Rules:      []rbacv1.PolicyRule{getPRTBS},
			},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().List(listOptions).Return(&rbacv1.ClusterRoleList{}, nil)
				m.EXPECT().Get("test-rt-project-mgmt", metav1.GetOptions{}).Return(nil, errNotFound)
				m.EXPECT().Create(projectManagementClusterRole.DeepCopy()).Return(nil, nil)
				m.EXPECT().Get("test-rt-project-mgmt-aggregator", metav1.GetOptions{}).Return(nil, errNotFound)
				m.EXPECT().Create(projectManagementAggregatingClusterRole.DeepCopy()).Return(nil, nil)
			},
		},
		{
			name: "cluster RT with no management plane privileges doesn't create CRs",
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-rt",
				},
				Context: "cluster",
				Rules:   []rbacv1.PolicyRule{getRoleTemplates},
			},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().List(listOptions).Return(&rbacv1.ClusterRoleList{}, nil)
			},
		},
		{
			name: "cluster RT with management plane privileges creates CRs",
			rt: &v3.RoleTemplate{
				ObjectMeta: defaultRoleTemplate.ObjectMeta,
				Context:    "cluster",
				TypeMeta:   defaultRoleTemplate.TypeMeta,
				Rules:      []rbacv1.PolicyRule{getCRTBs, getPRTBS},
			},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().List(listOptions).Return(&rbacv1.ClusterRoleList{}, nil)
				m.EXPECT().Get("test-rt-cluster-mgmt", metav1.GetOptions{}).Return(nil, errNotFound)
				m.EXPECT().Create(clusterManagementClusterRole.DeepCopy()).Return(nil, nil)
				m.EXPECT().Get("test-rt-cluster-mgmt-aggregator", metav1.GetOptions{}).Return(nil, errNotFound)
				m.EXPECT().Create(clusterManagementAggregatingClusterRole.DeepCopy()).Return(nil, nil)
				m.EXPECT().Get("test-rt-project-mgmt", metav1.GetOptions{}).Return(nil, errNotFound).AnyTimes()
				m.EXPECT().Create(projectManagementClusterRole.DeepCopy()).Return(nil, nil)
				m.EXPECT().Get("test-rt-project-mgmt-aggregator", metav1.GetOptions{}).Return(nil, errNotFound)
				m.EXPECT().Create(projectManagementAggregatingClusterRole.DeepCopy()).Return(nil, nil)
			},
		},
		{
			name: "update existing cluster roles",
			rt: &v3.RoleTemplate{
				ObjectMeta: defaultRoleTemplate.ObjectMeta,
				Context:    "project",
				TypeMeta:   defaultRoleTemplate.TypeMeta,
				Rules:      []rbacv1.PolicyRule{getPRTBS},
			},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().List(listOptions).Return(&rbacv1.ClusterRoleList{
					Items: []rbacv1.ClusterRole{},
				}, nil)
				existingCR := projectManagementClusterRole.DeepCopy()
				existingCR.Rules = []rbacv1.PolicyRule{} // Make it different
				m.EXPECT().Get("test-rt-project-mgmt", metav1.GetOptions{}).Return(existingCR, nil).AnyTimes()
				m.EXPECT().Get("test-rt-project-mgmt-aggregator", metav1.GetOptions{}).Return(projectManagementAggregatingClusterRole.DeepCopy(), nil)
				m.EXPECT().Update(projectManagementClusterRole.DeepCopy()).Return(nil, nil).AnyTimes()
			},
		},
		{
			name: "error updating existing cluster roles",
			rt: &v3.RoleTemplate{
				ObjectMeta: defaultRoleTemplate.ObjectMeta,
				Context:    "project",
				TypeMeta:   defaultRoleTemplate.TypeMeta,
				Rules:      []rbacv1.PolicyRule{getPRTBS},
			},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().List(listOptions).Return(&rbacv1.ClusterRoleList{
					Items: []rbacv1.ClusterRole{},
				}, nil)
				existingCR := projectManagementClusterRole.DeepCopy()
				existingCR.Rules = []rbacv1.PolicyRule{} // Make it different
				m.EXPECT().Get("test-rt-project-mgmt", metav1.GetOptions{}).Return(existingCR, nil).AnyTimes()
				m.EXPECT().Get("test-rt-project-mgmt-aggregator", metav1.GetOptions{}).Return(projectManagementAggregatingClusterRole.DeepCopy(), nil)
				m.EXPECT().Update(projectManagementClusterRole.DeepCopy()).Return(nil, errDefault).AnyTimes()
			},
			wantErr: true,
		},
		{
			name: "roletemplate changes from having mgmt rules to not having them",
			rt: &v3.RoleTemplate{
				ObjectMeta: defaultRoleTemplate.ObjectMeta,
				Context:    "project",
				TypeMeta:   defaultRoleTemplate.TypeMeta,
				Rules:      []rbacv1.PolicyRule{getRoleTemplates}, // No mgmt plane rules
			},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().List(listOptions).Return(&rbacv1.ClusterRoleList{
					Items: []rbacv1.ClusterRole{*projectManagementClusterRole.DeepCopy()},
				}, nil)
				m.EXPECT().Delete(projectManagementClusterRole.Name, &metav1.DeleteOptions{}).Return(nil)
			},
		},
		{
			name: "error getting external cluster role rules",
			rt: &v3.RoleTemplate{
				ObjectMeta: defaultRoleTemplate.ObjectMeta,
				External:   true,
			},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().Get("test-rt", metav1.GetOptions{}).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "use external rules over rules",
			rt: &v3.RoleTemplate{
				ObjectMeta:    defaultRoleTemplate.ObjectMeta,
				Context:       "project",
				TypeMeta:      defaultRoleTemplate.TypeMeta,
				External:      true,
				ExternalRules: []rbacv1.PolicyRule{getPRTBS},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{"management.cattle.io"},
						Verbs:     []string{"get"},
						Resources: []string{"roletemplates"},
					},
				},
			},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().List(listOptions).Return(&rbacv1.ClusterRoleList{}, nil)
				m.EXPECT().Get("test-rt-project-mgmt", metav1.GetOptions{}).Return(nil, errNotFound)
				m.EXPECT().Create(projectManagementClusterRole.DeepCopy()).Return(nil, nil)
				m.EXPECT().Get("test-rt-project-mgmt-aggregator", metav1.GetOptions{}).Return(nil, errNotFound)
				m.EXPECT().Create(projectManagementAggregatingClusterRole.DeepCopy()).Return(nil, nil)
			},
		},
		{
			name: "use external cluster role rules over rules",
			rt: &v3.RoleTemplate{
				ObjectMeta: defaultRoleTemplate.ObjectMeta,
				Context:    "project",
				TypeMeta:   defaultRoleTemplate.TypeMeta,
				External:   true,
				Rules:      []rbacv1.PolicyRule{getRoleTemplates},
			},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().List(listOptions).Return(&rbacv1.ClusterRoleList{}, nil)
				m.EXPECT().Get("test-rt", metav1.GetOptions{}).Return(&rbacv1.ClusterRole{
					Rules: []rbacv1.PolicyRule{getPRTBS},
				}, nil)
				m.EXPECT().Get("test-rt-project-mgmt", metav1.GetOptions{}).Return(nil, errNotFound)
				m.EXPECT().Create(projectManagementClusterRole.DeepCopy()).Return(nil, nil)
				m.EXPECT().Get("test-rt-project-mgmt-aggregator", metav1.GetOptions{}).Return(nil, errNotFound)
				m.EXPECT().Create(projectManagementAggregatingClusterRole.DeepCopy()).Return(nil, nil)
			},
		},
		{
			name: "inheriting mgmt plane rules, cluster role",
			rt: &v3.RoleTemplate{
				ObjectMeta:        defaultRoleTemplate.ObjectMeta,
				Context:           "cluster",
				TypeMeta:          defaultRoleTemplate.TypeMeta,
				Rules:             []rbacv1.PolicyRule{getRoleTemplates},
				RoleTemplateNames: []string{"child-rt"},
			},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().List(listOptions).Return(&rbacv1.ClusterRoleList{}, nil)
				m.EXPECT().Get("test-rt-project-mgmt-aggregator", metav1.GetOptions{}).Return(nil, errNotFound)
				m.EXPECT().Get("child-rt-project-mgmt-aggregator", metav1.GetOptions{}).Return(&rbacv1.ClusterRole{}, nil)
				m.EXPECT().Create(&rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-rt-project-mgmt-aggregator",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "test-rt",
								Kind:       "roletemplate",
								APIVersion: "management.cattle.io",
								UID:        "UID123",
							},
						},
						Labels: map[string]string{
							"management.cattle.io/aggregates":           "test-rt-project-mgmt-aggregator",
							"authz.cluster.cattle.io/clusterrole-owner": "test-rt",
						},
					},
					AggregationRule: &rbacv1.AggregationRule{
						ClusterRoleSelectors: []metav1.LabelSelector{
							{
								MatchLabels: map[string]string{"management.cattle.io/aggregates": "test-rt-project-mgmt"},
							},
							{
								MatchLabels: map[string]string{"management.cattle.io/aggregates": "child-rt-project-mgmt-aggregator"},
							},
						},
					},
				}).Return(nil, nil)
				m.EXPECT().Get("test-rt-cluster-mgmt-aggregator", metav1.GetOptions{}).Return(nil, errNotFound)
				m.EXPECT().Get("child-rt-cluster-mgmt-aggregator", metav1.GetOptions{}).Return(&rbacv1.ClusterRole{}, nil)
				m.EXPECT().Create(&rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-rt-cluster-mgmt-aggregator",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "test-rt",
								Kind:       "roletemplate",
								APIVersion: "management.cattle.io",
								UID:        "UID123",
							},
						},
						Labels: map[string]string{
							"management.cattle.io/aggregates":           "test-rt-cluster-mgmt-aggregator",
							"authz.cluster.cattle.io/clusterrole-owner": "test-rt",
						},
					},
					AggregationRule: &rbacv1.AggregationRule{
						ClusterRoleSelectors: []metav1.LabelSelector{
							{
								MatchLabels: map[string]string{"management.cattle.io/aggregates": "test-rt-cluster-mgmt"},
							},
							{
								MatchLabels: map[string]string{"management.cattle.io/aggregates": "child-rt-cluster-mgmt-aggregator"},
							},
						},
					},
				}).Return(nil, nil)
			},
		},
		{
			name: "inheriting mgmt plane rules, project role",
			rt: &v3.RoleTemplate{
				ObjectMeta:        defaultRoleTemplate.ObjectMeta,
				Context:           "project",
				TypeMeta:          defaultRoleTemplate.TypeMeta,
				Rules:             []rbacv1.PolicyRule{getRoleTemplates},
				RoleTemplateNames: []string{"child-rt"},
			},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().List(listOptions).Return(&rbacv1.ClusterRoleList{}, nil)
				m.EXPECT().Get("test-rt-project-mgmt-aggregator", metav1.GetOptions{}).Return(nil, errNotFound)
				m.EXPECT().Get("child-rt-project-mgmt-aggregator", metav1.GetOptions{}).Return(&rbacv1.ClusterRole{}, nil)
				m.EXPECT().Create(&rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-rt-project-mgmt-aggregator",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "test-rt",
								Kind:       "roletemplate",
								APIVersion: "management.cattle.io",
								UID:        "UID123",
							},
						},
						Labels: map[string]string{
							"management.cattle.io/aggregates":           "test-rt-project-mgmt-aggregator",
							"authz.cluster.cattle.io/clusterrole-owner": "test-rt",
						},
					},
					AggregationRule: &rbacv1.AggregationRule{
						ClusterRoleSelectors: []metav1.LabelSelector{
							{
								MatchLabels: map[string]string{"management.cattle.io/aggregates": "test-rt-project-mgmt"},
							},
							{
								MatchLabels: map[string]string{"management.cattle.io/aggregates": "child-rt-project-mgmt-aggregator"},
							},
						},
					},
				}).Return(nil, nil)
			},
		},
		{
			name: "inherited roles have no mgmt plane rules",
			rt: &v3.RoleTemplate{
				ObjectMeta:        defaultRoleTemplate.ObjectMeta,
				Context:           "cluster",
				TypeMeta:          defaultRoleTemplate.TypeMeta,
				Rules:             []rbacv1.PolicyRule{getRoleTemplates},
				RoleTemplateNames: []string{"child-rt"},
			},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().List(listOptions).Return(&rbacv1.ClusterRoleList{}, nil)
				m.EXPECT().Get("child-rt-project-mgmt-aggregator", metav1.GetOptions{}).Return(nil, errNotFound)
				m.EXPECT().Get("child-rt-cluster-mgmt-aggregator", metav1.GetOptions{}).Return(nil, errNotFound)
			},
		},
		{
			name: "error getting inherited project mgmt role",
			rt: &v3.RoleTemplate{
				ObjectMeta:        defaultRoleTemplate.ObjectMeta,
				Context:           "cluster",
				TypeMeta:          defaultRoleTemplate.TypeMeta,
				Rules:             []rbacv1.PolicyRule{getRoleTemplates},
				RoleTemplateNames: []string{"child-rt"},
			},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().Get("child-rt-project-mgmt-aggregator", metav1.GetOptions{}).Return(nil, errDefault)
				m.EXPECT().Get("child-rt-cluster-mgmt-aggregator", metav1.GetOptions{}).Return(nil, errNotFound)
			},
			wantErr: true,
		},
		{
			name: "error getting inherited cluster mgmt role",
			rt: &v3.RoleTemplate{
				ObjectMeta:        defaultRoleTemplate.ObjectMeta,
				Context:           "cluster",
				TypeMeta:          defaultRoleTemplate.TypeMeta,
				Rules:             []rbacv1.PolicyRule{getRoleTemplates},
				RoleTemplateNames: []string{"child-rt"},
			},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().Get("child-rt-cluster-mgmt-aggregator", metav1.GetOptions{}).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "error listing cluster roles",
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-rt",
				},
				Context: "project",
				Rules:   []rbacv1.PolicyRule{getRoleTemplates},
			},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().List(listOptions).Return(&rbacv1.ClusterRoleList{}, errDefault)
			},
			wantErr: true,
		},
		{
			name: "remove cluster roles that should not exist",
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-rt",
				},
				Context: "project",
				Rules:   []rbacv1.PolicyRule{getRoleTemplates},
			},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().List(listOptions).Return(&rbacv1.ClusterRoleList{
					Items: []rbacv1.ClusterRole{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "bad-cr"},
						},
						{
							ObjectMeta: metav1.ObjectMeta{Name: "bad-cr2"},
						},
					},
				}, nil)
				m.EXPECT().Delete("bad-cr", &metav1.DeleteOptions{}).Return(nil)
				m.EXPECT().Delete("bad-cr2", &metav1.DeleteOptions{}).Return(nil)
			},
		},
		{
			name: "dont remove cluster roles from downstream cluster handler",
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-rt",
				},
				Context: "project",
				Rules:   []rbacv1.PolicyRule{getRoleTemplates},
			},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().List(listOptions).Return(&rbacv1.ClusterRoleList{
					Items: []rbacv1.ClusterRole{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "test-rt"},
						},
						{
							ObjectMeta: metav1.ObjectMeta{Name: "test-rt-aggregator"},
						},
						{
							ObjectMeta: metav1.ObjectMeta{Name: "test-rt-promoted"},
						},
						{
							ObjectMeta: metav1.ObjectMeta{Name: "test-rt-promoted-aggregator"},
						},
						{
							ObjectMeta: metav1.ObjectMeta{Name: "bad-cr"},
						},
					},
				}, nil)
				m.EXPECT().Delete("bad-cr", &metav1.DeleteOptions{}).Return(nil)
			},
		},
		{
			name: "error deleting cluster roles, non blocking",
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-rt",
				},
				Context: "project",
				Rules:   []rbacv1.PolicyRule{getRoleTemplates},
			},
			setupClusterRoleController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().List(listOptions).Return(&rbacv1.ClusterRoleList{
					Items: []rbacv1.ClusterRole{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "bad-cr"},
						},
						{
							ObjectMeta: metav1.ObjectMeta{Name: "bad-cr2"},
						},
					},
				}, nil)
				m.EXPECT().Delete("bad-cr", &metav1.DeleteOptions{}).Return(errDefault)
				m.EXPECT().Delete("bad-cr2", &metav1.DeleteOptions{}).Return(errDefault)
			},
			wantErr: true,
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
			r := &roleTemplateHandler{
				crController: crController,
			}

			_, err := r.OnChange("", tt.rt)

			if (err != nil) != tt.wantErr {
				t.Errorf("roleTemplateHandler.WaitForClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func Test_getManagementPlaneRules(t *testing.T) {
	t.Parallel()
	sampleResources := map[string]string{
		"nodes": "management.cattle.io",
	}

	tests := []struct {
		name                string
		rules               []rbacv1.PolicyRule
		managementResources map[string]string
		want                []rbacv1.PolicyRule
	}{
		{
			name: "no management resources returns empty map",
			rules: []rbacv1.PolicyRule{
				{
					Resources: []string{"pods"},
					APIGroups: []string{"management.cattle.io"},
					Verbs:     []string{"get"},
				},
			},
			managementResources: sampleResources,
			want:                []rbacv1.PolicyRule{},
		},
		{
			name: "rules contains management resource",
			rules: []rbacv1.PolicyRule{
				{
					Resources: []string{"nodes"},
					APIGroups: []string{"management.cattle.io"},
					Verbs:     []string{"get"},
				},
			},
			managementResources: sampleResources,
			want: []rbacv1.PolicyRule{
				{
					Resources: []string{"nodes"},
					APIGroups: []string{"management.cattle.io"},
					Verbs:     []string{"get"},
				},
			},
		},
		{
			name: "rule that contains management resource and rule that contains other resource",
			rules: []rbacv1.PolicyRule{
				{
					Resources: []string{"nodes"},
					APIGroups: []string{"management.cattle.io"},
					Verbs:     []string{"get"},
				},
				{
					Resources: []string{"roletemplates"},
					APIGroups: []string{"management.cattle.io"},
					Verbs:     []string{"get"},
				},
			},
			managementResources: sampleResources,
			want: []rbacv1.PolicyRule{
				{
					Resources: []string{"nodes"},
					APIGroups: []string{"management.cattle.io"},
					Verbs:     []string{"get"},
				},
			},
		},
		{
			name: "multiple resources and apigroups in same rule get filtered",
			rules: []rbacv1.PolicyRule{
				{
					Resources: []string{"nodes", "pods"},
					APIGroups: []string{"management.cattle.io", "rbac.cattle.io"},
					Verbs:     []string{"get"},
				},
			},
			managementResources: sampleResources,
			want: []rbacv1.PolicyRule{
				{
					Resources: []string{"nodes"},
					APIGroups: []string{"management.cattle.io"},
					Verbs:     []string{"get"},
				},
			},
		},
		{
			name: "wildcard resources and apigroups get reduced to just management resources",
			rules: []rbacv1.PolicyRule{
				{
					Resources: []string{"*"},
					APIGroups: []string{"*"},
					Verbs:     []string{"get"},
				},
			},
			managementResources: sampleResources,
			want: []rbacv1.PolicyRule{
				{
					Resources: []string{"nodes"},
					APIGroups: []string{"management.cattle.io"},
					Verbs:     []string{"get"},
				},
			},
		},
		{
			name: "if resource names are specified they are not considered management resources",
			rules: []rbacv1.PolicyRule{
				{
					Resources:     []string{"nodes"},
					APIGroups:     []string{"management.cattle.io"},
					Verbs:         []string{"get"},
					ResourceNames: []string{"my-node"},
				},
			},
			managementResources: sampleResources,
			want:                []rbacv1.PolicyRule{},
		},
		{
			name: "get all cluster management plane resources",
			rules: []rbacv1.PolicyRule{
				{
					Resources: []string{"*"},
					APIGroups: []string{"*"},
					Verbs:     []string{"*"},
				},
			},
			managementResources: clusterManagementPlaneResources,
			want: []rbacv1.PolicyRule{
				{
					Resources: []string{"clusterscans"},
					APIGroups: []string{"management.cattle.io"},
					Verbs:     []string{"*"},
				},
				{
					Resources: []string{"clusterregistrationtokens"},
					APIGroups: []string{"management.cattle.io"},
					Verbs:     []string{"*"},
				},
				{
					Resources: []string{"clusterroletemplatebindings"},
					APIGroups: []string{"management.cattle.io"},
					Verbs:     []string{"*"},
				},
				{
					Resources: []string{"etcdbackups"},
					APIGroups: []string{"management.cattle.io"},
					Verbs:     []string{"*"},
				},
				{
					Resources: []string{"nodes"},
					APIGroups: []string{"management.cattle.io"},
					Verbs:     []string{"*"},
				},
				{
					Resources: []string{"nodepools"},
					APIGroups: []string{"management.cattle.io"},
					Verbs:     []string{"*"},
				},
				{
					Resources: []string{"projects"},
					APIGroups: []string{"management.cattle.io"},
					Verbs:     []string{"*"},
				},
				{
					Resources: []string{"etcdsnapshots"},
					APIGroups: []string{"rke.cattle.io"},
					Verbs:     []string{"*"},
				},
			},
		},
		{
			name: "get all project management plane resources",
			rules: []rbacv1.PolicyRule{
				{
					Resources: []string{"*"},
					APIGroups: []string{"*"},
					Verbs:     []string{"*"},
				},
			},
			managementResources: projectManagementPlaneResources,
			want: []rbacv1.PolicyRule{
				{
					Resources: []string{"projectroletemplatebindings"},
					APIGroups: []string{"management.cattle.io"},
					Verbs:     []string{"*"},
				},
				{
					Resources: []string{"secrets"},
					APIGroups: []string{""},
					Verbs:     []string{"*"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.ElementsMatch(t, getManagementPlaneRules(tt.rules, tt.managementResources), tt.want)
		})
	}
}

func Test_gatherRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		rt      *v3.RoleTemplate
		getFunc func() (*rbacv1.ClusterRole, error)
		want    []rbacv1.PolicyRule
		wantErr bool
	}{
		{
			name: "not external role template",
			rt: &v3.RoleTemplate{
				External: false,
				Rules:    []rbacv1.PolicyRule{getRoleTemplates},
			},
			want: []rbacv1.PolicyRule{getRoleTemplates},
		},
		{
			name: "external rules has priority over rules and external cluster role",
			rt: &v3.RoleTemplate{
				External:      true,
				ExternalRules: []rbacv1.PolicyRule{getRoleTemplates},
				Rules:         []rbacv1.PolicyRule{getPRTBS},
			},
			want: []rbacv1.PolicyRule{getRoleTemplates},
		},
		{
			name: "external cluster role has priority over rules",
			rt: &v3.RoleTemplate{
				External: true,
				Rules:    []rbacv1.PolicyRule{getPRTBS},
			},
			getFunc: func() (*rbacv1.ClusterRole, error) {
				return &rbacv1.ClusterRole{
					Rules: []rbacv1.PolicyRule{getRoleTemplates},
				}, nil
			},
			want: []rbacv1.PolicyRule{getRoleTemplates},
		},
		{
			name: "error getting external cluster role",
			rt: &v3.RoleTemplate{
				External: true,
				Rules:    []rbacv1.PolicyRule{getPRTBS},
			},
			getFunc: func() (*rbacv1.ClusterRole, error) {
				return nil, errDefault
			},
			wantErr: true,
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			crController := fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList](ctrl)
			if tt.getFunc != nil {
				crController.EXPECT().Get(tt.rt.Name, metav1.GetOptions{}).Return(tt.getFunc())
			}
			r := &roleTemplateHandler{
				crController: crController,
			}
			got, err := r.gatherRules(tt.rt)
			if (err != nil) != tt.wantErr {
				t.Errorf("roleTemplateHandler.gatherRules() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("roleTemplateHandler.gatherRules() = %v, want %v", got, tt.want)
			}
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
	clusterRoleWithNoAggregationLabel = rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{},
		},
	}
)

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
							rbac.AggregationLabel: "test-rt",
							"other-label":         "value",
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

			if err := removeLabelFromExternalRole(tt.rt, crController); (err != nil) != tt.wantErr {
				t.Errorf("roleTemplateHandler.removeLabelFromExternalRole() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
