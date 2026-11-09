package clusterupstreamrefresher

import (
	"testing"

	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
)

func strPtr(s string) *string { return &s }

func TestIsImmutableUpstreamField(t *testing.T) {
	tests := []struct {
		name        string
		cloudDriver string
		field       string
		expected    bool
	}{
		{
			name:        "eks ipFamily is immutable",
			cloudDriver: apimgmtv3.ClusterDriverEKS,
			field:       "ipFamily",
			expected:    true,
		},
		{
			name:        "eks kubernetesVersion is not treated as immutable",
			cloudDriver: apimgmtv3.ClusterDriverEKS,
			field:       "kubernetesVersion",
			expected:    false,
		},
		{
			name:        "ipFamily on non-eks driver is not immutable",
			cloudDriver: apimgmtv3.ClusterDriverAKS,
			field:       "ipFamily",
			expected:    false,
		},
		{
			name:        "unknown field is not immutable",
			cloudDriver: apimgmtv3.ClusterDriverEKS,
			field:       "someOtherField",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isImmutableUpstreamField(tt.cloudDriver, tt.field))
		})
	}
}

// toUnstructured mirrors what refreshClusterUpstreamSpec does when preparing the
// spec and upstream maps, so the tests exercise the same omitempty behavior as
// production.
func toUnstructured(t *testing.T, spec *eksv1.EKSClusterConfigSpec) map[string]interface{} {
	t.Helper()
	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(spec)
	require.NoError(t, err)
	return m
}

func TestMergeUpstreamSpec_EKSIPFamily(t *testing.T) {
	t.Run("seeds ipFamily onto imported cluster with unset spec value", func(t *testing.T) {
		// imported cluster: ipFamily is nil on the spec, so it is omitted from the map
		specMap := toUnstructured(t, &eksv1.EKSClusterConfigSpec{
			DisplayName: "imported-cluster",
			Region:      "us-east-1",
			Imported:    true,
		})
		// upstream (read from AWS) reports the immutable ipFamily
		upstreamMap := toUnstructured(t, &eksv1.EKSClusterConfigSpec{
			DisplayName: "imported-cluster",
			Region:      "us-east-1",
			Imported:    true,
			IPFamily:    strPtr("ipv6"),
		})

		changed := mergeUpstreamSpec(apimgmtv3.ClusterDriverEKS, specMap, upstreamMap)

		assert.True(t, changed, "expected spec to change when seeding ipFamily")
		assert.Equal(t, "ipv6", specMap["ipFamily"])

		// confirm the merged map round-trips back onto the spec
		merged := &eksv1.EKSClusterConfigSpec{}
		require.NoError(t, runtime.DefaultUnstructuredConverter.FromUnstructured(specMap, merged))
		require.NotNil(t, merged.IPFamily)
		assert.Equal(t, "ipv6", *merged.IPFamily)
	})

	t.Run("no change when ipFamily already matches upstream", func(t *testing.T) {
		specMap := toUnstructured(t, &eksv1.EKSClusterConfigSpec{
			DisplayName: "cluster",
			IPFamily:    strPtr("ipv4"),
		})
		upstreamMap := toUnstructured(t, &eksv1.EKSClusterConfigSpec{
			DisplayName: "cluster",
			IPFamily:    strPtr("ipv4"),
		})

		changed := mergeUpstreamSpec(apimgmtv3.ClusterDriverEKS, specMap, upstreamMap)

		assert.False(t, changed)
		assert.Equal(t, "ipv4", specMap["ipFamily"])
	})

	t.Run("does not seed ipFamily when upstream value is absent", func(t *testing.T) {
		specMap := toUnstructured(t, &eksv1.EKSClusterConfigSpec{
			DisplayName: "cluster",
		})
		upstreamMap := toUnstructured(t, &eksv1.EKSClusterConfigSpec{
			DisplayName: "cluster",
		})

		changed := mergeUpstreamSpec(apimgmtv3.ClusterDriverEKS, specMap, upstreamMap)

		assert.False(t, changed)
		_, ok := specMap["ipFamily"]
		assert.False(t, ok, "ipFamily should remain unset when upstream has no value")
	})

	t.Run("unset non-immutable field is not seeded from upstream", func(t *testing.T) {
		// kubernetesVersion is unset on the spec; it must not be pulled in by the
		// immutable-field exception since it is not immutable.
		specMap := toUnstructured(t, &eksv1.EKSClusterConfigSpec{
			DisplayName: "cluster",
		})
		upstreamMap := toUnstructured(t, &eksv1.EKSClusterConfigSpec{
			DisplayName:       "cluster",
			KubernetesVersion: strPtr("1.31"),
		})

		changed := mergeUpstreamSpec(apimgmtv3.ClusterDriverEKS, specMap, upstreamMap)

		assert.False(t, changed)
		// kubernetesVersion has no omitempty, so it is present as nil, but must not
		// be seeded from the upstream value since it is not an immutable field.
		assert.Nil(t, specMap["kubernetesVersion"], "unset non-immutable field must not be seeded")
	})

	t.Run("updates an already-set field when upstream differs", func(t *testing.T) {
		specMap := toUnstructured(t, &eksv1.EKSClusterConfigSpec{
			DisplayName:       "cluster",
			KubernetesVersion: strPtr("1.30"),
		})
		upstreamMap := toUnstructured(t, &eksv1.EKSClusterConfigSpec{
			DisplayName:       "cluster",
			KubernetesVersion: strPtr("1.31"),
		})

		changed := mergeUpstreamSpec(apimgmtv3.ClusterDriverEKS, specMap, upstreamMap)

		assert.True(t, changed)
		assert.Equal(t, "1.31", specMap["kubernetesVersion"])
	})
}
