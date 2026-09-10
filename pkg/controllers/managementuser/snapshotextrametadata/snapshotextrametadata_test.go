package snapshotextrametadata

import (
	"encoding/json"
	"errors"
	"testing"

	controlplanev1beta2 "github.com/rancher/cluster-api-provider-rke2/controlplane/api/v1beta2"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1/snapshotutil"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	provcluster "github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

const (
	clusterName        = "c-m-12345678"
	kubernetesVersion  = "v1.32.5+rke2r1"
	capiClusterName    = "downstream"
	capiClusterNS      = "fleet-default"
	rke2ControlPlaneCP = "RKE2ControlPlane"
)

// dynamicClientFake is a stand-in for lasso's dynamic controller. Mirrors the fake in
// snapshotbackpopulate_test.go.
type dynamicClientFake struct {
	obj runtime.Object
	err error
}

func (d *dynamicClientFake) Get(_ schema.GroupVersionKind, _, _ string) (runtime.Object, error) {
	return d.obj, d.err
}

// mgmtCluster returns a management cluster with both a populated spec and a populated status, so
// tests can tell the two apart after sanitizing.
func mgmtCluster() *apimgmtv3.Cluster {
	return &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterName,
		},
		Spec: apimgmtv3.ClusterSpec{
			DisplayName: capiClusterName,
			Rke2Config: &apimgmtv3.Rke2Config{
				Version: kubernetesVersion,
			},
		},
		Status: apimgmtv3.ClusterStatus{
			Driver:  apimgmtv3.ClusterDriverRke2,
			Version: &version.Info{GitVersion: kubernetesVersion},
			AppliedSpec: apimgmtv3.ClusterSpec{
				DisplayName: capiClusterName,
			},
		},
	}
}

// provCluster returns a provisioning cluster with every operation field populated, so tests can
// assert they are stripped.
func provCluster() *provv1.Cluster {
	return &provv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: capiClusterNS,
			Name:      capiClusterName,
		},
		Spec: provv1.ClusterSpec{
			KubernetesVersion: kubernetesVersion,
			RKEConfig: &provv1.RKEConfig{
				ClusterConfiguration: rkev1.ClusterConfiguration{
					AdditionalManifest: "# manifest",
				},
				ETCDSnapshotRestore:  &rkev1.ETCDSnapshotRestore{Name: "snapshot-1"},
				ETCDSnapshotCreate:   &rkev1.ETCDSnapshotCreate{Generation: 3},
				RotateCertificates:   &rkev1.RotateCertificates{Generation: 4},
				RotateEncryptionKeys: &rkev1.RotateEncryptionKeys{Generation: 5},
			},
		},
		Status: provv1.ClusterStatus{
			ClusterName: clusterName,
		},
	}
}

func rke2ControlPlane() *controlplanev1beta2.RKE2ControlPlane {
	return &controlplanev1beta2.RKE2ControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: capiClusterNS,
			Name:      capiClusterName,
		},
		Spec: controlplanev1beta2.RKE2ControlPlaneSpec{
			Version: kubernetesVersion,
		},
	}
}

func rke2ControlPlaneUnstructured(t *testing.T) *unstructured.Unstructured {
	t.Helper()

	umap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(rke2ControlPlane())
	require.NoError(t, err)
	return &unstructured.Unstructured{Object: umap}
}

// capiCluster returns a CAPI cluster whose control plane ref points at cpGroup/cpKind.
func capiCluster(cpGroup, cpKind string) *capi.Cluster {
	return &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: capiClusterNS,
			Name:      capiClusterName,
		},
		Spec: capi.ClusterSpec{
			ControlPlaneRef: capi.ContractVersionedObjectReference{
				APIGroup: cpGroup,
				Kind:     cpKind,
				Name:     capiClusterName,
			},
		},
	}
}

// turtlesMgmtCluster returns a mgmt cluster shell carrying the turtles capi-cluster-owner labels.
func turtlesMgmtCluster() *apimgmtv3.Cluster {
	cluster := mgmtCluster()
	cluster.Labels = map[string]string{
		capr.CAPIClusterOwnerLabel:   capiClusterName,
		capr.CAPIClusterOwnerNSLabel: capiClusterNS,
	}
	return cluster
}

// administratedMgmtCluster returns a mgmt cluster shell carrying the v2prov administrated
// annotation.
func administratedMgmtCluster() *apimgmtv3.Cluster {
	cluster := mgmtCluster()
	cluster.Annotations = map[string]string{administratedAnnotation: "true"}
	return cluster
}

func TestSanitize(t *testing.T) {
	t.Parallel()

	t.Run("purges the status subresource but keeps metadata and spec", func(t *testing.T) {
		t.Parallel()

		umap, err := sanitize(mgmtClusterKey, mgmtCluster())
		require.NoError(t, err)

		assert.NotContains(t, umap, "status")
		assert.Contains(t, umap, "metadata")
		assert.Contains(t, umap, "spec")
	})

	t.Run("keeps nested spec fields", func(t *testing.T) {
		t.Parallel()

		umap, err := sanitize(mgmtClusterKey, mgmtCluster())
		require.NoError(t, err)

		rke2Config, found, err := unstructured.NestedMap(umap, "spec", "rke2Config")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, kubernetesVersion, rke2Config["kubernetesVersion"])

		name, found, err := unstructured.NestedString(umap, "metadata", "name")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, clusterName, name)
	})

	t.Run("keeps apiVersion and kind when set", func(t *testing.T) {
		t.Parallel()

		cluster := mgmtCluster()
		cluster.TypeMeta = metav1.TypeMeta{
			APIVersion: "management.cattle.io/v3",
			Kind:       "Cluster",
		}

		umap, err := sanitize(mgmtClusterKey, cluster)
		require.NoError(t, err)

		assert.Equal(t, "management.cattle.io/v3", umap["apiVersion"])
		assert.Equal(t, "Cluster", umap["kind"])
	})

	t.Run("purges every top-level field that is not rendered", func(t *testing.T) {
		t.Parallel()

		// A ConfigMap stands in for any object whose subresource-backed fields are not called
		// "status": the allowlist has to drop those too, not just status.
		umap, err := sanitize(mgmtClusterKey, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "cm"},
			Data:       map[string]string{"key": "value"},
			BinaryData: map[string][]byte{"bin": []byte("value")},
		})
		require.NoError(t, err)

		assert.NotContains(t, umap, "data")
		assert.NotContains(t, umap, "binaryData")
		assert.Contains(t, umap, "metadata")
	})

	t.Run("strips the registered paths for the resource key", func(t *testing.T) {
		t.Parallel()

		umap, err := sanitize(provClusterKey, provCluster())
		require.NoError(t, err)

		rkeConfig, found, err := unstructured.NestedMap(umap, "spec", "rkeConfig")
		require.NoError(t, err)
		require.True(t, found)

		for _, stripped := range []string{"etcdSnapshotRestore", "etcdSnapshotCreate", "rotateCertificates", "rotateEncryptionKeys"} {
			assert.NotContains(t, rkeConfig, stripped)
		}
		// Everything else in the config survives.
		assert.Equal(t, "# manifest", rkeConfig["additionalManifest"])
	})

	t.Run("leaves the object alone when the resource key has no registered paths", func(t *testing.T) {
		t.Parallel()

		// The same provisioning cluster under a key with no strip paths keeps its operation fields,
		// proving the stripping is driven by the registry and not by field name.
		umap, err := sanitize(mgmtClusterKey, provCluster())
		require.NoError(t, err)

		rkeConfig, found, err := unstructured.NestedMap(umap, "spec", "rkeConfig")
		require.NoError(t, err)
		require.True(t, found)
		assert.Contains(t, rkeConfig, "etcdSnapshotRestore")
	})

	t.Run("errors when the object cannot be converted", func(t *testing.T) {
		t.Parallel()

		// ToUnstructured requires a non-nil pointer.
		_, err := sanitize(mgmtClusterKey, *mgmtCluster())
		assert.Error(t, err)
	})
}

func TestSelector(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "$['cluster.management.cattle.io'].spec.rke2Config.kubernetesVersion",
		selector(mgmtClusterKey, "spec", "rke2Config", "kubernetesVersion"))
	assert.Equal(t, "$['cluster.provisioning.cattle.io'].spec.kubernetesVersion",
		selector(provClusterKey, "spec", "kubernetesVersion"))
	assert.Equal(t, "$['rke2controlplane.controlplane.cluster.x-k8s.io'].spec.version",
		selector(rke2ControlPlaneKey, "spec", "version"))
}

// newTestHandler builds a handler with all dependencies mocked. Mocks with no EXPECT calls fail the
// test if they are used.
func newTestHandler(t *testing.T, dyn dynamicClient) (*handler, *fake.MockCacheInterface[*provv1.Cluster], *fake.MockCacheInterface[*capi.Cluster], *fake.MockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList]) {
	t.Helper()

	ctrl := gomock.NewController(t)
	provClusters := fake.NewMockCacheInterface[*provv1.Cluster](ctrl)
	capiClusters := fake.NewMockCacheInterface[*capi.Cluster](ctrl)
	configMaps := fake.NewMockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)

	return &handler{
		clusterName:      clusterName,
		configMap:        configMaps,
		dynamic:          dyn,
		provClusterCache: provClusters,
		capiClusterCache: capiClusters,
	}, provClusters, capiClusters, configMaps
}

func TestNewAdapter(t *testing.T) {
	t.Parallel()

	t.Run("true imported cluster", func(t *testing.T) {
		t.Parallel()

		h, _, _, _ := newTestHandler(t, nil)

		cluster := mgmtCluster()
		a, err := h.newAdapter(cluster)
		require.NoError(t, err)

		imported, ok := a.(*importedAdapter)
		require.True(t, ok, "expected *importedAdapter, got %T", a)
		assert.Same(t, cluster, imported.cluster)

		res, err := a.resources()
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Equal(t, mgmtClusterKey, res[0].key)
		assert.Equal(t, "$['cluster.management.cattle.io'].spec.rke2Config.kubernetesVersion", a.kubernetesVersionSelector())
	})

	t.Run("v2prov administrated cluster", func(t *testing.T) {
		t.Parallel()

		h, provClusters, _, _ := newTestHandler(t, nil)

		prov := provCluster()
		provClusters.EXPECT().GetByIndex(provcluster.ByCluster, clusterName).Return([]*provv1.Cluster{prov}, nil)

		a, err := h.newAdapter(administratedMgmtCluster())
		require.NoError(t, err)

		provisioning, ok := a.(*provisioningAdapter)
		require.True(t, ok, "expected *provisioningAdapter, got %T", a)
		assert.Same(t, prov, provisioning.cluster)

		res, err := a.resources()
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Equal(t, provClusterKey, res[0].key)
		assert.Equal(t, "$['cluster.provisioning.cattle.io'].spec.kubernetesVersion", a.kubernetesVersionSelector())
	})

	t.Run("turtles imported CAPRKE2 cluster", func(t *testing.T) {
		t.Parallel()

		h, _, capiClusters, _ := newTestHandler(t, &dynamicClientFake{obj: rke2ControlPlaneUnstructured(t)})

		capiClusters.EXPECT().Get(capiClusterNS, capiClusterName).
			Return(capiCluster(controlplanev1beta2.GroupVersion.Group, rke2ControlPlaneCP), nil)

		a, err := h.newAdapter(turtlesMgmtCluster())
		require.NoError(t, err)

		caprke2, ok := a.(*caprke2Adapter)
		require.True(t, ok, "expected *caprke2Adapter, got %T", a)
		assert.Equal(t, kubernetesVersion, caprke2.controlPlane.Spec.Version)

		res, err := a.resources()
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Equal(t, rke2ControlPlaneKey, res[0].key)
		assert.Equal(t, "$['rke2controlplane.controlplane.cluster.x-k8s.io'].spec.version", a.kubernetesVersionSelector())
	})

	t.Run("turtles imported cluster with a non-CAPRKE2 control plane is unsupported", func(t *testing.T) {
		t.Parallel()

		h, _, capiClusters, _ := newTestHandler(t, nil)

		capiClusters.EXPECT().Get(capiClusterNS, capiClusterName).
			Return(capiCluster("controlplane.cluster.x-k8s.io", "KubeadmControlPlane"), nil)

		_, err := h.newAdapter(turtlesMgmtCluster())
		assert.ErrorIs(t, err, errUnsupportedClusterType)
	})

	t.Run("only one capi-cluster-owner label is a misconfiguration", func(t *testing.T) {
		t.Parallel()

		h, _, _, _ := newTestHandler(t, nil)

		cluster := mgmtCluster()
		cluster.Labels = map[string]string{capr.CAPIClusterOwnerLabel: capiClusterName}

		_, err := h.newAdapter(cluster)
		require.Error(t, err)
		assert.NotErrorIs(t, err, errUnsupportedClusterType)
	})

	t.Run("propagates a CAPI cluster lookup error", func(t *testing.T) {
		t.Parallel()

		h, _, capiClusters, _ := newTestHandler(t, nil)

		expectedErr := errors.New("boom")
		capiClusters.EXPECT().Get(capiClusterNS, capiClusterName).Return(nil, expectedErr)

		_, err := h.newAdapter(turtlesMgmtCluster())
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("propagates an RKE2ControlPlane lookup error", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("boom")
		h, _, capiClusters, _ := newTestHandler(t, &dynamicClientFake{err: expectedErr})

		capiClusters.EXPECT().Get(capiClusterNS, capiClusterName).
			Return(capiCluster(controlplanev1beta2.GroupVersion.Group, rke2ControlPlaneCP), nil)

		_, err := h.newAdapter(turtlesMgmtCluster())
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("errors when the administrated cluster has no provisioning cluster", func(t *testing.T) {
		t.Parallel()

		h, provClusters, _, _ := newTestHandler(t, nil)

		provClusters.EXPECT().GetByIndex(provcluster.ByCluster, clusterName).Return(nil, nil)

		_, err := h.newAdapter(administratedMgmtCluster())
		assert.ErrorContains(t, err, "expected exactly 1 provisioning cluster")
	})

	t.Run("errors when the administrated cluster has more than one provisioning cluster", func(t *testing.T) {
		t.Parallel()

		h, provClusters, _, _ := newTestHandler(t, nil)

		provClusters.EXPECT().GetByIndex(provcluster.ByCluster, clusterName).
			Return([]*provv1.Cluster{provCluster(), provCluster()}, nil)

		_, err := h.newAdapter(administratedMgmtCluster())
		assert.ErrorContains(t, err, "expected exactly 1 provisioning cluster")
	})
}

// decodeResources decompresses the resources section of a rendered ConfigMap payload.
func decodeResources(t *testing.T, data map[string]string) map[string]any {
	t.Helper()

	require.Contains(t, data, resourcesKey)

	resources := map[string]any{}
	require.NoError(t, snapshotutil.DecompressInterface(data[resourcesKey], &resources))
	return resources
}

// decodeRestoreModes unmarshals the restoreModes section of a rendered ConfigMap payload.
func decodeRestoreModes(t *testing.T, data map[string]string) map[string]any {
	t.Helper()

	require.Contains(t, data, restoreModesKey)

	restoreModes := map[string]any{}
	require.NoError(t, json.Unmarshal([]byte(data[restoreModesKey]), &restoreModes))
	return restoreModes
}

func TestRenderData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		adapter          adapter
		expectedKey      string
		expectedSelector string
	}{
		{
			name:             "imported",
			adapter:          &importedAdapter{cluster: mgmtCluster()},
			expectedKey:      mgmtClusterKey,
			expectedSelector: "$['cluster.management.cattle.io'].spec.rke2Config.kubernetesVersion",
		},
		{
			name:             "v2prov",
			adapter:          &provisioningAdapter{cluster: provCluster()},
			expectedKey:      provClusterKey,
			expectedSelector: "$['cluster.provisioning.cattle.io'].spec.kubernetesVersion",
		},
		{
			name:             "caprke2",
			adapter:          &caprke2Adapter{controlPlane: rke2ControlPlane()},
			expectedKey:      rke2ControlPlaneKey,
			expectedSelector: "$['rke2controlplane.controlplane.cluster.x-k8s.io'].spec.version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data, err := renderData(tt.adapter)
			require.NoError(t, err)

			resources := decodeResources(t, data)
			require.Len(t, resources, 1)

			obj, ok := resources[tt.expectedKey].(map[string]any)
			require.True(t, ok, "resources should contain a %s entry, got %v", tt.expectedKey, resources)
			assert.NotContains(t, obj, "status", "the published object should have no subresources")

			assert.Equal(t, map[string]any{
				"all":               "*",
				"kubernetesVersion": tt.expectedSelector,
				"none":              "",
			}, decodeRestoreModes(t, data))
		})
	}

	t.Run("the kubernetesVersion selector resolves against the published resources", func(t *testing.T) {
		t.Parallel()

		// The selectors are strings, so nothing in the controller proves they point at a real
		// field. Walk each one by hand against the payload it was rendered with.
		for _, tt := range tests {
			data, err := renderData(tt.adapter)
			require.NoError(t, err)

			resources := decodeResources(t, data)
			obj, ok := resources[tt.expectedKey].(map[string]any)
			require.True(t, ok)

			var fields []string
			switch tt.expectedKey {
			case mgmtClusterKey:
				fields = []string{"spec", "rke2Config", "kubernetesVersion"}
			case provClusterKey:
				fields = []string{"spec", "kubernetesVersion"}
			case rke2ControlPlaneKey:
				fields = []string{"spec", "version"}
			}
			require.Equal(t, selector(tt.expectedKey, fields...), tt.expectedSelector)

			got, found, err := unstructured.NestedString(obj, fields...)
			require.NoError(t, err)
			require.True(t, found, "%s: %s does not resolve", tt.name, tt.expectedSelector)
			assert.Equal(t, kubernetesVersion, got)
		}
	})

	t.Run("strips the provisioning cluster operation fields", func(t *testing.T) {
		t.Parallel()

		data, err := renderData(&provisioningAdapter{cluster: provCluster()})
		require.NoError(t, err)

		resources := decodeResources(t, data)
		obj, ok := resources[provClusterKey].(map[string]any)
		require.True(t, ok)

		rkeConfig, found, err := unstructured.NestedMap(obj, "spec", "rkeConfig")
		require.NoError(t, err)
		require.True(t, found)

		for _, stripped := range []string{"etcdSnapshotRestore", "etcdSnapshotCreate", "rotateCertificates", "rotateEncryptionKeys"} {
			assert.NotContains(t, rkeConfig, stripped)
		}
	})
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

		h, _, _, configMaps := newTestHandler(t, nil)

		cached := existingConfigMap()
		configMaps.EXPECT().Get(metav1.NamespaceSystem, configMapName, gomock.Any()).Return(cached, nil)
		configMaps.EXPECT().Update(gomock.Any()).DoAndReturn(func(cm *corev1.ConfigMap) (*corev1.ConfigMap, error) {
			assert.Equal(t, metav1.NamespaceSystem, cm.Namespace)
			assert.Equal(t, configMapName, cm.Name)
			assert.Contains(t, decodeResources(t, cm.Data), mgmtClusterKey)
			assert.Contains(t, decodeRestoreModes(t, cm.Data), "kubernetesVersion")
			return cm, nil
		})

		cluster := mgmtCluster()
		out, err := h.onChange("", cluster)
		require.NoError(t, err)
		assert.Same(t, cluster, out)

		assert.Equal(t, existingConfigMap().Data, cached.Data, "the cached configmap must not be mutated")
	})

	t.Run("creates the configmap when it does not exist", func(t *testing.T) {
		t.Parallel()

		h, _, _, configMaps := newTestHandler(t, nil)

		configMaps.EXPECT().Get(metav1.NamespaceSystem, configMapName, gomock.Any()).Return(nil, notFound)
		configMaps.EXPECT().Create(gomock.Any()).DoAndReturn(func(cm *corev1.ConfigMap) (*corev1.ConfigMap, error) {
			assert.Equal(t, metav1.NamespaceSystem, cm.Namespace)
			assert.Equal(t, configMapName, cm.Name)
			assert.Contains(t, decodeResources(t, cm.Data), mgmtClusterKey)
			return cm, nil
		})

		cluster := mgmtCluster()
		out, err := h.onChange("", cluster)
		require.NoError(t, err)
		assert.Same(t, cluster, out)
	})

	t.Run("publishes the provisioning cluster for a v2prov cluster", func(t *testing.T) {
		t.Parallel()

		h, provClusters, _, configMaps := newTestHandler(t, nil)

		provClusters.EXPECT().GetByIndex(provcluster.ByCluster, clusterName).Return([]*provv1.Cluster{provCluster()}, nil)
		configMaps.EXPECT().Get(metav1.NamespaceSystem, configMapName, gomock.Any()).Return(existingConfigMap(), nil)
		configMaps.EXPECT().Update(gomock.Any()).DoAndReturn(func(cm *corev1.ConfigMap) (*corev1.ConfigMap, error) {
			assert.Contains(t, decodeResources(t, cm.Data), provClusterKey)
			assert.Equal(t, "$['cluster.provisioning.cattle.io'].spec.kubernetesVersion",
				decodeRestoreModes(t, cm.Data)["kubernetesVersion"])
			return cm, nil
		})

		_, err := h.onChange("", administratedMgmtCluster())
		require.NoError(t, err)
	})

	t.Run("publishes the RKE2ControlPlane for a turtles imported cluster", func(t *testing.T) {
		t.Parallel()

		h, _, capiClusters, configMaps := newTestHandler(t, &dynamicClientFake{obj: rke2ControlPlaneUnstructured(t)})

		capiClusters.EXPECT().Get(capiClusterNS, capiClusterName).
			Return(capiCluster(controlplanev1beta2.GroupVersion.Group, rke2ControlPlaneCP), nil)
		configMaps.EXPECT().Get(metav1.NamespaceSystem, configMapName, gomock.Any()).Return(existingConfigMap(), nil)
		configMaps.EXPECT().Update(gomock.Any()).DoAndReturn(func(cm *corev1.ConfigMap) (*corev1.ConfigMap, error) {
			assert.Contains(t, decodeResources(t, cm.Data), rke2ControlPlaneKey)
			assert.Equal(t, "$['rke2controlplane.controlplane.cluster.x-k8s.io'].spec.version",
				decodeRestoreModes(t, cm.Data)["kubernetesVersion"])
			return cm, nil
		})

		_, err := h.onChange("", turtlesMgmtCluster())
		require.NoError(t, err)
	})

	t.Run("does not write for an unsupported cluster type", func(t *testing.T) {
		t.Parallel()

		h, _, capiClusters, _ := newTestHandler(t, nil)

		capiClusters.EXPECT().Get(capiClusterNS, capiClusterName).
			Return(capiCluster("controlplane.cluster.x-k8s.io", "KubeadmControlPlane"), nil)

		cluster := turtlesMgmtCluster()
		out, err := h.onChange("", cluster)
		require.NoError(t, err)
		assert.Same(t, cluster, out)
	})

	t.Run("does not write when the data is unchanged", func(t *testing.T) {
		t.Parallel()

		data, err := renderData(&importedAdapter{cluster: mgmtCluster()})
		require.NoError(t, err)

		h, _, _, configMaps := newTestHandler(t, nil)

		cm := existingConfigMap()
		cm.Data = data
		configMaps.EXPECT().Get(metav1.NamespaceSystem, configMapName, gomock.Any()).Return(cm, nil)

		cluster := mgmtCluster()
		out, err := h.onChange("", cluster)
		require.NoError(t, err)
		assert.Same(t, cluster, out)
	})

	t.Run("ignores a cluster this agent is not responsible for", func(t *testing.T) {
		t.Parallel()

		h, _, _, _ := newTestHandler(t, nil)

		other := mgmtCluster()
		other.Name = "c-m-87654321"

		out, err := h.onChange("", other)
		require.NoError(t, err)
		assert.Same(t, other, out)
	})

	t.Run("ignores a nil cluster", func(t *testing.T) {
		t.Parallel()

		h, _, _, _ := newTestHandler(t, nil)

		out, err := h.onChange("", nil)
		require.NoError(t, err)
		assert.Nil(t, out)
	})

	t.Run("propagates a get error", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("boom")

		h, _, _, configMaps := newTestHandler(t, nil)
		configMaps.EXPECT().Get(metav1.NamespaceSystem, configMapName, gomock.Any()).Return(nil, expectedErr)

		out, err := h.onChange("", mgmtCluster())
		assert.ErrorIs(t, err, expectedErr)
		assert.Nil(t, out)
	})

	t.Run("propagates a create error", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("boom")

		h, _, _, configMaps := newTestHandler(t, nil)
		configMaps.EXPECT().Get(metav1.NamespaceSystem, configMapName, gomock.Any()).Return(nil, notFound)
		configMaps.EXPECT().Create(gomock.Any()).Return(nil, expectedErr)

		out, err := h.onChange("", mgmtCluster())
		assert.ErrorIs(t, err, expectedErr)
		assert.Nil(t, out)
	})

	t.Run("propagates an update error", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("boom")

		h, _, _, configMaps := newTestHandler(t, nil)
		configMaps.EXPECT().Get(metav1.NamespaceSystem, configMapName, gomock.Any()).Return(existingConfigMap(), nil)
		configMaps.EXPECT().Update(gomock.Any()).Return(nil, expectedErr)

		out, err := h.onChange("", mgmtCluster())
		assert.ErrorIs(t, err, expectedErr)
		assert.Nil(t, out)
	})

	t.Run("propagates an adapter resolution error", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("boom")

		h, provClusters, _, _ := newTestHandler(t, nil)
		provClusters.EXPECT().GetByIndex(provcluster.ByCluster, clusterName).Return(nil, expectedErr)

		out, err := h.onChange("", administratedMgmtCluster())
		assert.ErrorIs(t, err, expectedErr)
		assert.Nil(t, out)
	})
}
