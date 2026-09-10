package snapshotextrametadata

import (
	"encoding/json"
	"errors"
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
)

const expectedKubernetesVersionSelector = "$['cluster.management.cattle.io'].spec.rke2Config.kubernetesVersion"

// testCluster returns a management cluster with both a populated spec and a populated status, so
// tests can tell the two apart after sanitizing.
func testCluster() *apimgmtv3.Cluster {
	return &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "c-m-12345678",
		},
		Spec: apimgmtv3.ClusterSpec{
			DisplayName: "downstream",
			Rke2Config: &apimgmtv3.Rke2Config{
				Version: "v1.32.5+rke2r1",
			},
		},
		Status: apimgmtv3.ClusterStatus{
			Driver:  apimgmtv3.ClusterDriverRke2,
			Version: &version.Info{GitVersion: "v1.32.5+rke2r1"},
			AppliedSpec: apimgmtv3.ClusterSpec{
				DisplayName: "downstream",
			},
		},
	}
}

func TestSanitize(t *testing.T) {
	t.Parallel()

	t.Run("purges the status subresource but keeps metadata and spec", func(t *testing.T) {
		t.Parallel()

		out, err := sanitize(testCluster())
		require.NoError(t, err)

		umap, ok := out.(map[string]any)
		require.True(t, ok, "sanitize should return an unstructured map, got %T", out)

		assert.NotContains(t, umap, "status")
		assert.Contains(t, umap, "metadata")
		assert.Contains(t, umap, "spec")
	})

	t.Run("keeps nested spec fields", func(t *testing.T) {
		t.Parallel()

		out, err := sanitize(testCluster())
		require.NoError(t, err)

		umap, ok := out.(map[string]any)
		require.True(t, ok)

		spec, ok := umap["spec"].(map[string]any)
		require.True(t, ok, "spec should be a map, got %T", umap["spec"])

		rke2Config, ok := spec["rke2Config"].(map[string]any)
		require.True(t, ok, "spec.rke2Config should be a map, got %T", spec["rke2Config"])

		assert.Equal(t, "v1.32.5+rke2r1", rke2Config["kubernetesVersion"])

		metadata, ok := umap["metadata"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "c-m-12345678", metadata["name"])
	})

	t.Run("keeps apiVersion and kind when set", func(t *testing.T) {
		t.Parallel()

		cluster := testCluster()
		cluster.TypeMeta = metav1.TypeMeta{
			APIVersion: "management.cattle.io/v3",
			Kind:       "Cluster",
		}

		out, err := sanitize(cluster)
		require.NoError(t, err)

		umap, ok := out.(map[string]any)
		require.True(t, ok)

		assert.Equal(t, "management.cattle.io/v3", umap["apiVersion"])
		assert.Equal(t, "Cluster", umap["kind"])
	})

	t.Run("purges every top-level field that is not rendered", func(t *testing.T) {
		t.Parallel()

		// A ConfigMap stands in for any object whose subresource-backed fields are not called
		// "status": the allowlist has to drop those too, not just status.
		out, err := sanitize(&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "cm"},
			Data:       map[string]string{"key": "value"},
			BinaryData: map[string][]byte{"bin": []byte("value")},
		})
		require.NoError(t, err)

		umap, ok := out.(map[string]any)
		require.True(t, ok)

		assert.NotContains(t, umap, "data")
		assert.NotContains(t, umap, "binaryData")
		assert.Contains(t, umap, "metadata")
	})

	t.Run("errors when the object cannot be converted", func(t *testing.T) {
		t.Parallel()

		// ToUnstructured requires a non-nil pointer.
		_, err := sanitize(*testCluster())
		assert.Error(t, err)
	})
}

// assertRenderedData asserts that data is a well-formed extra metadata payload for testCluster().
func assertRenderedData(t *testing.T, data map[string]string) {
	t.Helper()

	require.Contains(t, data, resourcesKey)
	require.Contains(t, data, restoreModesKey)

	resources := map[string]any{}
	require.NoError(t, json.Unmarshal([]byte(data[resourcesKey]), &resources))

	cluster, ok := resources[mgmtClusterResourceKey].(map[string]any)
	require.True(t, ok, "resources should contain a %s entry, got %v", mgmtClusterResourceKey, resources)

	assert.NotContains(t, cluster, "status", "the published cluster should have no subresources")

	spec, ok := cluster["spec"].(map[string]any)
	require.True(t, ok)
	rke2Config, ok := spec["rke2Config"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "v1.32.5+rke2r1", rke2Config["kubernetesVersion"],
		"the kubernetesVersion selector must resolve against the published resources")

	restoreModes := map[string]any{}
	require.NoError(t, json.Unmarshal([]byte(data[restoreModesKey]), &restoreModes))
	assert.Equal(t, map[string]any{
		"all":               "*",
		"kubernetesVersion": expectedKubernetesVersionSelector,
		"none":              "",
	}, restoreModes)
}

func TestRenderData(t *testing.T) {
	t.Parallel()

	data, err := renderData(testCluster())
	require.NoError(t, err)
	assertRenderedData(t, data)
}

func TestOnChange(t *testing.T) {
	t.Parallel()

	existingConfigMap := func() *corev1.ConfigMap {
		return &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: metav1.NamespaceSystem,
				Name:      configMapName,
			},
			Data: map[string]string{"provisioning-cluster-spec": "stale"},
		}
	}

	notFound := apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, configMapName)

	t.Run("updates an existing configmap", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		configMaps := fake.NewMockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)

		cached := existingConfigMap()
		configMaps.EXPECT().Get(metav1.NamespaceSystem, configMapName, gomock.Any()).Return(cached, nil)
		configMaps.EXPECT().Update(gomock.Any()).DoAndReturn(func(cm *corev1.ConfigMap) (*corev1.ConfigMap, error) {
			assert.Equal(t, metav1.NamespaceSystem, cm.Namespace)
			assert.Equal(t, configMapName, cm.Name)
			assertRenderedData(t, cm.Data)
			return cm, nil
		})

		h := &handler{configMap: configMaps}

		cluster := testCluster()
		out, err := h.onChange("", cluster)
		require.NoError(t, err)
		assert.Same(t, cluster, out)

		assert.Equal(t, existingConfigMap().Data, cached.Data, "the cached configmap must not be mutated")
	})

	t.Run("creates the configmap when it does not exist", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		configMaps := fake.NewMockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)

		configMaps.EXPECT().Get(metav1.NamespaceSystem, configMapName, gomock.Any()).Return(nil, notFound)
		configMaps.EXPECT().Create(gomock.Any()).DoAndReturn(func(cm *corev1.ConfigMap) (*corev1.ConfigMap, error) {
			assert.Equal(t, metav1.NamespaceSystem, cm.Namespace)
			assert.Equal(t, configMapName, cm.Name)
			assertRenderedData(t, cm.Data)
			return cm, nil
		})

		h := &handler{configMap: configMaps}

		cluster := testCluster()
		out, err := h.onChange("", cluster)
		require.NoError(t, err)
		assert.Same(t, cluster, out)
	})

	t.Run("does not write when the data is unchanged", func(t *testing.T) {
		t.Parallel()

		data, err := renderData(testCluster())
		require.NoError(t, err)

		ctrl := gomock.NewController(t)
		configMaps := fake.NewMockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)

		cm := existingConfigMap()
		cm.Data = data
		configMaps.EXPECT().Get(metav1.NamespaceSystem, configMapName, gomock.Any()).Return(cm, nil)

		h := &handler{configMap: configMaps}

		cluster := testCluster()
		out, err := h.onChange("", cluster)
		require.NoError(t, err)
		assert.Same(t, cluster, out)
	})

	t.Run("propagates a get error", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("boom")

		ctrl := gomock.NewController(t)
		configMaps := fake.NewMockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
		configMaps.EXPECT().Get(metav1.NamespaceSystem, configMapName, gomock.Any()).Return(nil, expectedErr)

		h := &handler{configMap: configMaps}

		out, err := h.onChange("", testCluster())
		assert.ErrorIs(t, err, expectedErr)
		assert.Nil(t, out)
	})

	t.Run("propagates a create error", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("boom")

		ctrl := gomock.NewController(t)
		configMaps := fake.NewMockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
		configMaps.EXPECT().Get(metav1.NamespaceSystem, configMapName, gomock.Any()).Return(nil, notFound)
		configMaps.EXPECT().Create(gomock.Any()).Return(nil, expectedErr)

		h := &handler{configMap: configMaps}

		out, err := h.onChange("", testCluster())
		assert.ErrorIs(t, err, expectedErr)
		assert.Nil(t, out)
	})

	t.Run("propagates an update error", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("boom")

		ctrl := gomock.NewController(t)
		configMaps := fake.NewMockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
		configMaps.EXPECT().Get(metav1.NamespaceSystem, configMapName, gomock.Any()).Return(existingConfigMap(), nil)
		configMaps.EXPECT().Update(gomock.Any()).Return(nil, expectedErr)

		h := &handler{configMap: configMaps}

		out, err := h.onChange("", testCluster())
		assert.ErrorIs(t, err, expectedErr)
		assert.Nil(t, out)
	})

	t.Run("ignores a nil cluster", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		// No expectations: any call to the client fails the test.
		configMaps := fake.NewMockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)

		h := &handler{configMap: configMaps}

		out, err := h.onChange("", nil)
		require.NoError(t, err)
		assert.Nil(t, out)
	})
}
