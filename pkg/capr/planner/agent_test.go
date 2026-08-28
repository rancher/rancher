package planner

import (
	"testing"

	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/management/imported"
	"github.com/rancher/rancher/pkg/features"
	planapi "github.com/rancher/rancher/pkg/plan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Test_generateClusterAgentManifest_stableAcrossDriverFlip covers the regression where the
// cluster agent manifest baked into the node plan changed at the exact moment the agent's
// tunnel connected. authorizeCluster sets the management cluster's Status.Driver to
// "imported" during the tunnel handshake, which used to flip CATTLE_FEATURES and
// CATTLE_SUC_APP_NAME_OVERRIDE. That made the planner see a minor plan change, rewrite
// cluster-agent.yaml, and roll the agent that had just connected.
func Test_generateClusterAgentManifest_stableAcrossDriverFlip(t *testing.T) {
	features.ManagedSystemUpgradeController.Set(true)
	t.Cleanup(features.ManagedSystemUpgradeController.Unset)

	controlPlane := &rkev1.RKEControlPlane{
		Spec: rkev1.RKEControlPlaneSpec{
			ManagementClusterName: "c-m-abc12",
			KubernetesVersion:     "v1.33.5+rke2r1",
		},
	}
	entry := &planEntry{
		Metadata: &plan.Metadata{
			Labels: map[string]string{
				capr.ControlPlaneRoleLabel: "true",
				capr.EtcdRoleLabel:         "true",
			},
		},
	}

	render := func(driver string) []byte {
		mp := newMockPlanner(t, InfoFunctions{})
		mp.clusterRegistrationTokenCache.EXPECT().GetByIndex(ClusterRegToken, "c-m-abc12").
			Return([]*apisv3.ClusterRegistrationToken{{
				ObjectMeta: metav1.ObjectMeta{Name: "default-token"},
			}}, nil)
		mp.managementClusters.EXPECT().Get("c-m-abc12").Return(&apisv3.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "c-m-abc12",
				Annotations: map[string]string{imported.AdministratedAnnotation: "true"},
			},
			Spec: apisv3.ClusterSpec{
				DisplayName:    "testing-rke2",
				ImportedConfig: &apisv3.ImportedConfig{},
			},
			Status: apisv3.ClusterStatus{
				Driver:   driver,
				Provider: "rke2",
			},
		}, nil)

		manifest, err := mp.planner.generateClusterAgentManifest(controlPlane, entry)
		require.NoError(t, err)
		require.NotEmpty(t, manifest)
		return manifest
	}

	assert.Equal(t, string(render("")), string(render("imported")),
		"the cluster agent manifest must not change when Status.Driver flips to imported")
}

// Test_generateClusterAgentManifest_prefersDefaultToken covers the other half of the
// two-writer problem: the token rendered here has to be the one clusterdeploy renders, and
// it must not change when unrelated ClusterRegistrationTokens appear. A crt-* token created
// by generating a registration command sorts ahead of default-token.
func Test_generateClusterAgentManifest_prefersDefaultToken(t *testing.T) {
	controlPlane := &rkev1.RKEControlPlane{
		Spec: rkev1.RKEControlPlaneSpec{
			ManagementClusterName: "c-m-abc12",
			KubernetesVersion:     "v1.33.5+rke2r1",
		},
	}
	entry := &planEntry{Metadata: &plan.Metadata{Labels: map[string]string{capr.ControlPlaneRoleLabel: "true"}}}

	tokenCRT := func(name, secretName string) *apisv3.ClusterRegistrationToken {
		return &apisv3.ClusterRegistrationToken{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "c-m-abc12"},
			Status:     apisv3.ClusterRegistrationTokenStatus{TokenSecretName: secretName},
		}
	}

	render := func(tokens ...*apisv3.ClusterRegistrationToken) []byte {
		mp := newMockPlanner(t, InfoFunctions{})
		mp.clusterRegistrationTokenCache.EXPECT().GetByIndex(ClusterRegToken, "c-m-abc12").Return(tokens, nil)
		mp.managementClusters.EXPECT().Get("c-m-abc12").Return(&apisv3.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "c-m-abc12",
				Annotations: map[string]string{imported.AdministratedAnnotation: "true"},
			},
			Spec: apisv3.ClusterSpec{DisplayName: "testing-rke2"},
		}, nil)
		mp.secretCache.EXPECT().Get("c-m-abc12", gomock.Any()).DoAndReturn(
			func(_, name string) (*corev1.Secret, error) {
				return &corev1.Secret{Data: map[string][]byte{"token": []byte("token-for-" + name)}}, nil
			}).AnyTimes()

		manifest, err := mp.planner.generateClusterAgentManifest(controlPlane, entry)
		require.NoError(t, err)
		require.NotEmpty(t, manifest)
		return manifest
	}

	defaultOnly := render(tokenCRT("default-token", "default-token-secret"))
	// crt-abc sorts before default-token, so a name sort alone would pick the wrong one.
	withExtra := render(
		tokenCRT("crt-abc", "crt-abc-secret"),
		tokenCRT("default-token", "default-token-secret"),
	)

	assert.Equal(t, string(defaultOnly), string(withExtra),
		"an unrelated registration token must not change the rendered credentials")
}

func Test_minorPlanChangeDetected(t *testing.T) {
	majorFile := plan.File{Path: "/etc/rancher/rke2/config.yaml.d/50-rancher.yaml", Content: "old"}
	minorFile := plan.File{Path: "/var/lib/rancher/rke2/server/manifests/rancher/cluster-agent.yaml", Content: "old", Minor: true}

	withContent := func(f plan.File, content string) plan.File {
		f.Content = content
		return f
	}

	tests := []struct {
		name     string
		old, new plan.NodePlan
		expected bool
	}{
		{
			name:     "no files at all",
			expected: false,
		},
		{
			name:     "identical plans",
			old:      plan.NodePlan{Files: []plan.File{majorFile, minorFile}},
			new:      plan.NodePlan{Files: []plan.File{majorFile, minorFile}},
			expected: false,
		},
		{
			// The bug's signature: only the cluster agent manifest changed.
			name:     "minor file content changed",
			old:      plan.NodePlan{Files: []plan.File{majorFile, minorFile}},
			new:      plan.NodePlan{Files: []plan.File{majorFile, withContent(minorFile, "new")}},
			expected: true,
		},
		{
			name:     "minor file added",
			old:      plan.NodePlan{Files: []plan.File{majorFile}},
			new:      plan.NodePlan{Files: []plan.File{majorFile, minorFile}},
			expected: true,
		},
		{
			name:     "minor file removed",
			old:      plan.NodePlan{Files: []plan.File{majorFile, minorFile}},
			new:      plan.NodePlan{Files: []plan.File{majorFile}},
			expected: false,
		},
		{
			name:     "major file content changed",
			old:      plan.NodePlan{Files: []plan.File{majorFile, minorFile}},
			new:      plan.NodePlan{Files: []plan.File{withContent(majorFile, "new"), minorFile}},
			expected: false,
		},
		{
			name: "instructions changed",
			old: plan.NodePlan{
				Files:        []plan.File{minorFile},
				Instructions: []plan.OneTimeInstruction{{CommonInstruction: planapi.CommonInstruction{Name: "install"}}},
			},
			new: plan.NodePlan{
				Files: []plan.File{withContent(minorFile, "new")},
				Instructions: []plan.OneTimeInstruction{
					{CommonInstruction: planapi.CommonInstruction{Name: "install"}},
					{CommonInstruction: planapi.CommonInstruction{Name: "restart"}},
				},
			},
			expected: false,
		},
		{
			name: "probes changed",
			old: plan.NodePlan{
				Files:  []plan.File{minorFile},
				Probes: map[string]plan.Probe{"kubelet": {}},
			},
			new: plan.NodePlan{
				Files:  []plan.File{withContent(minorFile, "new")},
				Probes: map[string]plan.Probe{"kubelet": {}, "etcd": {}},
			},
			expected: false,
		},
		{
			name:     "error changed",
			old:      plan.NodePlan{Files: []plan.File{minorFile}},
			new:      plan.NodePlan{Files: []plan.File{withContent(minorFile, "new")}, Error: "boom"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, minorPlanChangeDetected(tt.old, tt.new))
		})
	}
}
