package rbac

import (
	"fmt"
	"strings"
	"testing"

	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rbacfakes "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1/fakes"
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Test_getProjectResourcesRules(t *testing.T) {
	tests := []struct {
		name      string
		rules     []rbacv1.PolicyRule
		apis      map[string]metav1.APIResource
		want      []rbacv1.PolicyRule
		wantError bool
	}{
		{
			name:  "empty rules",
			rules: []rbacv1.PolicyRule{},
			apis: map[string]metav1.APIResource{
				"pods": metav1.APIResource{},
			},
			want: []rbacv1.PolicyRule{},
		},
		{
			name: "non-listable rules",
			rules: []rbacv1.PolicyRule{
				{
					Verbs: []string{"create", "update", "delete", "get", "watch"},
				},
			},
			apis: map[string]metav1.APIResource{
				"pods": metav1.APIResource{},
			},
			want: []rbacv1.PolicyRule{},
		},
		{
			name: "resources are not in project resources API", // non-namespaced resources don't need these rules
			rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get", "list", "watch"},
					Resources: []string{"clusters"},
					APIGroups: []string{"management.cattle.io"},
				},
			},
			apis: map[string]metav1.APIResource{
				"pods": metav1.APIResource{},
			},
			want: []rbacv1.PolicyRule{},
		},
		{
			name: "resources are in project resources API",
			rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get", "list", "watch"},
					Resources: []string{"pods"},
					APIGroups: []string{""},
				},
				{
					Verbs:     []string{"get", "list", "watch"},
					Resources: []string{"deployments", "daemonsets"},
					APIGroups: []string{"apps"},
				},
			},
			apis: map[string]metav1.APIResource{
				"pods":             metav1.APIResource{},
				"apps.deployments": metav1.APIResource{},
				"apps.daemonsets":  metav1.APIResource{},
			},
			want: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"list"},
					Resources: []string{"pods", "apps.deployments", "apps.daemonsets"},
					APIGroups: []string{"resources.project.cattle.io"},
				},
			},
		},
		{
			name: "wildcard verb",
			rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"*"},
					Resources: []string{"pods"},
					APIGroups: []string{""},
				},
			},
			apis: map[string]metav1.APIResource{
				"pods": metav1.APIResource{},
			},
			want: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"list"},
					Resources: []string{"pods"},
					APIGroups: []string{"resources.project.cattle.io"},
				},
			},
		},
		{
			name: "wildcard resource",
			rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get", "list", "watch"},
					Resources: []string{"*"},
					APIGroups: []string{"apps"},
				},
			},
			apis: map[string]metav1.APIResource{
				"pods": metav1.APIResource{
					Name:  "pods",
					Group: "",
				},
				"apps.deployments": metav1.APIResource{
					Name:  "apps.deployments",
					Group: "apps",
				},
				"apps.daemonsets": metav1.APIResource{
					Name:  "apps.daemonsets",
					Group: "apps",
				},
			},
			want: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"list"},
					Resources: []string{"apps.daemonsets", "apps.deployments"},
					APIGroups: []string{"resources.project.cattle.io"},
				},
			},
		},
		{
			name: "wildcard resource for multiple apigroups",
			rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get", "list", "watch"},
					Resources: []string{"*"},
					APIGroups: []string{"apps", ""},
				},
			},
			apis: map[string]metav1.APIResource{
				"pods": metav1.APIResource{
					Name:  "pods",
					Group: "",
				},
				"apps.deployments": metav1.APIResource{
					Name:  "apps.deployments",
					Group: "apps",
				},
				"apps.daemonsets": metav1.APIResource{
					Name:  "apps.daemonsets",
					Group: "apps",
				},
			},
			want: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"list"},
					Resources: []string{"apps.daemonsets", "apps.deployments", "pods"},
					APIGroups: []string{"resources.project.cattle.io"},
				},
			},
		},
		{
			name: "wildcard resource and apigroup",
			rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get", "list", "watch"},
					Resources: []string{"*"},
					APIGroups: []string{"*"},
				},
			},
			apis: map[string]metav1.APIResource{
				"pods": metav1.APIResource{},
			},
			want: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"list"},
					Resources: []string{"*"},
					APIGroups: []string{"resources.project.cattle.io"},
				},
			},
		},
		{
			name: "wildcard apigroup",
			rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get", "list", "watch"},
					Resources: []string{"apps"},
					APIGroups: []string{"*"},
				},
			},
			apis: map[string]metav1.APIResource{
				"catalog.cattle.io.apps": metav1.APIResource{
					Name:  "catalog.cattle.io.apps",
					Group: "catalog.cattle.io",
				},
				"project.cattle.io.apps": metav1.APIResource{
					Name:  "project.cattle.io.apps",
					Group: "project.cattle.io",
				},
			},
			want: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"list"},
					Resources: []string{"catalog.cattle.io.apps", "project.cattle.io.apps"},
					APIGroups: []string{"resources.project.cattle.io"},
				},
			},
		},
		{
			name: "wildcard apigroup for multiple resources",
			rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get", "list", "watch"},
					Resources: []string{"apps", "deployments"},
					APIGroups: []string{"*"},
				},
			},
			apis: map[string]metav1.APIResource{
				"catalog.cattle.io.apps": metav1.APIResource{
					Name:  "catalog.cattle.io.apps",
					Group: "catalog.cattle.io",
				},
				"project.cattle.io.apps": metav1.APIResource{
					Name:  "project.cattle.io.apps",
					Group: "project.cattle.io",
				},
				"apps.deployments": metav1.APIResource{
					Name:  "apps.deployments",
					Group: "apps",
				},
			},
			want: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"list"},
					Resources: []string{"apps.deployments", "catalog.cattle.io.apps", "project.cattle.io.apps"},
					APIGroups: []string{"resources.project.cattle.io"},
				},
			},
		},
		{
			name: "named resources",
			rules: []rbacv1.PolicyRule{
				{
					Verbs:         []string{"get", "list", "watch"},
					Resources:     []string{"pods"},
					APIGroups:     []string{""},
					ResourceNames: []string{"nginx", "mysql"},
				},
			},
			apis: map[string]metav1.APIResource{
				"pods": metav1.APIResource{},
			},
			want: []rbacv1.PolicyRule{},
		},
		{
			name: "non resource URLs",
			rules: []rbacv1.PolicyRule{
				{
					Verbs:           []string{"get", "list", "watch"},
					NonResourceURLs: []string{"/service"},
				},
			},
			apis: map[string]metav1.APIResource{
				"pods": metav1.APIResource{},
			},
			want: []rbacv1.PolicyRule{},
		},
		{
			name: "informer not ready",
			rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get", "list", "watch"},
					Resources: []string{"pods"},
					APIGroups: []string{""},
				},
			},
			apis:      nil,
			wantError: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			apis := &fakeAPIs{
				resourceMap: test.apis,
			}
			m := &manager{apis: apis}
			got, err := m.getProjectResourcesRules(test.rules)
			if test.wantError {
				assert.Error(t, err)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, test.want, got)
			}
		})
	}
}

func Test_ensureProjectResourcesRoles(t *testing.T) {
	var mockClusterRoles map[string]*rbacv1.ClusterRole
	tests := []struct {
		name            string
		rt              *v3.RoleTemplate
		getFunc         func(ns, name string) (*rbacv1.ClusterRole, error)
		setup           func()
		wantCreateCalls int
		wantUpdateCalls int
		wantDeleteCalls int
		wantClusterRole *rbacv1.ClusterRole
		wantError       bool
	}{
		{
			name: "create new",
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			},
			getFunc: func(ns, name string) (*rbacv1.ClusterRole, error) {
				if !strings.HasSuffix(name, "-projectresources") {
					return &rbacv1.ClusterRole{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
						Rules: []rbacv1.PolicyRule{
							{
								Verbs:     []string{"list"},
								Resources: []string{"pods"},
								APIGroups: []string{""},
							},
						},
					}, nil
				}
				return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
			},
			wantCreateCalls: 1,
			wantUpdateCalls: 0,
			wantDeleteCalls: 0,
			wantClusterRole: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-projectresources",
					Annotations: map[string]string{
						"authz.cluster.cattle.io/clusterrole-owner": "test",
						"resources.project.cattle.io/authz":         "test",
					},
				},
				Rules: []rbacv1.PolicyRule{
					{
						Verbs:     []string{"list"},
						Resources: []string{"pods"},
						APIGroups: []string{"resources.project.cattle.io"},
					},
				},
			},
		},
		{
			name: "update existing",
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			},
			getFunc: func(ns, name string) (*rbacv1.ClusterRole, error) {
				if !strings.HasSuffix(name, "-projectresources") {
					return &rbacv1.ClusterRole{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
						Rules: []rbacv1.PolicyRule{
							{
								Verbs:     []string{"list"},
								Resources: []string{"pods"},
								APIGroups: []string{""},
							},
						},
					}, nil
				}
				return &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-projectresources",
						Annotations: map[string]string{
							"authz.cluster.cattle.io/clusterrole-owner": "test",
							"resources.project.cattle.io/authz":         "test",
						},
					},
				}, nil
			},
			wantCreateCalls: 0,
			wantUpdateCalls: 1,
			wantDeleteCalls: 0,
			wantClusterRole: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-projectresources",
					Annotations: map[string]string{
						"authz.cluster.cattle.io/clusterrole-owner": "test",
						"resources.project.cattle.io/authz":         "test",
					},
				},
				Rules: []rbacv1.PolicyRule{
					{
						Verbs:     []string{"list"},
						Resources: []string{"pods"},
						APIGroups: []string{"resources.project.cattle.io"},
					},
				},
			},
		},
		{
			name: "no namespaced rules",
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			},
			getFunc: func(ns, name string) (*rbacv1.ClusterRole, error) {
				if !strings.HasSuffix(name, "-projectresources") {
					return &rbacv1.ClusterRole{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
						Rules: []rbacv1.PolicyRule{
							{
								Verbs:     []string{"list"},
								Resources: []string{"namespaces"},
								APIGroups: []string{""},
							},
						},
					}, nil
				}
				return nil, nil
			},
			wantCreateCalls: 0,
			wantUpdateCalls: 0,
			wantDeleteCalls: 0,
			wantClusterRole: nil,
		},
		{
			name: "no namespaced rules update",
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			},
			getFunc: func(ns, name string) (*rbacv1.ClusterRole, error) {
				if !strings.HasSuffix(name, "-projectresources") {
					return &rbacv1.ClusterRole{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
						Rules: []rbacv1.PolicyRule{
							{
								Verbs:     []string{"list"},
								Resources: []string{"namespaces"},
								APIGroups: []string{""},
							},
						},
					}, nil
				}
				return &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-projectresources",
						Annotations: map[string]string{
							"authz.cluster.cattle.io/clusterrole-owner": "test",
							"resources.project.cattle.io/authz":         "test",
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							Verbs:     []string{"list"},
							Resources: []string{"pods"},
							APIGroups: []string{"resources.project.cattle.io"},
						},
					},
				}, nil
			},
			wantCreateCalls: 0,
			wantUpdateCalls: 0,
			wantDeleteCalls: 1,
		},
		{
			name: "equal rules",
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			},
			getFunc: func(ns, name string) (*rbacv1.ClusterRole, error) {
				if !strings.HasSuffix(name, "-projectresources") {
					return &rbacv1.ClusterRole{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
							Annotations: map[string]string{
								"authz.cluster.cattle.io/clusterrole-owner": "test",
								"resources.project.cattle.io/authz":         "test",
							},
						},
						Rules: []rbacv1.PolicyRule{
							{
								Verbs:     []string{"list"},
								Resources: []string{"pods"},
								APIGroups: []string{""},
							},
						},
					}, nil
				}
				return &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-projectresources",
						Annotations: map[string]string{
							"authz.cluster.cattle.io/clusterrole-owner": "test",
							"resources.project.cattle.io/authz":         "test",
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							Verbs:     []string{"list"},
							Resources: []string{"pods"},
							APIGroups: []string{"resources.project.cattle.io"},
						},
					},
				}, nil
			},
			wantCreateCalls: 0,
			wantUpdateCalls: 0,
			wantDeleteCalls: 0,
		},
		{
			name: "parent does not exist",
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			},
			getFunc: func(ns, name string) (*rbacv1.ClusterRole, error) {
				return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
			},
			wantCreateCalls: 0,
			wantUpdateCalls: 0,
			wantDeleteCalls: 0,
			wantError:       true,
		},
		{
			name: "conflict on update",
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			},
			getFunc: func(ns, name string) (*rbacv1.ClusterRole, error) {
				if !strings.HasSuffix(name, "-projectresources") {
					return &rbacv1.ClusterRole{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
						Rules: []rbacv1.PolicyRule{
							{
								Verbs:     []string{"list"},
								Resources: []string{"pods"},
								APIGroups: []string{""},
							},
						},
					}, nil
				}
				return &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-projectresources",
						ResourceVersion: "earlier",
						Annotations: map[string]string{
							"authz.cluster.cattle.io/clusterrole-owner": "test",
							"resources.project.cattle.io/authz":         "test",
						},
					},
				}, nil
			},
			setup: func() {
				mockClusterRoles["test-projectresources"] = &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-projectresources",
						ResourceVersion: "later",
						Annotations: map[string]string{
							"authz.cluster.cattle.io/clusterrole-owner": "test",
							"resources.project.cattle.io/authz":         "test",
						},
					},
				}
			},
			wantCreateCalls: 0,
			wantUpdateCalls: 2,
			wantDeleteCalls: 0,
			wantClusterRole: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-projectresources",
					ResourceVersion: "later",
					Annotations: map[string]string{
						"authz.cluster.cattle.io/clusterrole-owner": "test",
						"resources.project.cattle.io/authz":         "test",
					},
				},
				Rules: []rbacv1.PolicyRule{
					{
						Verbs:     []string{"list"},
						Resources: []string{"pods"},
						APIGroups: []string{"resources.project.cattle.io"},
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockClusterRoles = make(map[string]*rbacv1.ClusterRole)
			manager := &manager{
				crLister: &rbacfakes.ClusterRoleListerMock{
					GetFunc: test.getFunc,
				},
				clusterRoles: &rbacfakes.ClusterRoleInterfaceMock{
					CreateFunc: func(in1 *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
						mockClusterRoles[in1.Name] = in1
						return in1, nil
					},
					GetFunc: func(name string, _ metav1.GetOptions) (*rbacv1.ClusterRole, error) {
						return mockClusterRoles[name], nil
					},
					UpdateFunc: func(in1 *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
						if cr, ok := mockClusterRoles[in1.Name]; ok && cr.ObjectMeta.ResourceVersion > in1.ObjectMeta.ResourceVersion {
							return nil, apierrors.NewConflict(schema.GroupResource{}, in1.Name, nil)
						}
						mockClusterRoles[in1.Name] = in1
						return in1, nil
					},
					DeleteFunc: func(_ string, _ *metav1.DeleteOptions) error {
						return nil
					},
				},
				apis: &fakeAPIs{
					resourceMap: map[string]metav1.APIResource{
						"pods":             metav1.APIResource{},
						"apps.deployments": metav1.APIResource{},
					},
				},
			}
			if test.setup != nil {
				test.setup()
			}
			err := manager.ensureProjectResourcesRoles(test.rt)
			if test.wantError {
				assert.Error(t, err)
				return
			}
			assert.Nil(t, err)
			actualCreateCalls := len(manager.clusterRoles.(*rbacfakes.ClusterRoleInterfaceMock).CreateCalls())
			assert.Equal(t, test.wantCreateCalls, actualCreateCalls, fmt.Sprintf("expected %d create calls, got %d", test.wantCreateCalls, actualCreateCalls))
			actualUpdateCalls := len(manager.clusterRoles.(*rbacfakes.ClusterRoleInterfaceMock).UpdateCalls())
			assert.Equal(t, test.wantUpdateCalls, actualUpdateCalls, fmt.Sprintf("expected %d update calls, got %d", test.wantUpdateCalls, actualUpdateCalls))
			actualDeleteCalls := len(manager.clusterRoles.(*rbacfakes.ClusterRoleInterfaceMock).DeleteCalls())
			assert.Equal(t, test.wantDeleteCalls, actualDeleteCalls, fmt.Sprintf("expected %d delete calls, got %d", test.wantDeleteCalls, actualDeleteCalls))
			assert.Equal(t, test.wantClusterRole, mockClusterRoles["test-projectresources"])
		})
	}
}

func Test_ensureProjectResourcesClusterRoleBindings(t *testing.T) {
	var clusterRoleBindings []*rbacv1.ClusterRoleBinding
	tests := []struct {
		name            string
		roles           map[string]*v3.RoleTemplate
		binding         *v3.ClusterRoleTemplateBinding
		cache           []*rbacv1.ClusterRoleBinding
		want            *rbacv1.ClusterRoleBinding
		wantCreateCalls int
		wantDeleteCalls int
	}{
		{
			name: "create new",
			roles: map[string]*v3.RoleTemplate{
				"test": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
			},
			binding: &v3.ClusterRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "cluster1",
				},
				UserName:         "user1",
				ClusterName:      "cluster1",
				RoleTemplateName: "test",
			},
			want: &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: pkgrbac.NameForClusterRoleBinding(rbacv1.RoleRef{Kind: "ClusterRole", Name: "test-projectresources"}, rbacv1.Subject{Kind: "User", Name: "user1"}),
					Annotations: map[string]string{
						"resources.project.cattle.io/authz": "cluster1_test-binding",
					},
					Labels: map[string]string{
						"authz.cluster.cattle.io/rtb-owner-updated": "cluster1_test-binding-projectresources",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "rbac.authorization.k8s.io/v1",
							Kind:       "ClusterRoleBinding",
							Name:       "test-binding",
							UID:        "",
						},
					},
				},
				RoleRef: rbacv1.RoleRef{
					Kind: "ClusterRole",
					Name: "test-projectresources",
				},
				Subjects: []rbacv1.Subject{
					{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "User",
						Name:     "user1",
					},
				},
			},
			wantCreateCalls: 1,
		},
		{
			name: "already exists",
			roles: map[string]*v3.RoleTemplate{
				"test": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
			},
			binding: &v3.ClusterRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "cluster1",
				},
				UserName:         "user1",
				ClusterName:      "cluster1",
				RoleTemplateName: "test",
			},
			cache: []*rbacv1.ClusterRoleBinding{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: pkgrbac.NameForClusterRoleBinding(rbacv1.RoleRef{Kind: "ClusterRole", Name: "test-projectresources"}, rbacv1.Subject{Kind: "User", Name: "user1"}),
						Annotations: map[string]string{
							"resources.project.cattle.io/authz": "cluster1_test-binding",
						},
						Labels: map[string]string{
							"authz.cluster.cattle.io/rtb-owner-updated": "cluster1_test-binding-projectresources",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "rbac.authorization.k8s.io/v1",
								Kind:       "ClusterRoleBinding",
								Name:       "test-binding",
								UID:        "",
							},
						},
					},
					RoleRef: rbacv1.RoleRef{
						Kind: "ClusterRole",
						Name: "test-projectresources",
					},
					Subjects: []rbacv1.Subject{
						{
							APIGroup: "rbac.authorization.k8s.io",
							Kind:     "User",
							Name:     "user1",
						},
					},
				},
			},
			want: &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: pkgrbac.NameForClusterRoleBinding(rbacv1.RoleRef{Kind: "ClusterRole", Name: "test-projectresources"}, rbacv1.Subject{Kind: "User", Name: "user1"}),
					Annotations: map[string]string{
						"resources.project.cattle.io/authz": "cluster1_test-binding",
					},
					Labels: map[string]string{
						"authz.cluster.cattle.io/rtb-owner-updated": "cluster1_test-binding-projectresources",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "rbac.authorization.k8s.io/v1",
							Kind:       "ClusterRoleBinding",
							Name:       "test-binding",
							UID:        "",
						},
					},
				},
				RoleRef: rbacv1.RoleRef{
					Kind: "ClusterRole",
					Name: "test-projectresources",
				},
				Subjects: []rbacv1.Subject{
					{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "User",
						Name:     "user1",
					},
				},
			},
		},
		{
			name: "delete non matching",
			roles: map[string]*v3.RoleTemplate{
				"test": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
			},
			binding: &v3.ClusterRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "cluster1",
				},
				UserName:         "user1",
				ClusterName:      "cluster1",
				RoleTemplateName: "test",
			},
			cache: []*rbacv1.ClusterRoleBinding{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: pkgrbac.NameForClusterRoleBinding(rbacv1.RoleRef{Kind: "ClusterRole", Name: "test-projectresources"}, rbacv1.Subject{Kind: "User", Name: "user1"}),
						Annotations: map[string]string{
							"resources.project.cattle.io/authz": "cluster1_test-binding",
						},
						Labels: map[string]string{
							"authz.cluster.cattle.io/rtb-owner-updated": "cluster1_test-binding-projectresources",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "rbac.authorization.k8s.io/v1",
								Kind:       "ClusterRoleBinding",
								Name:       "test-binding",
								UID:        "",
							},
						},
					},
					RoleRef: rbacv1.RoleRef{
						Kind: "ClusterRole",
						Name: "test-projectresources",
					},
					Subjects: []rbacv1.Subject{
						{
							APIGroup: "rbac.authorization.k8s.io",
							Kind:     "User",
							Name:     "user2",
						},
					},
				},
			},
			want: &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: pkgrbac.NameForClusterRoleBinding(rbacv1.RoleRef{Kind: "ClusterRole", Name: "test-projectresources"}, rbacv1.Subject{Kind: "User", Name: "user1"}),
					Annotations: map[string]string{
						"resources.project.cattle.io/authz": "cluster1_test-binding",
					},
					Labels: map[string]string{
						"authz.cluster.cattle.io/rtb-owner-updated": "cluster1_test-binding-projectresources",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "rbac.authorization.k8s.io/v1",
							Kind:       "ClusterRoleBinding",
							Name:       "test-binding",
							UID:        "",
						},
					},
				},
				RoleRef: rbacv1.RoleRef{
					Kind: "ClusterRole",
					Name: "test-projectresources",
				},
				Subjects: []rbacv1.Subject{
					{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "User",
						Name:     "user1",
					},
				},
			},
			wantCreateCalls: 1,
			wantDeleteCalls: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manager := &manager{
				crbLister: &rbacfakes.ClusterRoleBindingListerMock{
					GetFunc: func(_, _ string) (*rbacv1.ClusterRoleBinding, error) {
						return &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "test-binding"}}, nil
					},
					ListFunc: func(_ string, _ labels.Selector) ([]*rbacv1.ClusterRoleBinding, error) {
						return test.cache, nil
					},
				},
				clusterRoleBindings: &rbacfakes.ClusterRoleBindingInterfaceMock{
					CreateFunc: func(in1 *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
						clusterRoleBindings = append(clusterRoleBindings, in1)
						return in1, nil
					},
					UpdateFunc: func(in1 *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
						clusterRoleBindings = append(clusterRoleBindings, in1)
						return in1, nil
					},
					DeleteFunc: func(name string, _ *metav1.DeleteOptions) error {
						for i, rb := range clusterRoleBindings {
							if rb.Name == name {
								clusterRoleBindings = append(clusterRoleBindings[:i], clusterRoleBindings[i+1:]...)
							}
							return nil
						}
						return apierrors.NewNotFound(schema.GroupResource{}, name)
					},
				},
			}
			if test.cache == nil {
				clusterRoleBindings = make([]*rbacv1.ClusterRoleBinding, 0)
			} else {
				clusterRoleBindings = test.cache
			}
			err := manager.ensureProjectResourcesClusterRoleBindings(test.roles, test.binding)
			assert.Nil(t, err)
			assert.Equal(t, test.want, clusterRoleBindings[len(clusterRoleBindings)-1])
			assert.Equal(t, test.wantCreateCalls, len(manager.clusterRoleBindings.(*rbacfakes.ClusterRoleBindingInterfaceMock).CreateCalls()))
			assert.Equal(t, test.wantDeleteCalls, len(manager.clusterRoleBindings.(*rbacfakes.ClusterRoleBindingInterfaceMock).DeleteCalls()))
		})
	}
}

func Test_ensureProjectResourcesRoleBindings(t *testing.T) {
	var roleBindings map[string][]*rbacv1.RoleBinding
	tests := []struct {
		name            string
		roles           map[string]*v3.RoleTemplate
		binding         *v3.ProjectRoleTemplateBinding
		cache           map[string][]*rbacv1.RoleBinding
		want            *rbacv1.RoleBinding
		wantCreateCalls int
		wantDeleteCalls int
	}{
		{
			name: "create new",
			roles: map[string]*v3.RoleTemplate{
				"test": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
			},
			binding: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "project1",
				},
				UserName:         "user1",
				ProjectName:      "cluster1:project1",
				RoleTemplateName: "test",
			},
			want: &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pkgrbac.NameForRoleBinding("project1", rbacv1.RoleRef{Kind: "ClusterRole", Name: "test-projectresources"}, rbacv1.Subject{Kind: "User", Name: "user1"}),
					Namespace: "project1",
					Annotations: map[string]string{
						"resources.project.cattle.io/authz": "project1_test-binding",
					},
					Labels: map[string]string{
						"authz.cluster.cattle.io/rtb-owner-updated": "project1_test-binding",
					},
				},
				RoleRef: rbacv1.RoleRef{
					Kind: "ClusterRole",
					Name: "test-projectresources",
				},
				Subjects: []rbacv1.Subject{
					{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "User",
						Name:     "user1",
					},
				},
			},
			wantCreateCalls: 1,
		},
		{
			name: "already exists",
			roles: map[string]*v3.RoleTemplate{
				"test": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
			},
			binding: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "project1",
				},
				UserName:         "user1",
				ProjectName:      "cluster1:project1",
				RoleTemplateName: "test",
			},
			cache: map[string][]*rbacv1.RoleBinding{
				"project1": {
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      pkgrbac.NameForRoleBinding("project1", rbacv1.RoleRef{Kind: "ClusterRole", Name: "test-projectresources"}, rbacv1.Subject{Kind: "User", Name: "user1"}),
							Namespace: "project1",
							Annotations: map[string]string{
								"resources.project.cattle.io/authz": "project1_test-binding",
							},
							Labels: map[string]string{
								"authz.cluster.cattle.io/rtb-owner-updated": "project1_test-binding",
							},
						},
						RoleRef: rbacv1.RoleRef{
							Kind: "ClusterRole",
							Name: "test-projectresources",
						},
						Subjects: []rbacv1.Subject{
							{
								APIGroup: "rbac.authorization.k8s.io",
								Kind:     "User",
								Name:     "user1",
							},
						},
					},
				},
			},
			want: &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pkgrbac.NameForRoleBinding("project1", rbacv1.RoleRef{Kind: "ClusterRole", Name: "test-projectresources"}, rbacv1.Subject{Kind: "User", Name: "user1"}),
					Namespace: "project1",
					Annotations: map[string]string{
						"resources.project.cattle.io/authz": "project1_test-binding",
					},
					Labels: map[string]string{
						"authz.cluster.cattle.io/rtb-owner-updated": "project1_test-binding",
					},
				},
				RoleRef: rbacv1.RoleRef{
					Kind: "ClusterRole",
					Name: "test-projectresources",
				},
				Subjects: []rbacv1.Subject{
					{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "User",
						Name:     "user1",
					},
				},
			},
		},
		{
			name: "delete non matching",
			roles: map[string]*v3.RoleTemplate{
				"test": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
			},
			binding: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "project1",
				},
				UserName:         "user1",
				ProjectName:      "cluster1:project1",
				RoleTemplateName: "test",
			},
			cache: map[string][]*rbacv1.RoleBinding{
				"project1": {
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      pkgrbac.NameForRoleBinding("project1", rbacv1.RoleRef{Kind: "ClusterRole", Name: "test-projectresources"}, rbacv1.Subject{Kind: "User", Name: "user2"}),
							Namespace: "project1",
							Annotations: map[string]string{
								"resources.project.cattle.io/authz": "project1_test-binding",
							},
							Labels: map[string]string{
								"authz.cluster.cattle.io/rtb-owner-updated": "project1_test-binding",
							},
						},
						RoleRef: rbacv1.RoleRef{
							Kind: "ClusterRole",
							Name: "test-projectresources",
						},
						Subjects: []rbacv1.Subject{
							{
								APIGroup: "rbac.authorization.k8s.io",
								Kind:     "User",
								Name:     "user2",
							},
						},
					},
				},
			},
			want: &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pkgrbac.NameForRoleBinding("project1", rbacv1.RoleRef{Kind: "ClusterRole", Name: "test-projectresources"}, rbacv1.Subject{Kind: "User", Name: "user1"}),
					Namespace: "project1",
					Annotations: map[string]string{
						"resources.project.cattle.io/authz": "project1_test-binding",
					},
					Labels: map[string]string{
						"authz.cluster.cattle.io/rtb-owner-updated": "project1_test-binding",
					},
				},
				RoleRef: rbacv1.RoleRef{
					Kind: "ClusterRole",
					Name: "test-projectresources",
				},
				Subjects: []rbacv1.Subject{
					{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "User",
						Name:     "user1",
					},
				},
			},
			wantCreateCalls: 1,
			wantDeleteCalls: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manager := &manager{
				rbLister: &rbacfakes.RoleBindingListerMock{
					GetFunc: func(namespace, name string) (*rbacv1.RoleBinding, error) {
						rbs, ok := roleBindings[namespace]
						if !ok {
							return nil, apierrors.NewNotFound(schema.GroupResource{}, namespace)
						}
						for _, rb := range rbs {
							if rb.Name == name {
								return rb, nil
							}
						}
						return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
					},
					ListFunc: func(namespace string, _ labels.Selector) ([]*rbacv1.RoleBinding, error) {
						rbs, ok := roleBindings[namespace]
						if !ok {
							return nil, apierrors.NewNotFound(schema.GroupResource{}, namespace)
						}
						return rbs, nil
					},
				},
				roleBindings: &rbacfakes.RoleBindingInterfaceMock{
					CreateFunc: func(in1 *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
						if roleBindings[in1.Namespace] == nil {
							roleBindings[in1.Namespace] = make([]*rbacv1.RoleBinding, 0)
						}
						roleBindings[in1.Namespace] = append(roleBindings[in1.Namespace], in1)
						return in1, nil
					},
					DeleteNamespacedFunc: func(namespace, name string, _ *metav1.DeleteOptions) error {
						rbs, ok := roleBindings[namespace]
						if !ok {
							return apierrors.NewNotFound(schema.GroupResource{}, namespace)
						}
						for i, rb := range rbs {
							if rb.Name == name {
								roleBindings[namespace] = append(roleBindings[namespace][:i], roleBindings[namespace][i+1:]...)
							}
							return nil
						}
						return apierrors.NewNotFound(schema.GroupResource{}, name)
					},
				},
			}
			ns := "project1"
			if test.cache == nil {
				roleBindings = map[string][]*rbacv1.RoleBinding{
					"project1": nil,
				}
			} else {
				roleBindings = test.cache
			}
			err := manager.ensureProjectResourcesRoleBindings(ns, test.roles, test.binding)
			assert.Nil(t, err)
			assert.Equal(t, test.want, roleBindings[ns][len(roleBindings[ns])-1])
			assert.Equal(t, test.wantCreateCalls, len(manager.roleBindings.(*rbacfakes.RoleBindingInterfaceMock).CreateCalls()))
			assert.Equal(t, test.wantDeleteCalls, len(manager.roleBindings.(*rbacfakes.RoleBindingInterfaceMock).DeleteNamespacedCalls()))
		})
	}
}

type fakeAPIs struct {
	resourceMap map[string]metav1.APIResource
}

func (f *fakeAPIs) List() []metav1.APIResource {
	result := make([]metav1.APIResource, 0)
	for _, v := range f.resourceMap {
		result = append(result, v)
	}
	return result
}

func (f *fakeAPIs) Get(resource, group string) (metav1.APIResource, bool) {
	if group == "" {
		api, ok := f.resourceMap[resource]
		return api, ok
	}
	api, ok := f.resourceMap[group+"."+resource]
	return api, ok
}

func (f *fakeAPIs) GetKindForResource(_ schema.GroupVersionResource) (string, error) {
	panic("not implemented")
}
