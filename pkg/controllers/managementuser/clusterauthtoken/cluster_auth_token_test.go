package clusterauthtoken

import (
	"fmt"
	"testing"
	"time"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	clusterv3 "github.com/rancher/rancher/pkg/generated/norman/cluster.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestClusterAuthTokenHandlerSync(t *testing.T) {
	ctrl := gomock.NewController(t)
	tokenName := "kubeconfig-u-h6knxgjtch"

	t.Run("clusterAuthToken is nil", func(t *testing.T) {
		handler := &clusterAuthTokenHandler{}

		_, err := handler.sync("", nil)
		require.Nil(t, err)
	})

	t.Run("clusterAuthToken is being deleted", func(t *testing.T) {
		handler := &clusterAuthTokenHandler{}

		deletedAt := metav1.NewTime(time.Now())
		_, err := handler.sync("", &clusterv3.ClusterAuthToken{
			ObjectMeta: metav1.ObjectMeta{
				DeletionTimestamp: &deletedAt,
			},
			Enabled: true,
		})
		require.Nil(t, err)
	})

	t.Run("lastUsedAt is not set", func(t *testing.T) {
		handler := &clusterAuthTokenHandler{}

		_, err := handler.sync("", &clusterv3.ClusterAuthToken{
			Enabled: true,
		})
		require.Nil(t, err)

	})

	t.Run("clusterAuthToken is disabled", func(t *testing.T) {
		handler := &clusterAuthTokenHandler{}

		_, err := handler.sync("", &clusterv3.ClusterAuthToken{
			Enabled: false,
		})
		require.Nil(t, err)
	})

	t.Run("clusterAuthToken is expired", func(t *testing.T) {
		handler := &clusterAuthTokenHandler{}

		_, err := handler.sync("", &clusterv3.ClusterAuthToken{
			Enabled:   true,
			ExpiresAt: time.Now().Add(-time.Second).Format(time.RFC3339),
		})
		require.Nil(t, err)
	})

	t.Run("token not found", func(t *testing.T) {
		tokenCache := fake.NewMockNonNamespacedCacheInterface[*apiv3.Token](ctrl)
		tokenCache.EXPECT().Get(tokenName).Return(nil, apierrors.NewNotFound(schema.GroupResource{}, tokenName)).Times(1)

		handler := &clusterAuthTokenHandler{
			tokenCache: tokenCache,
		}

		lastUsedAt := metav1.NewTime(time.Now())

		_, err := handler.sync("", &clusterv3.ClusterAuthToken{
			ObjectMeta: metav1.ObjectMeta{
				Name: tokenName,
			},
			Enabled:    true,
			LastUsedAt: &lastUsedAt,
		})
		require.Nil(t, err)
	})

	t.Run("error getting token", func(t *testing.T) {
		tokenCache := fake.NewMockNonNamespacedCacheInterface[*apiv3.Token](ctrl)
		tokenCache.EXPECT().Get(tokenName).Return(nil, fmt.Errorf("some error")).Times(1)

		handler := &clusterAuthTokenHandler{
			tokenCache: tokenCache,
		}

		lastUsedAt := metav1.NewTime(time.Now())

		_, err := handler.sync("", &clusterv3.ClusterAuthToken{
			ObjectMeta: metav1.ObjectMeta{
				Name: tokenName,
			},
			Enabled:    true,
			LastUsedAt: &lastUsedAt,
		})
		require.Nil(t, err)
	})

	t.Run("token was used more recently than clusterAuthToken", func(t *testing.T) {
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

		handler := &clusterAuthTokenHandler{
			tokenCache: tokenCache,
		}

		_, err := handler.sync("", &clusterv3.ClusterAuthToken{
			ObjectMeta: metav1.ObjectMeta{
				Name: tokenName,
			},
			Enabled:    true,
			LastUsedAt: &clusterAuthTokenLastUsedAt,
		})
		require.Nil(t, err)
	})

	t.Run("token updated successfully", func(t *testing.T) {
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

		handler := &clusterAuthTokenHandler{
			tokenCache:  tokenCache,
			tokenClient: tokenClient,
		}

		_, err := handler.sync("", &clusterv3.ClusterAuthToken{
			ObjectMeta: metav1.ObjectMeta{
				Name: tokenName,
			},
			Enabled:    true,
			LastUsedAt: &clusterAuthTokenLastUsedAt,
		})
		require.Nil(t, err)
	})

	t.Run("error updating token doesn't fail sync", func(t *testing.T) {
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
		tokenClient.EXPECT().Patch(tokenName, gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("some error")).Times(1)

		handler := &clusterAuthTokenHandler{
			tokenCache:  tokenCache,
			tokenClient: tokenClient,
		}

		_, err := handler.sync("", &clusterv3.ClusterAuthToken{
			ObjectMeta: metav1.ObjectMeta{
				Name: tokenName,
			},
			Enabled:    true,
			LastUsedAt: &clusterAuthTokenLastUsedAt,
		})
		require.Nil(t, err)
	})
}
