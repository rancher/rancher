package rancher

import (
	"fmt"
	"testing"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	exttokenstore "github.com/rancher/rancher/pkg/ext/stores/tokens"
	// v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/tokens"
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
		secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
	) {
		// migrate legacy tokens
		tCache := fake.NewMockNonNamespacedCacheInterface[*apiv3.Token](ctrl)
		v3tokens.EXPECT().Cache().
			Return(tCache)
		tCache.EXPECT().List(labels.SelectorFromSet(labels.Set{tokens.TokenKindLabel: "session"})).
			Return([]*apiv3.Token{
				&apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "v3session"}},
			}, nil)
		v3tokens.EXPECT().Delete("v3session", gomock.Any()).
			Return(nil)

		// migrate ext tokens
		secrets.EXPECT().
			List(exttokenstore.TokenNamespace, metav1.ListOptions{
				LabelSelector: labels.Set{
					exttokenstore.KindLabel: exttokenstore.IsLogin,
				}.AsSelector().String(),
			}).Return(&corev1.SecretList{
			Items: []corev1.Secret{
				{ObjectMeta: metav1.ObjectMeta{Name: "extsession"}},
			}}, nil)
		secrets.EXPECT().Delete(exttokenstore.TokenNamespace, "extsession", gomock.Any()).
			Return(nil)
	}

	tests := map[string]struct {
		err   error
		setup func(
			ctrl *gomock.Controller,
			configmaps *fake.MockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList],
			cmCache *fake.MockCacheInterface[*corev1.ConfigMap],
			tokens *fake.MockNonNamespacedControllerInterface[*apiv3.Token, *apiv3.TokenList],
			secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
		)
	}{
		"config map retrieval error": {
			err: fmt.Errorf("some transient issue"),
			setup: func(
				ctrl *gomock.Controller,
				configmaps *fake.MockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList],
				cmCache *fake.MockCacheInterface[*corev1.ConfigMap],
				tokens *fake.MockNonNamespacedControllerInterface[*apiv3.Token, *apiv3.TokenList],
				secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
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
				secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
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
				secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
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
			setup: func(
				ctrl *gomock.Controller,
				configmaps *fake.MockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList],
				cmCache *fake.MockCacheInterface[*corev1.ConfigMap],
				tokens *fake.MockNonNamespacedControllerInterface[*apiv3.Token, *apiv3.TokenList],
				secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
			) {
				cmCache.EXPECT().Get(cattleNamespace, forceUpgradeLogoutConfig).
					Return(&corev1.ConfigMap{
						Data: map[string]string{
							rancherVersionKey: lastMigrationBeforeTarget,
						},
					}, nil)

				commonMigrationSetup(ctrl, cmCache, tokens, secrets)

				configmaps.EXPECT().Create(&corev1.ConfigMap{
					Data: map[string]string{
						rancherVersionKey: migrationTarget,
					},
				})
			},
		},
		"migration triggered when config map not present": {
			setup: func(
				ctrl *gomock.Controller,
				configmaps *fake.MockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList],
				cmCache *fake.MockCacheInterface[*corev1.ConfigMap],
				tokens *fake.MockNonNamespacedControllerInterface[*apiv3.Token, *apiv3.TokenList],
				secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
			) {
				cmCache.EXPECT().Get(cattleNamespace, forceUpgradeLogoutConfig).Return(nil, nil)

				commonMigrationSetup(ctrl, cmCache, tokens, secrets)

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
			setup: func(
				ctrl *gomock.Controller,
				configmaps *fake.MockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList],
				cmCache *fake.MockCacheInterface[*corev1.ConfigMap],
				tokens *fake.MockNonNamespacedControllerInterface[*apiv3.Token, *apiv3.TokenList],
				secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
			) {
				cmCache.EXPECT().Get(cattleNamespace, forceUpgradeLogoutConfig).
					Return(&corev1.ConfigMap{Data: map[string]string{}}, nil)
				
				commonMigrationSetup (ctrl, cmCache, tokens, secrets)

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

			nsCache := fake.NewMockNonNamespacedCacheInterface[*corev1.Namespace](ctrl)
			nsCache.EXPECT().Get(exttokenstore.TokenNamespace).AnyTimes()

			secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
			scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
			secrets.EXPECT().Cache().Return(scache)

			users := fake.NewMockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList](ctrl)
			ucache := fake.NewMockNonNamespacedCacheInterface[*apiv3.User](ctrl)
			users.EXPECT().Cache().Return(ucache)

			tcache := fake.NewMockNonNamespacedCacheInterface[*apiv3.Token](ctrl)
			ccache := fake.NewMockNonNamespacedCacheInterface[*apiv3.Cluster](ctrl)

			timer := exttokenstore.NewMocktimeHandler(ctrl)
			hasher := exttokenstore.NewMockhashHandler(ctrl)
			auth := exttokenstore.NewMockauthHandler(ctrl)

			estore := exttokenstore.NewSystem(nil, nsCache, secrets, users, tcache, ccache, timer, hasher, auth, nil)

			if spec.setup != nil {
				spec.setup(ctrl, configmaps, cmCache, tokens, secrets)
			}

			err := forceUpgradeLogout(configmaps, tokens, estore, migrationTarget)

			if spec.err != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), spec.err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
