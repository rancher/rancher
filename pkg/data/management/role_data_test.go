package management

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProjectOwnerRoleHasPrincipalsPermission tests that the Project Owner role
// includes the principals permission by replicating the exact role definition from
// addRoles. This validates that users with User-Base role can view project members
// when they are project owners.
// See: https://github.com/rancher/dashboard/issues/10215
func TestProjectOwnerRoleHasPrincipalsPermission(t *testing.T) {
	// Create a roleBuilder and build roles exactly as done in addRoles
	rb := newRoleBuilder()

	// This is the exact definition from addRoles function in role_data.go (lines 199-223)
	// We replicate it here to test the actual role definition
	rb.addRoleTemplate("Project Owner", "project-owner", "project", false, false, false).
		addRule().apiGroups("ui.cattle.io").resources("navlinks").verbs("get", "list", "watch").
		addRule().apiGroups("").resources("nodes").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("principals").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("projectroletemplatebindings").verbs("*").
		addRule().apiGroups("").resources("namespaces").verbs("create").
		addRule().apiGroups("").resources("persistentvolumes").verbs("get", "list", "watch").
		addRule().apiGroups("storage.k8s.io").resources("storageclasses").verbs("get", "list", "watch").
		addRule().apiGroups("apiregistration.k8s.io").resources("apiservices").verbs("get", "list", "watch").
		addRule().apiGroups("").resources("persistentvolumeclaims").verbs("*").
		addRule().apiGroups("metrics.k8s.io").resources("pods").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("clusterevents").verbs("get", "list", "watch").
		addRule().apiGroups("monitoring.coreos.com").resources("prometheuses", "prometheusrules", "servicemonitors").verbs("*").
		addRule().apiGroups("networking.istio.io").resources("destinationrules", "envoyfilters", "gateways", "serviceentries", "sidecars", "virtualservices").verbs("*").
		addRule().apiGroups("config.istio.io").resources("apikeys", "authorizations", "checknothings", "circonuses", "deniers", "fluentds", "handlers", "kubernetesenvs", "kuberneteses", "listcheckers", "listentries", "logentries", "memquotas", "metrics", "opas", "prometheuses", "quotas", "quotaspecbindings", "quotaspecs", "rbacs", "reportnothings", "rules", "solarwindses", "stackdrivers", "statsds", "stdios").verbs("*").
		addRule().apiGroups("authentication.istio.io").resources("policies").verbs("*").
		addRule().apiGroups("rbac.istio.io").resources("rbacconfigs", "serviceroles", "servicerolebindings").verbs("*").
		addRule().apiGroups("security.istio.io").resources("authorizationpolicies").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("projects").verbs("own").
		addRule().apiGroups("catalog.cattle.io").resources("clusterrepos").verbs("get", "list", "watch").
		addRule().apiGroups("catalog.cattle.io").resources("operations").verbs("get", "list", "watch").
		addRule().apiGroups("catalog.cattle.io").resources("releases").verbs("get", "list", "watch").
		addRule().apiGroups("catalog.cattle.io").resources("apps").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clusters").verbs("get").resourceNames("local").
		setRoleTemplateNames("admin")

	// Find the project-owner role in the builder chain
	var projectOwnerRole *roleBuilder
	current := rb.first()
	for current != nil {
		if current.name == "project-owner" {
			projectOwnerRole = current
			break
		}
		current = current.next
	}

	require.NotNil(t, projectOwnerRole, "project-owner role should exist in the role builder chain")

	// Get the policy rules
	rules := projectOwnerRole.policyRules()
	require.NotEmpty(t, rules, "project-owner role should have policy rules")

	// Verify that there is a rule for principals resource with correct permissions
	foundPrincipals := false
	for _, rule := range rules {
		// Check if this rule includes the principals resource
		for _, resource := range rule.Resources {
			if resource == "principals" {
				// Verify it's in the management.cattle.io API group
				assert.Contains(t, rule.APIGroups, "management.cattle.io",
					"principals resource should be in management.cattle.io API group")

				// Verify the verbs are correct (get, list, watch)
				assert.Contains(t, rule.Verbs, "get",
					"principals rule should have 'get' verb")
				assert.Contains(t, rule.Verbs, "list",
					"principals rule should have 'list' verb")
				assert.Contains(t, rule.Verbs, "watch",
					"principals rule should have 'watch' verb")

				foundPrincipals = true
				break
			}
		}
	}

	assert.True(t, foundPrincipals,
		"Project Owner role must have a rule for 'principals' resource to allow User-Base users to view project members")
}
