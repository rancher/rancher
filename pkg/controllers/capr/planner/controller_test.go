package planner

import (
	"errors"
	"testing"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	rkecontrollers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func TestMachineSelectorFileSourceIndexer(t *testing.T) {
	tests := []struct {
		name         string
		controlPlane *rkev1.RKEControlPlane
		want         []string
	}{
		{
			name: "indexes secret and configmap sources across entries",
			controlPlane: &rkev1.RKEControlPlane{
				ObjectMeta: metav1.ObjectMeta{Namespace: "fleet-default"},
				Spec: rkev1.RKEControlPlaneSpec{ClusterConfiguration: rkev1.ClusterConfiguration{
					MachineSelectorFiles: []rkev1.RKEProvisioningFiles{
						{FileSources: []rkev1.ProvisioningFileSource{
							{Secret: rkev1.K8sObjectFileSource{Name: "credentials"}, ConfigMap: rkev1.K8sObjectFileSource{Name: "settings"}},
							{}, // Empty source names are skipped.
						}},
						{FileSources: []rkev1.ProvisioningFileSource{
							{Secret: rkev1.K8sObjectFileSource{Name: "other"}},
						}},
					},
				}},
			},
			want: []string{
				"secret/fleet-default/credentials",
				"configmap/fleet-default/settings",
				"secret/fleet-default/other",
			},
		},
		{
			name: "no machine selector files",
			controlPlane: &rkev1.RKEControlPlane{
				ObjectMeta: metav1.ObjectMeta{Namespace: "fleet-default"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := machineSelectorFileSourceIndexer(tt.controlPlane)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

type testControlPlaneCache struct {
	rkecontrollers.RKEControlPlaneCache
	indexName string
	key       string
	results   []*rkev1.RKEControlPlane
	err       error
}

func (c *testControlPlaneCache) GetByIndex(indexName, key string) ([]*rkev1.RKEControlPlane, error) {
	c.indexName, c.key = indexName, key
	return c.results, c.err
}

func TestResolvePlannerKeys(t *testing.T) {
	referencingControlPlanes := []*rkev1.RKEControlPlane{
		{ObjectMeta: metav1.ObjectMeta{Namespace: "fleet-default", Name: "referencing-a"}},
		{ObjectMeta: metav1.ObjectMeta{Namespace: "fleet-default", Name: "referencing-b"}},
	}

	t.Run("secret includes cluster label, name list, and references", func(t *testing.T) {
		cache := &testControlPlaneCache{results: referencingControlPlanes}
		h := &handler{controlPlaneCache: cache}
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Namespace: "fleet-default",
			Name:      "shared",
			Labels:    map[string]string{capr.ClusterNameLabel: "label-cluster"},
			Annotations: map[string]string{
				capr.AuthorizedObjectAnnotation: "named-a,named-b",
			},
		}}

		got, err := h.resolvePlannerKeys(secret.Namespace, secret.Name, secret)
		assert.NoError(t, err)
		assert.Equal(t, []relatedresource.Key{
			{Namespace: "fleet-default", Name: "label-cluster"},
			{Namespace: "fleet-default", Name: "named-a"},
			{Namespace: "fleet-default", Name: "named-b"},
			{Namespace: "fleet-default", Name: "referencing-a"},
			{Namespace: "fleet-default", Name: "referencing-b"},
		}, got)
		assert.Equal(t, byMachineSelectorFileSource, cache.indexName)
		assert.Equal(t, "secret/fleet-default/shared", cache.key)
	})

	t.Run("configmap references requeue with or without selector annotation", func(t *testing.T) {
		for _, tt := range []struct {
			name        string
			annotations map[string]string
		}{
			{name: "annotation absent"},
			{
				name: "selector annotation present",
				annotations: map[string]string{
					capr.AuthorizedObjectSelectorAnnotation: "env=dev",
				},
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				cache := &testControlPlaneCache{results: referencingControlPlanes}
				h := &handler{controlPlaneCache: cache}
				configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
					Namespace:   "fleet-default",
					Name:        "shared",
					Annotations: tt.annotations,
				}}

				got, err := h.resolvePlannerKeys(configMap.Namespace, configMap.Name, configMap)
				assert.NoError(t, err)
				assert.Equal(t, []relatedresource.Key{
					{Namespace: "fleet-default", Name: "referencing-a"},
					{Namespace: "fleet-default", Name: "referencing-b"},
				}, got)
				assert.Equal(t, byMachineSelectorFileSource, cache.indexName)
				assert.Equal(t, "configmap/fleet-default/shared", cache.key)
			})
		}
	})

	t.Run("configmap name list is resolved without references", func(t *testing.T) {
		cache := &testControlPlaneCache{}
		h := &handler{controlPlaneCache: cache}
		configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Namespace: "fleet-default",
			Name:      "shared",
			Annotations: map[string]string{
				capr.AuthorizedObjectAnnotation: "named-a,named-b",
			},
		}}

		got, err := h.resolvePlannerKeys(configMap.Namespace, configMap.Name, configMap)
		assert.NoError(t, err)
		assert.Equal(t, []relatedresource.Key{
			{Namespace: "fleet-default", Name: "named-a"},
			{Namespace: "fleet-default", Name: "named-b"},
		}, got)
		assert.Equal(t, "configmap/fleet-default/shared", cache.key)
	})
}

func TestResolvePlannerKeysOtherEvents(t *testing.T) {
	t.Run("machine maps to its cluster", func(t *testing.T) {
		cache := &testControlPlaneCache{}
		h := &handler{controlPlaneCache: cache}
		machine := &capi.Machine{ObjectMeta: metav1.ObjectMeta{
			Namespace: "fleet-default",
			Name:      "machine",
			Labels:    map[string]string{capi.ClusterNameLabel: "cluster"},
		}}

		got, err := h.resolvePlannerKeys(machine.Namespace, machine.Name, machine)
		assert.NoError(t, err)
		assert.Equal(t, []relatedresource.Key{{Namespace: "fleet-default", Name: "cluster"}}, got)
		assert.Empty(t, cache.indexName)
	})

	t.Run("nil object has no keys", func(t *testing.T) {
		cache := &testControlPlaneCache{}
		h := &handler{controlPlaneCache: cache}

		got, err := h.resolvePlannerKeys("", "", nil)
		assert.NoError(t, err)
		assert.Empty(t, got)
		assert.Empty(t, cache.indexName)
	})

	t.Run("index error is returned", func(t *testing.T) {
		cache := &testControlPlaneCache{err: errors.New("index unavailable")}
		h := &handler{controlPlaneCache: cache}
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "fleet-default", Name: "shared"}}

		_, err := h.resolvePlannerKeys(secret.Namespace, secret.Name, secret)
		assert.ErrorIs(t, err, cache.err)
	})
}
