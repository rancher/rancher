package provisioningcluster

import (
	"encoding/json"
	"testing"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1/snapshotutil"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/jsonmergepatch"
)

// nestedMap walks a decoded chart values tree, failing the test if any hop is
// missing or is not a map.
func nestedMap(t *testing.T, m map[string]any, keys ...string) map[string]any {
	t.Helper()
	cur := m
	for _, k := range keys {
		next, ok := cur[k].(map[string]any)
		require.Truef(t, ok, "expected a map at %q, got %T", k, cur[k])
		cur = next
	}
	return cur
}

// TestChartValuesNullSurvivesInClusterSpecAnnotation reproduces the loss of an
// explicit null between the Cluster and the RKEControlPlane, and shows that the
// ClusterSpecAnnotation already carries the same null across that boundary
// intact.
//
// Wrangler's apply updates an existing RKEControlPlane with a
// types.MergePatchType patch built by jsonmergepatch.CreateThreeWayJSONMergePatch,
// because rke.cattle.io/v1 is a CRD and so is not registered in the client-go
// scheme. RFC 7386 defines a null member in a merge patch as "remove this key",
// so a literal null is inexpressible in any structurally-merged field. A string
// field is replaced atomically and is therefore unaffected -- and the annotation
// is already a gzip+base64 string of the whole cluster spec.
func TestChartValuesNullSurvivesInClusterSpecAnnotation(t *testing.T) {
	// The cluster as it was when the RKEControlPlane was last applied.
	before, err := rkeControlPlane(chartValuesCluster(t, map[string]any{
		"rke2-coredns": map[string]any{
			"resources": map[string]any{
				"limits": map[string]any{"memory": "128Mi"},
			},
		},
	}))
	require.NoError(t, err)

	// The user now sets cpu to an explicit null to drop the chart default.
	after, err := rkeControlPlane(chartValuesCluster(t, map[string]any{
		"rke2-coredns": map[string]any{
			"resources": map[string]any{
				"limits": map[string]any{"cpu": nil, "memory": "130Mi"},
			},
		},
	}))
	require.NoError(t, err)

	original, err := json.Marshal(before)
	require.NoError(t, err)
	modified, err := json.Marshal(after)
	require.NoError(t, err)

	// current is the live object, which here matches what was last applied.
	patch, err := jsonmergepatch.CreateThreeWayJSONMergePatch(original, modified, original)
	require.NoError(t, err)

	patched, err := jsonpatch.MergePatch(original, patch)
	require.NoError(t, err)

	result := &rkev1.RKEControlPlane{}
	require.NoError(t, json.Unmarshal(patched, result))

	t.Run("the structured spec.chartValues field drops the null", func(t *testing.T) {
		limits := nestedMap(t, result.Spec.ChartValues.Data, "rke2-coredns", "resources", "limits")

		// This is the bug: the key the user asked to null out is simply gone, so
		// the rendered HelmChartConfig never tells Helm to remove the default.
		assert.NotContains(t, limits, "cpu")
		// Non-null siblings cross the boundary fine, which is why the failure is
		// silent -- the update looks like it worked.
		assert.Equal(t, "130Mi", limits["memory"])
	})

	t.Run("the ClusterSpecAnnotation carries the null intact", func(t *testing.T) {
		raw, ok := result.Annotations[capr.ClusterSpecAnnotation]
		require.True(t, ok, "the control plane must carry %s", capr.ClusterSpecAnnotation)

		spec, err := snapshotutil.DecompressClusterSpec(raw)
		require.NoError(t, err)
		require.NotNil(t, spec.RKEConfig)

		limits := nestedMap(t, spec.RKEConfig.ChartValues.Data, "rke2-coredns", "resources", "limits")

		require.Contains(t, limits, "cpu", "the explicit null must still be present")
		assert.Nil(t, limits["cpu"])
		assert.Equal(t, "130Mi", limits["memory"])
	})

	t.Run("re-marshalling the annotated values emits the null", func(t *testing.T) {
		// addChartConfigs json.Marshals the per-chart values into
		// spec.valuesContent. Once the null reaches it, it renders correctly --
		// no change is needed there.
		spec, err := snapshotutil.DecompressClusterSpec(result.Annotations[capr.ClusterSpecAnnotation])
		require.NoError(t, err)

		values, err := json.Marshal(spec.RKEConfig.ChartValues.Data["rke2-coredns"])
		require.NoError(t, err)

		assert.JSONEq(t, `{"resources":{"limits":{"cpu":null,"memory":"130Mi"}}}`, string(values))
	})
}
