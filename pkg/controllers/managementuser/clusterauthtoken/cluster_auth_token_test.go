package clusterauthtoken

import (
	"fmt"
	"testing"
	"time"

	extv1 "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	etoken "github.com/rancher/rancher/pkg/ext/stores/tokens"
	clusterv3 "github.com/rancher/rancher/pkg/generated/norman/cluster.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestClusterAuthTokenHandlerSync(t *testing.T) {
	ctrl := gomock.NewController(t)
	tokenName := "kubeconfig-u-h6knxgjtch"

	t.Run("clusterAuthToken is nil", func(t *testing.T) {
		handler := &clusterAuthTokenHandler{}

		obj, err := handler.sync("", nil)
		require.NoError(t, err)
		require.Nil(t, obj)
	})

	t.Run("clusterAuthToken is being deleted", func(t *testing.T) {
		handler := &clusterAuthTokenHandler{}

		deletedAt := metav1.NewTime(time.Now())
		obj, err := handler.sync("", &clusterv3.ClusterAuthToken{
			ObjectMeta: metav1.ObjectMeta{
				DeletionTimestamp: &deletedAt,
			},
			Enabled: true,
		})
		require.NoError(t, err)
		require.Nil(t, obj)
	})

	t.Run("lastUsedAt is not set", func(t *testing.T) {
		handler := &clusterAuthTokenHandler{}

		obj, err := handler.sync("", &clusterv3.ClusterAuthToken{
			Enabled: true,
		})
		require.NoError(t, err)
		require.NotNil(t, obj)
	})

	t.Run("clusterAuthToken is disabled", func(t *testing.T) {
		handler := &clusterAuthTokenHandler{}

		obj, err := handler.sync("", &clusterv3.ClusterAuthToken{
			Enabled: false,
		})
		require.NoError(t, err)
		require.NotNil(t, obj)
	})

	t.Run("clusterAuthToken is expired", func(t *testing.T) {
		handler := &clusterAuthTokenHandler{}

		obj, err := handler.sync("", &clusterv3.ClusterAuthToken{
			Enabled:   true,
			ExpiresAt: time.Now().Add(-time.Second).Format(time.RFC3339),
		})
		require.NoError(t, err)
		require.NotNil(t, obj)
	})

	t.Run("token not found", func(t *testing.T) {
		tokenCache := fake.NewMockNonNamespacedCacheInterface[*apiv3.Token](ctrl)
		tokenCache.EXPECT().Get(tokenName).
			Return(nil, apierrors.NewNotFound(schema.GroupResource{}, tokenName)).Times(1)

		eTokenCache := fake.NewMockNonNamespacedCacheInterface[*extv1.Token](ctrl)
		eTokenCache.EXPECT().Get(tokenName).
			Return(nil, apierrors.NewNotFound(schema.GroupResource{}, tokenName)).Times(1)

		handler := &clusterAuthTokenHandler{
			tokenCache:  tokenCache,
			eTokenCache: eTokenCache,
		}

		lastUsedAt := metav1.NewTime(time.Now())

		obj, err := handler.sync("", &clusterv3.ClusterAuthToken{
			ObjectMeta: metav1.ObjectMeta{
				Name: tokenName,
			},
			Enabled:    true,
			LastUsedAt: &lastUsedAt,
		})
		require.NoError(t, err)
		require.NotNil(t, obj)
	})

	t.Run("error getting v3 token", func(t *testing.T) {
		tokenCache := fake.NewMockNonNamespacedCacheInterface[*apiv3.Token](ctrl)
		tokenCache.EXPECT().Get(tokenName).Return(nil, fmt.Errorf("some error")).Times(1)

		eTokenCache := fake.NewMockNonNamespacedCacheInterface[*extv1.Token](ctrl)
		eTokenCache.EXPECT().Get(tokenName).
			Return(nil, apierrors.NewNotFound(schema.GroupResource{}, tokenName)).Times(1)

		handler := &clusterAuthTokenHandler{
			tokenCache:  tokenCache,
			eTokenCache: eTokenCache,
		}

		lastUsedAt := metav1.NewTime(time.Now())

		obj, err := handler.sync("", &clusterv3.ClusterAuthToken{
			ObjectMeta: metav1.ObjectMeta{
				Name: tokenName,
			},
			Enabled:    true,
			LastUsedAt: &lastUsedAt,
		})
		require.Error(t, err)
		require.Nil(t, obj)
	})

	t.Run("error getting ext token", func(t *testing.T) {
		eTokenCache := fake.NewMockNonNamespacedCacheInterface[*extv1.Token](ctrl)
		eTokenCache.EXPECT().Get(tokenName).Return(nil, fmt.Errorf("some error")).Times(1)

		handler := &clusterAuthTokenHandler{
			eTokenCache: eTokenCache,
		}

		lastUsedAt := metav1.NewTime(time.Now())

		obj, err := handler.sync("", &clusterv3.ClusterAuthToken{
			ObjectMeta: metav1.ObjectMeta{
				Name: tokenName,
			},
			Enabled:    true,
			LastUsedAt: &lastUsedAt,
		})
		require.Error(t, err)
		require.Nil(t, obj)
	})

	t.Run("v3 token is expired", func(t *testing.T) {
		now := time.Now()
		tokenLastUsedAt := metav1.NewTime(now.Add(-time.Minute))
		tokenCreatedAt := metav1.NewTime(now.Add(-time.Hour))
		clusterAuthTokenLastUsedAt := metav1.NewTime(now)

		tokenCache := fake.NewMockNonNamespacedCacheInterface[*apiv3.Token](ctrl)
		tokenCache.EXPECT().Get(tokenName).Return(&apiv3.Token{
			ObjectMeta: metav1.ObjectMeta{
				Name:              tokenName,
				CreationTimestamp: tokenCreatedAt,
			},
			LastUsedAt: &tokenLastUsedAt,
			TTLMillis:  now.Sub(tokenCreatedAt.Time).Milliseconds() - 1,
		}, nil).Times(1)

		eTokenCache := fake.NewMockNonNamespacedCacheInterface[*extv1.Token](ctrl)
		eTokenCache.EXPECT().Get(tokenName).
			Return(nil, apierrors.NewNotFound(schema.GroupResource{}, tokenName)).Times(1)

		handler := &clusterAuthTokenHandler{
			tokenCache:  tokenCache,
			eTokenCache: eTokenCache,
		}

		obj, err := handler.sync("", &clusterv3.ClusterAuthToken{
			ObjectMeta: metav1.ObjectMeta{
				Name: tokenName,
			},
			Enabled:    true,
			LastUsedAt: &clusterAuthTokenLastUsedAt,
		})
		require.NoError(t, err)
		require.NotNil(t, obj)
	})

	t.Run("ext token is expired", func(t *testing.T) {
		now := time.Now()
		tokenLastUsedAt := metav1.NewTime(now.Add(-time.Minute))
		tokenCreatedAt := metav1.NewTime(now.Add(-time.Hour))
		clusterAuthTokenLastUsedAt := metav1.NewTime(now)

		eTokenCache := fake.NewMockNonNamespacedCacheInterface[*extv1.Token](ctrl)
		eTokenCache.EXPECT().Get(tokenName).Return(&extv1.Token{
			ObjectMeta: metav1.ObjectMeta{
				Name:              tokenName,
				CreationTimestamp: tokenCreatedAt,
			},
			Spec: extv1.TokenSpec{
				TTL: now.Sub(tokenCreatedAt.Time).Milliseconds() - 1,
			},
			Status: extv1.TokenStatus{
				LastUsedAt: &tokenLastUsedAt,
				Expired:    true,
			},
		}, nil).Times(1)

		handler := &clusterAuthTokenHandler{
			eTokenCache: eTokenCache,
		}

		obj, err := handler.sync("", &clusterv3.ClusterAuthToken{
			ObjectMeta: metav1.ObjectMeta{
				Name: tokenName,
			},
			Enabled:    true,
			LastUsedAt: &clusterAuthTokenLastUsedAt,
		})
		require.NoError(t, err)
		require.NotNil(t, obj)
	})

	t.Run("v3 token was used more recently than clusterAuthToken", func(t *testing.T) {
		now := time.Now()
		tokenLastUsedAt := metav1.NewTime(now)
		clusterAuthTokenLastUsedAt := metav1.NewTime(now.Add(-time.Second))

		tokenCache := fake.NewMockNonNamespacedCacheInterface[*apiv3.Token](ctrl)
		tokenCache.EXPECT().Get(tokenName).Return(&apiv3.Token{
			ObjectMeta: metav1.ObjectMeta{
				Name: tokenName,
			},
			LastUsedAt: &tokenLastUsedAt,
		}, nil).Times(1)

		eTokenCache := fake.NewMockNonNamespacedCacheInterface[*extv1.Token](ctrl)
		eTokenCache.EXPECT().Get(tokenName).
			Return(nil, apierrors.NewNotFound(schema.GroupResource{}, tokenName)).Times(1)

		handler := &clusterAuthTokenHandler{
			tokenCache:  tokenCache,
			eTokenCache: eTokenCache,
		}

		obj, err := handler.sync("", &clusterv3.ClusterAuthToken{
			ObjectMeta: metav1.ObjectMeta{
				Name: tokenName,
			},
			Enabled:    true,
			LastUsedAt: &clusterAuthTokenLastUsedAt,
		})
		require.NoError(t, err)
		require.NotNil(t, obj)
	})

	t.Run("ext token was used more recently than clusterAuthToken", func(t *testing.T) {
		now := time.Now()
		tokenLastUsedAt := metav1.NewTime(now)
		clusterAuthTokenLastUsedAt := metav1.NewTime(now.Add(-time.Second))

		eTokenCache := fake.NewMockNonNamespacedCacheInterface[*extv1.Token](ctrl)
		eTokenCache.EXPECT().Get(tokenName).Return(&extv1.Token{
			ObjectMeta: metav1.ObjectMeta{
				Name: tokenName,
			},
			Status: extv1.TokenStatus{
				LastUsedAt: &tokenLastUsedAt,
			},
		}, nil).Times(1)

		handler := &clusterAuthTokenHandler{
			eTokenCache: eTokenCache,
		}

		obj, err := handler.sync("", &clusterv3.ClusterAuthToken{
			ObjectMeta: metav1.ObjectMeta{
				Name: tokenName,
			},
			Enabled:    true,
			LastUsedAt: &clusterAuthTokenLastUsedAt,
		})
		require.NoError(t, err)
		require.NotNil(t, obj)
	})

	t.Run("v3 token updated successfully", func(t *testing.T) {
		now := time.Now()
		tokenLastUsedAt := metav1.NewTime(now.Add(-time.Second))
		clusterAuthTokenLastUsedAt := metav1.NewTime(now)

		tokenCache := fake.NewMockNonNamespacedCacheInterface[*apiv3.Token](ctrl)
		tokenCache.EXPECT().Get(tokenName).Return(&apiv3.Token{
			ObjectMeta: metav1.ObjectMeta{
				Name: tokenName,
			},
			LastUsedAt: &tokenLastUsedAt,
		}, nil).Times(1)

		tokenClient := fake.NewMockNonNamespacedClientInterface[*apiv3.Token, *apiv3.TokenList](ctrl)
		tokenClient.EXPECT().Patch(tokenName, gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)

		eTokenCache := fake.NewMockNonNamespacedCacheInterface[*extv1.Token](ctrl)
		eTokenCache.EXPECT().Get(tokenName).
			Return(nil, apierrors.NewNotFound(schema.GroupResource{}, tokenName)).Times(1)

		handler := &clusterAuthTokenHandler{
			tokenCache:  tokenCache,
			tokenClient: tokenClient,
			eTokenCache: eTokenCache,
		}

		obj, err := handler.sync("", &clusterv3.ClusterAuthToken{
			ObjectMeta: metav1.ObjectMeta{
				Name: tokenName,
			},
			Enabled:    true,
			LastUsedAt: &clusterAuthTokenLastUsedAt,
		})
		require.NoError(t, err)
		require.NotNil(t, obj)
	})

	t.Run("ext token updated successfully", func(t *testing.T) {
		now := time.Now()
		tokenLastUsedAt := metav1.NewTime(now.Add(-time.Second))
		clusterAuthTokenLastUsedAt := metav1.NewTime(now)

		eTokenCache := fake.NewMockNonNamespacedCacheInterface[*extv1.Token](ctrl)
		eTokenCache.EXPECT().Get(tokenName).Return(&extv1.Token{
			ObjectMeta: metav1.ObjectMeta{
				Name: tokenName,
			},
			Status: extv1.TokenStatus{
				LastUsedAt: &tokenLastUsedAt,
			},
		}, nil).Times(1)

		eSecretClient := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		eSecretClient.EXPECT().Patch("cattle-tokens", tokenName, gomock.Any(), gomock.Any()).
			Return(nil, nil).Times(1)
		eSecretClient.EXPECT().Cache().Return(nil)

		users := fake.NewMockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList](ctrl)
		users.EXPECT().Cache().Return(nil)

		eTokenStore := etoken.NewSystem(nil, nil, eSecretClient, users, nil, nil, nil, nil, nil)

		handler := &clusterAuthTokenHandler{
			eTokenCache: eTokenCache,
			eTokenStore: eTokenStore,
		}

		obj, err := handler.sync("", &clusterv3.ClusterAuthToken{
			ObjectMeta: metav1.ObjectMeta{
				Name: tokenName,
			},
			Enabled:    true,
			LastUsedAt: &clusterAuthTokenLastUsedAt,
		})
		require.NoError(t, err)
		require.NotNil(t, obj)
	})

	t.Run("error updating v3 token", func(t *testing.T) {
		now := time.Now()
		tokenLastUsedAt := metav1.NewTime(now.Add(-time.Second))
		clusterAuthTokenLastUsedAt := metav1.NewTime(now)

		tokenCache := fake.NewMockNonNamespacedCacheInterface[*apiv3.Token](ctrl)
		tokenCache.EXPECT().Get(tokenName).Return(&apiv3.Token{
			ObjectMeta: metav1.ObjectMeta{
				Name: tokenName,
			},
			LastUsedAt: &tokenLastUsedAt,
		}, nil).Times(1)

		tokenClient := fake.NewMockNonNamespacedClientInterface[*apiv3.Token, *apiv3.TokenList](ctrl)
		tokenClient.EXPECT().
			Patch(tokenName, gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("some error")).
			Times(1)

		eTokenCache := fake.NewMockNonNamespacedCacheInterface[*extv1.Token](ctrl)
		eTokenCache.EXPECT().Get(tokenName).
			Return(nil, apierrors.NewNotFound(schema.GroupResource{}, tokenName)).Times(1)

		handler := &clusterAuthTokenHandler{
			tokenCache:  tokenCache,
			tokenClient: tokenClient,
			eTokenCache: eTokenCache,
		}

		obj, err := handler.sync("", &clusterv3.ClusterAuthToken{
			ObjectMeta: metav1.ObjectMeta{
				Name: tokenName,
			},
			Enabled:    true,
			LastUsedAt: &clusterAuthTokenLastUsedAt,
		})
		require.Error(t, err)
		require.Nil(t, obj)
	})

	t.Run("error updating ext token", func(t *testing.T) {
		now := time.Now()
		tokenLastUsedAt := metav1.NewTime(now.Add(-time.Second))
		clusterAuthTokenLastUsedAt := metav1.NewTime(now)

		eTokenCache := fake.NewMockNonNamespacedCacheInterface[*extv1.Token](ctrl)
		eTokenCache.EXPECT().Get(tokenName).Return(&extv1.Token{
			ObjectMeta: metav1.ObjectMeta{
				Name: tokenName,
			},
			Status: extv1.TokenStatus{
				LastUsedAt: &tokenLastUsedAt,
			},
		}, nil).Times(1)

		eSecretClient := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		eSecretClient.EXPECT().Patch("cattle-tokens", tokenName, gomock.Any(), gomock.Any()).
			Return(nil, fmt.Errorf("some error")).Times(1)
		eSecretClient.EXPECT().Cache().Return(nil)

		users := fake.NewMockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList](ctrl)
		users.EXPECT().Cache().Return(nil)

		eTokenStore := etoken.NewSystem(nil, nil, eSecretClient, users, nil, nil, nil, nil, nil)

		handler := &clusterAuthTokenHandler{
			eTokenCache: eTokenCache,
			eTokenStore: eTokenStore,
		}

		obj, err := handler.sync("", &clusterv3.ClusterAuthToken{
			ObjectMeta: metav1.ObjectMeta{
				Name: tokenName,
			},
			Enabled:    true,
			LastUsedAt: &clusterAuthTokenLastUsedAt,
		})
		require.Error(t, err)
		require.Nil(t, obj)
	})
}
