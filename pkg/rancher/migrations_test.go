package rancher

import (
	"fmt"
	"testing"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/tokens"
	exttokenstore "github.com/rancher/rancher/pkg/ext/stores/tokens"
	rancherversion "github.com/rancher/rancher/pkg/version"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

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
}

func (e *extDeletionStub) DeleteCollection(options *metav1.ListOptions) error {
	require.Equal(e.t, options, &metav1.ListOptions{
		LabelSelector: labels.Set{
			exttokenstore.KindLabel: exttokenstore.IsLogin,
		}.AsSelector().String(),
	})
	e.count = e.count + 1
	return nil
}
