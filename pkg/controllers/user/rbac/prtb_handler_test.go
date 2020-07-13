package rbac

import (
	"fmt"
	"testing"

	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/rbac/v1"
)

func Test_manager_checkForGlobalResourceRules(t *testing.T) {
	type tests struct {
		name     string
		role     *v3.RoleTemplate
		resource string
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
			want:     map[string]bool{},
		},
	}

	m := &manager{}

	for _, test := range testCases {
		got, err := m.checkForGlobalResourceRules(test.role, test.resource)
		assert.Nil(t, err)
		assert.Equal(t, test.want, got, fmt.Sprintf("test %v failed", test.name))
	}
}
