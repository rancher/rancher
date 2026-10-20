package rancher

import (
	"fmt"
	"testing"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/tokens"
	exttokenstore "github.com/rancher/rancher/pkg/ext/stores/tokens"
	rancherversion "github.com/rancher/rancher/pkg/version"
	controllerv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func TestLegacyClusterAutoscalerSettingValues(t *testing.T) {
	tests := map[string]struct {
		value      string
		chartMatch bool
		imageMatch bool
	}{
		"registry host": {
			value:      "registry.example.com/rancher/appco-kubernetes-cluster-autoscaler",
			imageMatch: true,
		},
		"registry port": {
			value:      "registry.example.com:5000/rancher/appco-kubernetes-cluster-autoscaler",
			imageMatch: true,
		},
		"chart repository": {
			value:      "oci://registry.example.com/rancher/charts/appco-kubernetes-cluster-autoscaler",
			chartMatch: true,
		},
		"custom repository": {
			value: "oci://registry.example.com/custom/cluster-autoscaler",
		},
		"tagged image": {
			value: "registry.example.com/rancher/appco-kubernetes-cluster-autoscaler:v1",
		},
		"extra path": {
			value: "registry.example.com/rancher/appco-kubernetes-cluster-autoscaler/extra",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if got := legacyClusterAutoscalerChartRepositoryRegexp.MatchString(test.value); got != test.chartMatch {
				t.Fatalf("chart match = %v, want %v", got, test.chartMatch)
			}
			if got := legacyClusterAutoscalerImageRegexp.MatchString(test.value); got != test.imageMatch {
				t.Fatalf("image match = %v, want %v", got, test.imageMatch)
			}
		})
	}
}

func TestMigrateLegacyClusterAutoscalerSettings(t *testing.T) {
	originalVersion := rancherversion.Version
	rancherversion.Version = "v1.0.0"
	t.Cleanup(func() {
		rancherversion.Version = originalVersion
	})

	newConfigMapController := func(ctrl *gomock.Controller) (*fake.MockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList], *fake.MockCacheInterface[*corev1.ConfigMap]) {
		configMaps := fake.NewMockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
		cache := fake.NewMockCacheInterface[*corev1.ConfigMap](ctrl)
		configMaps.EXPECT().Cache().Return(cache)
		return configMaps, cache
	}

	t.Run("skips a completed migration", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		configMaps, cache := newConfigMapController(ctrl)
		settings := fake.NewMockNonNamespacedControllerInterface[*apiv3.Setting, *apiv3.SettingList](ctrl)
		cache.EXPECT().Get(cattleNamespace, migrateLegacyClusterAutoscalerSettingsConfig).Return(&corev1.ConfigMap{
			Data: map[string]string{legacyClusterAutoscalerSettingsMigratedKey: "true"},
		}, nil)

		require.NoError(t, migrateLegacyClusterAutoscalerSettings(configMaps, settings))
	})

	t.Run("clears legacy settings and records completion", func(t *testing.T) {
		t.Setenv("CATTLE_BASE_CLUSTER_AUTOSCALER_CHART_REPOSITORY", "")
		t.Setenv("CATTLE_BASE_CLUSTER_AUTOSCALER_IMAGE", "")

		ctrl := gomock.NewController(t)
		configMaps, cache := newConfigMapController(ctrl)
		settings := fake.NewMockNonNamespacedControllerInterface[*apiv3.Setting, *apiv3.SettingList](ctrl)
		cache.EXPECT().Get(cattleNamespace, migrateLegacyClusterAutoscalerSettingsConfig).Return(nil, nil)

		chart := &apiv3.Setting{
			ObjectMeta: metav1.ObjectMeta{Name: clusterAutoscalerChartRepository},
			Value:      "oci://registry.example.com/rancher/charts/appco-kubernetes-cluster-autoscaler",
			Source:     "env",
		}
		image := &apiv3.Setting{
			ObjectMeta: metav1.ObjectMeta{Name: clusterAutoscalerImage},
			Value:      "registry.example.com/rancher/appco-kubernetes-cluster-autoscaler",
		}
		settings.EXPECT().Get(clusterAutoscalerChartRepository, metav1.GetOptions{}).Return(chart, nil)
		settings.EXPECT().Update(&apiv3.Setting{
			ObjectMeta: metav1.ObjectMeta{Name: clusterAutoscalerChartRepository},
		})
		settings.EXPECT().Get(clusterAutoscalerImage, metav1.GetOptions{}).Return(image, nil)
		settings.EXPECT().Update(&apiv3.Setting{
			ObjectMeta: metav1.ObjectMeta{Name: clusterAutoscalerImage},
		})
		configMaps.EXPECT().Create(&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      migrateLegacyClusterAutoscalerSettingsConfig,
				Namespace: cattleNamespace,
			},
			Data: map[string]string{legacyClusterAutoscalerSettingsMigratedKey: "true"},
		})

		require.NoError(t, migrateLegacyClusterAutoscalerSettings(configMaps, settings))
	})

	t.Run("retains custom and environment-configured settings", func(t *testing.T) {
		t.Setenv("CATTLE_BASE_CLUSTER_AUTOSCALER_CHART_REPOSITORY", "oci://configured.example.com/charts")
		t.Setenv("CATTLE_BASE_CLUSTER_AUTOSCALER_IMAGE", "")

		ctrl := gomock.NewController(t)
		configMaps, cache := newConfigMapController(ctrl)
		settings := fake.NewMockNonNamespacedControllerInterface[*apiv3.Setting, *apiv3.SettingList](ctrl)
		cache.EXPECT().Get(cattleNamespace, migrateLegacyClusterAutoscalerSettingsConfig).Return(nil, nil)
		settings.EXPECT().Get(clusterAutoscalerImage, metav1.GetOptions{}).Return(&apiv3.Setting{
			ObjectMeta: metav1.ObjectMeta{Name: clusterAutoscalerImage},
			Value:      "registry.example.com/custom/autoscaler",
		}, nil)
		configMaps.EXPECT().Create(gomock.Any())

		require.NoError(t, migrateLegacyClusterAutoscalerSettings(configMaps, settings))
	})

	t.Run("does not record completion when updating a setting fails", func(t *testing.T) {
		t.Setenv("CATTLE_BASE_CLUSTER_AUTOSCALER_CHART_REPOSITORY", "")
		t.Setenv("CATTLE_BASE_CLUSTER_AUTOSCALER_IMAGE", "")

		ctrl := gomock.NewController(t)
		configMaps, cache := newConfigMapController(ctrl)
		settings := fake.NewMockNonNamespacedControllerInterface[*apiv3.Setting, *apiv3.SettingList](ctrl)
		cache.EXPECT().Get(cattleNamespace, migrateLegacyClusterAutoscalerSettingsConfig).Return(nil, nil)
		settings.EXPECT().Get(clusterAutoscalerChartRepository, metav1.GetOptions{}).Return(&apiv3.Setting{
			ObjectMeta: metav1.ObjectMeta{Name: clusterAutoscalerChartRepository},
			Value:      "oci://registry.example.com/rancher/charts/appco-kubernetes-cluster-autoscaler",
		}, nil).AnyTimes()
		settings.EXPECT().Update(gomock.Any()).Return(nil, fmt.Errorf("update failed")).AnyTimes()

		require.Error(t, migrateLegacyClusterAutoscalerSettings(configMaps, settings))
	})
}

func TestForceUpgradeLogout(t *testing.T) {

	const (
		lastMigrationBeforeTarget string = "v2.0.0"
		migrationTarget           string = "v3.0.0"
		lastMigrationBeyondTarget string = "v4.0.0"
	)
	rancherversion.Version = migrationTarget

	commonMigrationSetup := func(
		ctrl *gomock.Controller,
		configmaps *fake.MockCacheInterface[*corev1.ConfigMap],
		v3tokens *fake.MockNonNamespacedControllerInterface[*apiv3.Token, *apiv3.TokenList],
		extDeleter *extDeletionStub,
	) {
		// legacy tokens
		tCache := fake.NewMockNonNamespacedCacheInterface[*apiv3.Token](ctrl)
		v3tokens.EXPECT().Cache().
			Return(tCache)
		tCache.EXPECT().List(labels.SelectorFromSet(labels.Set{tokens.TokenKindLabel: "session"})).
			Return([]*apiv3.Token{
				&apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "v3session"}},
			}, nil)
		v3tokens.EXPECT().Delete("v3session", gomock.Any()).
			Return(nil)
	}

	tests := map[string]struct {
		err   error
		setup func(
			ctrl *gomock.Controller,
			configmaps *fake.MockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList],
			cmCache *fake.MockCacheInterface[*corev1.ConfigMap],
			tokens *fake.MockNonNamespacedControllerInterface[*apiv3.Token, *apiv3.TokenList],
			extDeleter *extDeletionStub,
		)
		extCount int
	}{
		"config map retrieval error": {
			err: fmt.Errorf("some transient issue"),
			setup: func(
				ctrl *gomock.Controller,
				configmaps *fake.MockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList],
				cmCache *fake.MockCacheInterface[*corev1.ConfigMap],
				tokens *fake.MockNonNamespacedControllerInterface[*apiv3.Token, *apiv3.TokenList],
				extDeleter *extDeletionStub,
			) {
				cmCache.EXPECT().Get("cattle-system", "forceupgradelogout").
					Return(nil, fmt.Errorf("some transient issue"))
			},
		},
		"no migration for last migration >= target version": {
			setup: func(
				ctrl *gomock.Controller,
				configmaps *fake.MockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList],
				cmCache *fake.MockCacheInterface[*corev1.ConfigMap],
				tokens *fake.MockNonNamespacedControllerInterface[*apiv3.Token, *apiv3.TokenList],
				extDeleter *extDeletionStub,
			) {
				cmCache.EXPECT().Get(cattleNamespace, forceUpgradeLogoutConfig).
					Return(&corev1.ConfigMap{
						Data: map[string]string{
							rancherVersionKey: lastMigrationBeyondTarget,
						},
					}, nil)
			},
		},
		"no migration for last migration == target version": {
			setup: func(
				ctrl *gomock.Controller,
				configmaps *fake.MockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList],
				cmCache *fake.MockCacheInterface[*corev1.ConfigMap],
				tokens *fake.MockNonNamespacedControllerInterface[*apiv3.Token, *apiv3.TokenList],
				extDeleter *extDeletionStub,
			) {
				cmCache.EXPECT().Get(cattleNamespace, forceUpgradeLogoutConfig).
					Return(&corev1.ConfigMap{
						Data: map[string]string{
							rancherVersionKey: migrationTarget,
						},
					}, nil)
			},
		},
		"migration triggered for last migration <= target version": {
			extCount: 1,
			setup: func(
				ctrl *gomock.Controller,
				configmaps *fake.MockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList],
				cmCache *fake.MockCacheInterface[*corev1.ConfigMap],
				tokens *fake.MockNonNamespacedControllerInterface[*apiv3.Token, *apiv3.TokenList],
				extDeleter *extDeletionStub,
			) {
				cmCache.EXPECT().Get(cattleNamespace, forceUpgradeLogoutConfig).
					Return(&corev1.ConfigMap{
						Data: map[string]string{
							rancherVersionKey: lastMigrationBeforeTarget,
						},
					}, nil)

				commonMigrationSetup(ctrl, cmCache, tokens, extDeleter)

				configmaps.EXPECT().Create(&corev1.ConfigMap{
					Data: map[string]string{
						rancherVersionKey: migrationTarget,
					},
				})
			},
		},
		"migration triggered when config map not present": {
			extCount: 1,
			setup: func(
				ctrl *gomock.Controller,
				configmaps *fake.MockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList],
				cmCache *fake.MockCacheInterface[*corev1.ConfigMap],
				tokens *fake.MockNonNamespacedControllerInterface[*apiv3.Token, *apiv3.TokenList],
				extDeleter *extDeletionStub,
			) {
				cmCache.EXPECT().Get(cattleNamespace, forceUpgradeLogoutConfig).Return(nil, nil)

				commonMigrationSetup(ctrl, cmCache, tokens, extDeleter)

				configmaps.EXPECT().Create(&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      forceUpgradeLogoutConfig,
						Namespace: cattleNamespace,
					},
					Data: map[string]string{
						rancherVersionKey: migrationTarget,
					},
				})
			},
		},
		"migration triggered when no last migration available": {
			extCount: 1,
			setup: func(
				ctrl *gomock.Controller,
				configmaps *fake.MockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList],
				cmCache *fake.MockCacheInterface[*corev1.ConfigMap],
				tokens *fake.MockNonNamespacedControllerInterface[*apiv3.Token, *apiv3.TokenList],
				extDeleter *extDeletionStub,
			) {
				cmCache.EXPECT().Get(cattleNamespace, forceUpgradeLogoutConfig).
					Return(&corev1.ConfigMap{Data: map[string]string{}}, nil)

				commonMigrationSetup(ctrl, cmCache, tokens, extDeleter)

				configmaps.EXPECT().Create(&corev1.ConfigMap{
					Data: map[string]string{
						rancherVersionKey: migrationTarget,
					},
				})
			},
		},
		"migration ignores ext bulk deletion errors": {
			extCount: 1,
			setup: func(
				ctrl *gomock.Controller,
				configmaps *fake.MockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList],
				cmCache *fake.MockCacheInterface[*corev1.ConfigMap],
				tokens *fake.MockNonNamespacedControllerInterface[*apiv3.Token, *apiv3.TokenList],
				extDeleter *extDeletionStub,
			) {
				cmCache.EXPECT().Get(cattleNamespace, forceUpgradeLogoutConfig).
					Return(&corev1.ConfigMap{Data: map[string]string{}}, nil)

				commonMigrationSetup(ctrl, cmCache, tokens, extDeleter)
				extDeleter.err = fmt.Errorf("some error")

				configmaps.EXPECT().Create(&corev1.ConfigMap{
					Data: map[string]string{
						rancherVersionKey: migrationTarget,
					},
				})
			},
		},
	}

	for name, spec := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			configmaps := fake.NewMockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
			cmCache := fake.NewMockCacheInterface[*corev1.ConfigMap](ctrl)
			configmaps.EXPECT().Cache().Return(cmCache)

			tokens := fake.NewMockNonNamespacedControllerInterface[*apiv3.Token, *apiv3.TokenList](ctrl)

			eDeleter := &extDeletionStub{t: t}

			if spec.setup != nil {
				spec.setup(ctrl, configmaps, cmCache, tokens, eDeleter)
			}

			err := forceUpgradeLogout(configmaps, tokens, eDeleter, migrationTarget)

			require.Equal(t, eDeleter.count, spec.extCount)
			if spec.err != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), spec.err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// extDeletionStub implements [tokenCollectionDeleter]
type extDeletionStub struct {
	count int
	t     *testing.T
	err   error
}

func (e *extDeletionStub) DeleteCollection(options *metav1.ListOptions) error {
	require.Equal(e.t, options, &metav1.ListOptions{
		LabelSelector: labels.Set{
			exttokenstore.KindLabel: exttokenstore.IsLogin,
		}.AsSelector().String(),
	})
	e.count = e.count + 1
	return e.err
}

type fakeMigrationConfigMapClient struct {
	controllerv1.ConfigMapController
	cm *corev1.ConfigMap
}

type fakeMigrationConfigMapCache struct {
	generic.CacheInterface[*corev1.ConfigMap]
	client *fakeMigrationConfigMapClient
}

func (c *fakeMigrationConfigMapCache) Get(namespace, name string) (*corev1.ConfigMap, error) {
	return c.client.Get(namespace, name, metav1.GetOptions{})
}

func (f *fakeMigrationConfigMapClient) Cache() generic.CacheInterface[*corev1.ConfigMap] {
	return &fakeMigrationConfigMapCache{client: f}
}

func (f *fakeMigrationConfigMapClient) Get(namespace, name string, opts metav1.GetOptions) (*corev1.ConfigMap, error) {
	if f.cm != nil {
		return f.cm, nil
	}
	return nil, apierrors.NewNotFound(corev1.Resource("configmaps"), name)
}

func (f *fakeMigrationConfigMapClient) Create(cm *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	f.cm = cm
	return cm, nil
}

func (f *fakeMigrationConfigMapClient) Update(cm *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	f.cm = cm
	return cm, nil
}
