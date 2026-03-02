package fleetworkspace

import (
	"testing"

	mgmt "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestOnChange(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		workspace       *mgmt.FleetWorkspace
		wantNoObjects   bool
		wantAnnotations map[string]string
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
			wantAnnotations: map[string]string{},
			wantLabels:      map[string]string{},
		},
		"cattle.io annotations are stripped from namespace": {
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
			wantAnnotations: map[string]string{
				"example.com/keep": "should-be-kept",
			},
			wantLabels: map[string]string{},
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
			wantAnnotations: map[string]string{},
			wantLabels: map[string]string{
				"example.com/keep": "should-be-kept",
			},
		},
		"kubectl.kubernetes.io annotations are stripped from namespace": {
			workspace: &mgmt.FleetWorkspace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fleet-default",
					Annotations: map[string]string{
						"kubectl.kubernetes.io/last-applied-configuration": "should-be-stripped",
						"example.com/keep": "should-be-kept",
					},
				},
			},
			wantAnnotations: map[string]string{
				"example.com/keep": "should-be-kept",
			},
			wantLabels: map[string]string{},
		},
		"non-cattle annotations and labels are passed through to namespace": {
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
			wantAnnotations: map[string]string{
				"owner":            "team-a",
				"cost-center":      "12345",
				"example.com/keep": "yes",
			},
			wantLabels: map[string]string{
				"team":    "team-a",
				"project": "example",
			},
		},
		"mix of cattle.io and user-defined annotations: only user-defined survive": {
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
			wantAnnotations: map[string]string{
				"owner": "team-a",
			},
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
			wantAnnotations: map[string]string{},
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

			require.Len(t, objs, 1)
			ns, ok := objs[0].(*corev1.Namespace)
			require.True(t, ok, "expected returned object to be a *corev1.Namespace")

			assert.Equal(t, tc.workspace.Name, ns.Name)
			assert.Equal(t, tc.wantAnnotations, ns.Annotations)
			assert.Equal(t, tc.wantLabels, ns.Labels)
		})
	}
}
