package rbac

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	wname "github.com/rancher/wrangler/v3/pkg/name"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const longGlobalRoleName = "my-custom-role-for-project-x-with-a-very-long-descriptive-name-here"

var inheritedTestRules = []rbacv1.PolicyRule{{
	APIGroups: []string{""},
	Resources: []string{"pods"},
	Verbs:     []string{"get", "list"},
}}

func TestInheritedRoleName(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "nsgrole3-testns3", InheritedRoleName("nsgrole3", "testns3"))
	// a name long enough to be truncated must hash the raw name joined with the namespace,
	// not a pre-truncated version of the name
	assert.Equal(t, wname.SafeConcatName(longGlobalRoleName, "testns3"), InheritedRoleName(longGlobalRoleName, "testns3"))
}

func TestBuildInheritedRole(t *testing.T) {
	t.Parallel()

	gr := &v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{Name: "nsgrole3"},
		InheritedNamespacedRules: map[string][]rbacv1.PolicyRule{
			"testns3": inheritedTestRules,
		},
	}

	role := BuildInheritedRole(gr, "testns3")

	assert.Equal(t, "nsgrole3-testns3", role.Name)
	assert.Equal(t, "testns3", role.Namespace)
	assert.Equal(t, map[string]string{GrOwnerLabel: "nsgrole3"}, role.Labels)
	assert.Equal(t, inheritedTestRules, role.Rules)
}

func TestBuildInheritedRoleBinding(t *testing.T) {
	t.Parallel()

	grb := &v3.GlobalRoleBinding{
		ObjectMeta:     metav1.ObjectMeta{Name: "grb1"},
		GlobalRoleName: "nsgrole3",
		UserName:       "user-abc",
	}

	rb := BuildInheritedRoleBinding(grb, "testns3")

	assert.Equal(t, "grb1-testns3", rb.Name)
	assert.Equal(t, "testns3", rb.Namespace)
	assert.Equal(t, map[string]string{GrbOwnerLabel: "grb1"}, rb.Labels)
	assert.Equal(t, []rbacv1.Subject{{
		Kind:     "User",
		Name:     "user-abc",
		APIGroup: rbacv1.GroupName,
	}}, rb.Subjects)
	assert.Equal(t, rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "Role",
		Name:     "nsgrole3-testns3",
	}, rb.RoleRef)
}

// TestBuildInheritedRoleBindingReferencesRoleForLongGlobalRoleName pins the invariant that broke
// with hand-rolled names: for GlobalRole names long enough to be truncated, the RoleBinding must
// still reference the Role by the exact name the Role is created with.
func TestBuildInheritedRoleBindingReferencesRoleForLongGlobalRoleName(t *testing.T) {
	t.Parallel()

	gr := &v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{Name: longGlobalRoleName},
		InheritedNamespacedRules: map[string][]rbacv1.PolicyRule{
			"testns3": inheritedTestRules,
		},
	}
	grb := &v3.GlobalRoleBinding{
		ObjectMeta:     metav1.ObjectMeta{Name: "grb1"},
		GlobalRoleName: longGlobalRoleName,
		UserName:       "user-abc",
	}

	role := BuildInheritedRole(gr, "testns3")
	rb := BuildInheritedRoleBinding(grb, "testns3")

	assert.Equal(t, role.Name, rb.RoleRef.Name)
}

func TestAreRolesSame(t *testing.T) {
	t.Parallel()

	base := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nsgrole3-testns3",
			Namespace: "testns3",
			Labels:    map[string]string{GrOwnerLabel: "nsgrole3"},
		},
		Rules: inheritedTestRules,
	}

	assert.True(t, AreRolesSame(base, base.DeepCopy()))

	differentRules := base.DeepCopy()
	differentRules.Rules = []rbacv1.PolicyRule{{APIGroups: []string{"apps"}, Resources: []string{"deployments"}, Verbs: []string{"get"}}}
	assert.False(t, AreRolesSame(base, differentRules))

	differentLabels := base.DeepCopy()
	differentLabels.Labels = map[string]string{GrOwnerLabel: "other"}
	assert.False(t, AreRolesSame(base, differentLabels))
}
