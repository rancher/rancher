package systemtemplate

import (
	"bytes"
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/imported"
	"github.com/rancher/rancher/pkg/controllers/management/importedclusterversionmanagement"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const capiOwnedLabel = "cluster-api.cattle.io/owned"

// setMSUCFeature toggles the managed-system-upgrade-controller feature for the duration
// of a test.
func setMSUCFeature(t *testing.T, enabled bool) {
	t.Helper()
	features.ManagedSystemUpgradeController.Set(enabled)
	t.Cleanup(features.ManagedSystemUpgradeController.Unset)
}

func TestGetDesiredFeatures(t *testing.T) {
	msucName := features.ManagedSystemUpgradeController.Name()

	tests := []struct {
		name        string
		msucFeature bool
		cluster     *apimgmtv3.Cluster
		expectMSUC  bool
	}{
		// The regression this guards: for a v2prov cluster Status.Driver is "" until the
		// cluster agent's tunnel is authorized and only then becomes "imported". Both
		// states must render the same feature set, otherwise the CAPR planner sees the
		// cluster-agent manifest change at the instant the agent connects and rolls it.
		{
			name:        "v2prov before agent connects (Driver empty)",
			msucFeature: true,
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "c-m-abc12",
					Annotations: map[string]string{imported.AdministratedAnnotation: "true"},
				},
				Status: apimgmtv3.ClusterStatus{Provider: "rke2"},
			},
			expectMSUC: true,
		},
		{
			name:        "v2prov after agent connects (Driver imported)",
			msucFeature: true,
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "c-m-abc12",
					Annotations: map[string]string{imported.AdministratedAnnotation: "true"},
				},
				Status: apimgmtv3.ClusterStatus{Driver: "imported", Provider: "rke2"},
			},
			expectMSUC: true,
		},
		{
			name:        "v2prov before Status.Provider is populated",
			msucFeature: true,
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "c-m-abc12",
					Annotations: map[string]string{imported.AdministratedAnnotation: "true"},
				},
			},
			expectMSUC: true,
		},
		{
			// The node-driver/custom branch intentionally ignores the feature flag:
			// Rancher needs SUC in order to upgrade a v2prov cluster's Kubernetes version.
			name:        "v2prov with the MSUC feature disabled",
			msucFeature: false,
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "c-m-abc12",
					Annotations: map[string]string{imported.AdministratedAnnotation: "true"},
				},
				Status: apimgmtv3.ClusterStatus{Driver: "imported", Provider: "rke2"},
			},
			expectMSUC: true,
		},
		{
			name:        "imported rke2, version management on",
			msucFeature: true,
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "c-abc12",
					Annotations: map[string]string{
						importedclusterversionmanagement.VersionManagementAnno: "true",
					},
				},
				Status: apimgmtv3.ClusterStatus{Driver: "rke2", Provider: "rke2"},
			},
			expectMSUC: true,
		},
		{
			name:        "imported rke2, version management off",
			msucFeature: true,
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "c-abc12",
					Annotations: map[string]string{
						importedclusterversionmanagement.VersionManagementAnno: "false",
					},
				},
				Status: apimgmtv3.ClusterStatus{Driver: "rke2", Provider: "rke2"},
			},
			expectMSUC: false,
		},
		{
			name:        "imported rke2, version management off but CAPI owned",
			msucFeature: true,
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "c-abc12",
					Labels: map[string]string{capiOwnedLabel: "true"},
					Annotations: map[string]string{
						importedclusterversionmanagement.VersionManagementAnno: "false",
					},
				},
				Status: apimgmtv3.ClusterStatus{Driver: "rke2", Provider: "rke2"},
			},
			expectMSUC: true,
		},
		{
			name:        "imported rke2 with the MSUC feature disabled",
			msucFeature: false,
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "c-abc12",
					Annotations: map[string]string{
						importedclusterversionmanagement.VersionManagementAnno: "true",
					},
				},
				Status: apimgmtv3.ClusterStatus{Driver: "rke2", Provider: "rke2"},
			},
			expectMSUC: false,
		},
		{
			// An imported cluster passes through Driver=="imported" on its way to
			// Driver=="rke2". Without the annotation it must not be treated as v2prov,
			// otherwise MSUC flips on and then back off, redeploying the agent twice.
			name:        "imported rke2 transiting Driver imported is not treated as v2prov",
			msucFeature: true,
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "c-abc12",
					Annotations: map[string]string{
						importedclusterversionmanagement.VersionManagementAnno: "false",
					},
				},
				Status: apimgmtv3.ClusterStatus{Driver: "imported", Provider: "rke2"},
			},
			expectMSUC: false,
		},
		{
			name:        "CAPI/turtles cluster is not administrated",
			msucFeature: true,
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "c-m-abc12",
					Annotations: map[string]string{imported.AdministratedAnnotation: "false"},
				},
				Status: apimgmtv3.ClusterStatus{Provider: "rke2"},
			},
			expectMSUC: false,
		},
		{
			name:        "hosted cluster",
			msucFeature: true,
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "c-abc12"},
				Status:     apimgmtv3.ClusterStatus{Driver: "EKS", Provider: "eks"},
			},
			expectMSUC: false,
		},
		{
			name:        "harvester imported cluster",
			msucFeature: true,
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "c-abc12"},
				Status:     apimgmtv3.ClusterStatus{Driver: "imported", Provider: "harvester"},
			},
			expectMSUC: false,
		},
		{
			name:        "local cluster",
			msucFeature: true,
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "local"},
				Spec:       apimgmtv3.ClusterSpec{Internal: true},
				Status:     apimgmtv3.ClusterStatus{Driver: "imported", Provider: "local"},
			},
			expectMSUC: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setMSUCFeature(t, tt.msucFeature)

			got := GetDesiredFeatures(tt.cluster)
			assert.Equal(t, tt.expectMSUC, got[msucName], "%s should be %v", msucName, tt.expectMSUC)
		})
	}
}

// TestGetDesiredFeatures_stableAcrossDriverFlip asserts the invariant directly: nothing in
// the desired feature set may depend on Status.Driver flipping from "" to "imported" when
// the cluster agent's tunnel is authorized.
func TestGetDesiredFeatures_stableAcrossDriverFlip(t *testing.T) {
	setMSUCFeature(t, true)

	before := GetDesiredFeatures(administratedCluster(""))
	after := GetDesiredFeatures(administratedCluster("imported"))

	assert.Equal(t, before, after)
}

// TestSystemTemplate_stableAcrossDriverFlip renders the whole cluster agent manifest either
// side of the tunnel-connect Status.Driver flip and asserts it is byte-identical. The CAPR
// planner writes this manifest into the node plan, so any difference here is a minor plan
// change that rewrites the file and rolls the agent that just connected.
func TestSystemTemplate_stableAcrossDriverFlip(t *testing.T) {
	setMSUCFeature(t, true)

	render := func(driver string) string {
		cluster := administratedCluster(driver)
		var b bytes.Buffer
		err := SystemTemplate(&b, &TemplateOps{
			AgentImage:    "rancher/rancher-agent:v2.12.0",
			AssetsImage:   "rancher/assets:charts",
			Namespace:     cluster.Name,
			Token:         "test-token",
			URL:           "https://rancher.example.com",
			Cluster:       cluster,
			AgentFeatures: GetDesiredFeatures(cluster),
			Mutator:       namespace.Mutator{},
		})
		require.NoError(t, err)
		return b.String()
	}

	assert.Equal(t, render(""), render("imported"))
}

func administratedCluster(driver string) *apimgmtv3.Cluster {
	return &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "c-m-abc12",
			Annotations: map[string]string{imported.AdministratedAnnotation: "true"},
		},
		Spec: apimgmtv3.ClusterSpec{
			DisplayName:    "testing-rke2",
			ImportedConfig: &apimgmtv3.ImportedConfig{},
		},
		Status: apimgmtv3.ClusterStatus{
			Driver:   driver,
			Provider: "rke2",
		},
	}
}
