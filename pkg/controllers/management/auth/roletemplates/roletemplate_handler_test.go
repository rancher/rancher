package roletemplates

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/rbac/v1"
)

func Test_getManagementPlaneRules(t *testing.T) {
	sampleResources := map[string]string{
		"nodes": "management.cattle.io",
	}

	tests := []struct {
		name                string
		rules               []v1.PolicyRule
		managementResources map[string]string
		want                []v1.PolicyRule
	}{
		{
			name: "no management resources returns empty map",
			rules: []v1.PolicyRule{
				{
					Resources: []string{"pods"},
					APIGroups: []string{"management.cattle.io"},
					Verbs:     []string{"get"},
				},
			},
			managementResources: sampleResources,
			want:                []v1.PolicyRule{},
		},
		{
			name: "rules contains management resource",
			rules: []v1.PolicyRule{
				{
					Resources: []string{"nodes"},
					APIGroups: []string{"management.cattle.io"},
					Verbs:     []string{"get"},
				},
			},
			managementResources: sampleResources,
			want: []v1.PolicyRule{
				{
					Resources: []string{"nodes"},
					APIGroups: []string{"management.cattle.io"},
					Verbs:     []string{"get"},
				},
			},
		},
		{
			name: "rule that contains management resource and rule that contains other resource",
			rules: []v1.PolicyRule{
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
			want: []v1.PolicyRule{
				{
					Resources: []string{"nodes"},
					APIGroups: []string{"management.cattle.io"},
					Verbs:     []string{"get"},
				},
			},
		},
		{
			name: "multiple resources and apigroups in same rule get filtered",
			rules: []v1.PolicyRule{
				{
					Resources: []string{"nodes", "pods"},
					APIGroups: []string{"management.cattle.io", "rbac.cattle.io"},
					Verbs:     []string{"get"},
				},
			},
			managementResources: sampleResources,
			want: []v1.PolicyRule{
				{
					Resources: []string{"nodes"},
					APIGroups: []string{"management.cattle.io"},
					Verbs:     []string{"get"},
				},
			},
		},
		{
			name: "wildcard resources and apigroups get reduced to just management resources",
			rules: []v1.PolicyRule{
				{
					Resources: []string{"*"},
					APIGroups: []string{"*"},
					Verbs:     []string{"get"},
				},
			},
			managementResources: sampleResources,
			want: []v1.PolicyRule{
				{
					Resources: []string{"nodes"},
					APIGroups: []string{"management.cattle.io"},
					Verbs:     []string{"get"},
				},
			},
		},
		{
			name: "if resource names are specified they are not considered management resources",
			rules: []v1.PolicyRule{
				{
					Resources:     []string{"nodes"},
					APIGroups:     []string{"management.cattle.io"},
					Verbs:         []string{"get"},
					ResourceNames: []string{"my-node"},
				},
			},
			managementResources: sampleResources,
			want:                []v1.PolicyRule{},
		},
		{
			name: "get all cluster management plane resources",
			rules: []v1.PolicyRule{
				{
					Resources: []string{"*"},
					APIGroups: []string{"*"},
					Verbs:     []string{"*"},
				},
			},
			managementResources: clusterManagementPlaneResources,
			want: []v1.PolicyRule{
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
			rules: []v1.PolicyRule{
				{
					Resources: []string{"*"},
					APIGroups: []string{"*"},
					Verbs:     []string{"*"},
				},
			},
			managementResources: projectManagementPlaneResources,
			want: []v1.PolicyRule{
				{
					Resources: []string{"apps"},
					APIGroups: []string{"project.cattle.io"},
					Verbs:     []string{"*"},
				},
				{
					Resources: []string{"sourcecodeproviderconfigs"},
					APIGroups: []string{"project.cattle.io"},
					Verbs:     []string{"*"},
				},
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
			assert.ElementsMatch(t, getManagementPlaneRules(tt.rules, tt.managementResources), tt.want)
		})
	}
}
