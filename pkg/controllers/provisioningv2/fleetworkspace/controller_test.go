package fleetworkspace

import (
	"testing"

	mgmt "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestOnChangeNamespace verifies that OnChange produces the correct Namespace
// object for each workspace configuration.  Every case here omits a creatorId
// annotation so the focus stays on namespace metadata; the RBAC path is covered
// by TestOnChangeWithCreatorID.
func TestOnChangeNamespace(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		workspace       *mgmt.FleetWorkspace
		wantNoObjects   bool
		wantObjectCount int
		wantLabels      map[string]string
	}{
		"unmanaged workspace returns no objects": {
			workspace: &mgmt.FleetWorkspace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fleet-default",
					Annotations: map[string]string{
						managed: "false",
					},
				},
			},
			wantNoObjects: true,
		},
		"workspace with no labels or annotations produces empty namespace metadata": {
			workspace: &mgmt.FleetWorkspace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fleet-default",
				},
			},
			wantObjectCount: 1,
			wantLabels:      map[string]string{},
		},
		"workspace annotations are not copied to namespace": {
			workspace: &mgmt.FleetWorkspace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fleet-default",
					Annotations: map[string]string{
						"cattle.io/internal":  "should-be-stripped",
						"foo.cattle.io/owner": "also-stripped",
						"example.com/keep":    "should-be-kept",
					},
				},
			},
			wantObjectCount: 1,
			wantLabels:      map[string]string{},
		},
		"cattle.io labels are stripped from namespace": {
			workspace: &mgmt.FleetWorkspace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fleet-default",
					Labels: map[string]string{
						"cattle.io/internal":  "should-be-stripped",
						"foo.cattle.io/owner": "also-stripped",
						"example.com/keep":    "should-be-kept",
					},
				},
			},
			wantObjectCount: 1,
			wantLabels: map[string]string{
				"example.com/keep": "should-be-kept",
			},
		},
		"kubectl.kubernetes.io annotations are not copied to namespace": {
			workspace: &mgmt.FleetWorkspace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fleet-default",
					Annotations: map[string]string{
						"kubectl.kubernetes.io/last-applied-configuration": "should-be-stripped",
						"example.com/keep": "should-be-kept",
					},
				},
			},
			wantObjectCount: 1,
			wantLabels:      map[string]string{},
		},
		"non-cattle labels are passed through to namespace": {
			workspace: &mgmt.FleetWorkspace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fleet-default",
					Annotations: map[string]string{
						"owner":            "team-a",
						"cost-center":      "12345",
						"example.com/keep": "yes",
					},
					Labels: map[string]string{
						"team":    "team-a",
						"project": "example",
					},
				},
			},
			wantObjectCount: 1,
			wantLabels: map[string]string{
				"team":    "team-a",
				"project": "example",
			},
		},
		"mix of cattle.io and user-defined labels: only user-defined survive": {
			workspace: &mgmt.FleetWorkspace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fleet-default",
					Annotations: map[string]string{
						"field.cattle.io/creatorId": "user-xyz",
						"owner":                     "team-a",
					},
					Labels: map[string]string{
						"cattle.io/something": "stripped",
						"environment":         "production",
					},
				},
			},
			wantObjectCount: 4,
			wantLabels: map[string]string{
				"environment": "production",
			},
		},
		"namespace name matches workspace name": {
			workspace: &mgmt.FleetWorkspace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-custom-workspace",
				},
			},
			wantObjectCount: 1,
			wantLabels:      map[string]string{},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			h := &handle{}
			objs, _, err := h.OnChange(tc.workspace, mgmt.FleetWorkspaceStatus{})

			require.NoError(t, err)

			if tc.wantNoObjects {
				assert.Empty(t, objs)
				return
			}

			require.Len(t, objs, tc.wantObjectCount)
			ns, ok := objs[0].(*corev1.Namespace)
			require.True(t, ok, "expected returned object to be a *corev1.Namespace")

			assert.Equal(t, tc.workspace.Name, ns.Name)
			assert.Nil(t, ns.Annotations)
			assert.Equal(t, tc.wantLabels, ns.Labels)
		})
	}
}

// TestOnChangeWithCreatorID verifies that a workspace carrying the
// field.cattle.io/creatorId annotation results in four objects: a Namespace
// plus the three RBAC objects that grant the creator administrative access to
// the workspace.  The creatorId value is set by the Rancher management API from
// the authenticated caller's identity and is not propagated into the namespace
// annotations.
func TestOnChangeWithCreatorID(t *testing.T) {
	t.Parallel()

	h := &handle{}
	workspace := &mgmt.FleetWorkspace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fleet-default",
			Annotations: map[string]string{
				"field.cattle.io/creatorId": "user-xyz",
			},
		},
	}

	objs, _, err := h.OnChange(workspace, mgmt.FleetWorkspaceStatus{})
	require.NoError(t, err)
	require.Len(t, objs, 4)

	// [0] Namespace – this branch's controller does not copy workspace
	// annotations into the child namespace.
	ns, ok := objs[0].(*corev1.Namespace)
	require.True(t, ok)
	assert.Equal(t, "fleet-default", ns.Name)
	assert.Nil(t, ns.Annotations,
		"workspace annotations must not be propagated to the child namespace")

	// [1] RoleBinding – binds the creator to the fleetworkspace-admin ClusterRole
	// in the workspace namespace, granting full access to Fleet resources there.
	adminBinding, ok := objs[1].(*rbacv1.RoleBinding)
	require.True(t, ok)
	assert.Equal(t, "fleetworkspace-admin-binding-fleet-default", adminBinding.Name)
	assert.Equal(t, "fleet-default", adminBinding.Namespace)
	require.Len(t, adminBinding.Subjects, 1)
	assert.Equal(t, "user-xyz", adminBinding.Subjects[0].Name)
	assert.Equal(t, "fleetworkspace-admin", adminBinding.RoleRef.Name)

	// [2] ClusterRole – allows the creator to manage the FleetWorkspace object
	// itself (scoped to this workspace's resource name).
	ownRole, ok := objs[2].(*rbacv1.ClusterRole)
	require.True(t, ok)
	assert.Equal(t, "fleetworkspace-own-fleet-default", ownRole.Name)
	require.Len(t, ownRole.Rules, 1)
	assert.Equal(t, []string{"fleetworkspaces"}, ownRole.Rules[0].Resources)
	assert.Equal(t, []string{"fleet-default"}, ownRole.Rules[0].ResourceNames)

	// [3] ClusterRoleBinding – binds the creator to the ClusterRole above.
	ownBinding, ok := objs[3].(*rbacv1.ClusterRoleBinding)
	require.True(t, ok)
	assert.Equal(t, "fleetworkspace-own-binding-fleet-default", ownBinding.Name)
	require.Len(t, ownBinding.Subjects, 1)
	assert.Equal(t, "user-xyz", ownBinding.Subjects[0].Name)
	assert.Equal(t, "fleetworkspace-own-fleet-default", ownBinding.RoleRef.Name)
}

// TestOnChangeWithoutCreatorIDProducesOnlyNamespace verifies that workspaces
// created without a creatorId (e.g. by the provisioning controller for system
// workspaces) result in a namespace only, with no RBAC objects.
func TestOnChangeWithoutCreatorIDProducesOnlyNamespace(t *testing.T) {
	t.Parallel()

	h := &handle{}
	workspace := &mgmt.FleetWorkspace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fleet-default",
		},
	}

	objs, _, err := h.OnChange(workspace, mgmt.FleetWorkspaceStatus{})
	require.NoError(t, err)
	require.Len(t, objs, 1)

	_, ok := objs[0].(*corev1.Namespace)
	require.True(t, ok, "sole object must be the Namespace")
}
