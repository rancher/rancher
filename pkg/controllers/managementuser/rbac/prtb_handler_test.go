package rbac

import (
	"fmt"
	"testing"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/api/rbac/v1"
)

func Test_manager_checkForGlobalResourceRules(t *testing.T) {
	type tests struct {
		name     string
		role     *v3.RoleTemplate
		resource string
		baseRule rbacv1.PolicyRule
		want     map[string]bool
	}

	testCases := []tests{
		{
			name: "valid_api_group_persistentvolumes",
			role: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"put"},
						APIGroups: []string{""},
						Resources: []string{"persistentvolumes"},
					},
				},
			},
			resource: "persistentvolumes",
			baseRule: rbacv1.PolicyRule{},
			want:     map[string]bool{"put": true},
		},
		{
			name: "invalid_api_group_persistentvolumes",
			role: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"put"},
						APIGroups: []string{"foo"},
						Resources: []string{"persistentvolumes"},
					},
				},
			},
			resource: "persistentvolumes",
			baseRule: rbacv1.PolicyRule{},
			want:     map[string]bool{},
		},
		{
			name: "valid_api_group_storageclasses",
			role: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"put"},
						APIGroups: []string{"storage.k8s.io"},
						Resources: []string{"storageclasses"},
					},
				},
			},
			resource: "storageclasses",
			baseRule: rbacv1.PolicyRule{},
			want:     map[string]bool{"put": true},
		},
		{
			name: "invalid_api_group_storageclasses",
			role: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"put"},
						APIGroups: []string{"foo"},
						Resources: []string{"storageclasses"},
					},
				},
			},
			resource: "storageclasses",
			baseRule: rbacv1.PolicyRule{},
			want:     map[string]bool{},
		},
		{
			name: "valid_api_group_start",
			role: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"put"},
						APIGroups: []string{""},
						Resources: []string{"*"},
					},
				},
			},
			resource: "persistentvolumes",
			baseRule: rbacv1.PolicyRule{},
			want:     map[string]bool{"put": true},
		},
		{
			name: "invalid_api_group_star",
			role: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"put"},
						APIGroups: []string{"foo"},
						Resources: []string{"*"},
					},
				},
			},
			resource: "persistentvolumes",
			baseRule: rbacv1.PolicyRule{},
			want:     map[string]bool{},
		},
		{
			name: "cluster_rule_match",
			role: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"get"},
						APIGroups: []string{"management.cattle.io"},
						Resources: []string{"clusters"},
					},
				},
			},
			resource: "clusters",
			baseRule: rbacv1.PolicyRule{},
			want:     map[string]bool{"get": true},
		},
		{
			name: "cluster_rule_resource_names_match",
			role: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:         []string{"get"},
						APIGroups:     []string{"management.cattle.io"},
						Resources:     []string{"clusters"},
						ResourceNames: []string{"local"},
					},
				},
			},
			resource: "clusters",
			baseRule: rbacv1.PolicyRule{
				ResourceNames: []string{"local"},
			},
			want: map[string]bool{"get": true},
		},
		{
			name: "cluster_rule_baserule_resource_names_no_match",
			role: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"get"},
						APIGroups: []string{"management.cattle.io"},
						Resources: []string{"clusters"},
					},
				},
			},
			resource: "clusters",
			baseRule: rbacv1.PolicyRule{
				ResourceNames: []string{"local"},
			},
			want: map[string]bool{},
		},
		{
			name: "cluster_rule_roletemplate_resource_names_no_match",
			role: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:         []string{"get"},
						APIGroups:     []string{"management.cattle.io"},
						Resources:     []string{"clusters"},
						ResourceNames: []string{"local"},
					},
				},
			},
			resource: "clusters",
			baseRule: rbacv1.PolicyRule{},
			want:     map[string]bool{},
		},
	}

	m := &manager{}

	for _, test := range testCases {
		got, err := m.checkForGlobalResourceRules(test.role, test.resource, test.baseRule)
		assert.Nil(t, err)
		assert.Equal(t, test.want, got, fmt.Sprintf("test %v failed", test.name))
	}
}
