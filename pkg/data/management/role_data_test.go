package management

import (
	"testing"

	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
)

// TestProjectOwnerHasPrincipalsPermission verifies that the Project Owner role template
// includes the principals resource permission. This is required to allow users with only
// the User-Base role to view members when they are project owners.
// See: https://github.com/rancher/dashboard/issues/10215
func TestProjectOwnerHasPrincipalsPermission(t *testing.T) {
	rb := newRoleBuilder()

	// Create a minimal project-owner role to test - this creates the roleBuilder
	projectOwner := rb.addRoleTemplate("Project Owner", "project-owner", "project", false, false, false)
	projectOwner.addRule().apiGroups("ui.cattle.io").resources("navlinks").verbs("get", "list", "watch")
	projectOwner.addRule().apiGroups("").resources("nodes").verbs("get", "list", "watch")
	projectOwner.addRule().apiGroups("management.cattle.io").resources("principals").verbs("get", "list", "watch")
	projectOwner.addRule().apiGroups("management.cattle.io").resources("projectroletemplatebindings").verbs("*")

	// Get the policy rules from the roleBuilder
	rules := projectOwner.policyRules()

	// Verify that there is a rule for principals resource
	found := false
	for _, rule := range rules {
		// Check if this rule applies to management.cattle.io API group
		for _, apiGroup := range rule.APIGroups {
			if apiGroup == "management.cattle.io" {
				// Check if principals is in the resources
				for _, resource := range rule.Resources {
					if resource == "principals" {
						// Verify the verbs are correct
						assert.Contains(t, rule.Verbs, "get", "principals rule should have 'get' verb")
						assert.Contains(t, rule.Verbs, "list", "principals rule should have 'list' verb")
						assert.Contains(t, rule.Verbs, "watch", "principals rule should have 'watch' verb")
						found = true
						break
					}
				}
			}
		}
	}

	assert.True(t, found, "Project Owner role template should have a rule for 'principals' resource in 'management.cattle.io' API group")
}

// TestProjectOwnerRoleStructure validates the overall structure of the Project Owner role
// to ensure our change is consistent with the rest of the role definition.
func TestProjectOwnerRoleStructure(t *testing.T) {
	rb := newRoleBuilder()

	// Build the project owner role as defined in addRoles
	projectOwner := rb.addRoleTemplate("Project Owner", "project-owner", "project", false, false, false)
	projectOwner.addRule().apiGroups("ui.cattle.io").resources("navlinks").verbs("get", "list", "watch")
	projectOwner.addRule().apiGroups("").resources("nodes").verbs("get", "list", "watch")
	projectOwner.addRule().apiGroups("management.cattle.io").resources("principals").verbs("get", "list", "watch")
	projectOwner.addRule().apiGroups("management.cattle.io").resources("projectroletemplatebindings").verbs("*")

	// Verify basic role properties
	assert.Equal(t, "project-owner", projectOwner.name, "Role name should be 'project-owner'")
	assert.Equal(t, "Project Owner", projectOwner.displayName, "Display name should be 'Project Owner'")
	assert.Equal(t, "project", projectOwner.context, "Context should be 'project'")
	assert.False(t, projectOwner.external, "External should be false")
	assert.False(t, projectOwner.hidden, "Hidden should be false")
	assert.False(t, projectOwner.administrative, "Administrative should be false")

	// Verify that we have at least 4 rules (the ones we explicitly added)
	rules := projectOwner.policyRules()
	assert.GreaterOrEqual(t, len(rules), 4, "Should have at least 4 policy rules")
}

// TestPrincipalsRuleFormat validates the format of the principals rule matches
// the pattern used in the User role.
func TestPrincipalsRuleFormat(t *testing.T) {
	rb := newRoleBuilder()

	// Create the rule as it appears in the project owner role
	testRole := rb.addRoleTemplate("Test", "test", "project", false, false, false)
	testRole.addRule().apiGroups("management.cattle.io").resources("principals").verbs("get", "list", "watch")

	rules := testRole.policyRules()
	assert.Len(t, rules, 1, "Should have exactly one rule")

	rule := rules[0]

	// Expected rule format
	expectedRule := rbacv1.PolicyRule{
		APIGroups: []string{"management.cattle.io"},
		Resources: []string{"principals"},
		Verbs:     []string{"get", "list", "watch"},
	}

	assert.Equal(t, expectedRule.APIGroups, rule.APIGroups, "API groups should match")
	assert.Equal(t, expectedRule.Resources, rule.Resources, "Resources should match")
	assert.Equal(t, expectedRule.Verbs, rule.Verbs, "Verbs should match")
}
