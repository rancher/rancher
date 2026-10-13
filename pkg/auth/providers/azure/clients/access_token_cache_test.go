package clients

import (
	"context"
	"strings"
	"testing"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/cache"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	wranglerfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestAccessTokenSecretName(t *testing.T) {
	assert.Equal(t, "azuread-access-token", AccessTokenSecretName("azuread"))
	assert.Equal(t, "azuread-access-token", AccessTokenSecretName(""))
	assert.Equal(t, "azure-eu-access-token", AccessTokenSecretName("azure-eu"))

	longName := AccessTokenSecretName(strings.Repeat("a", 253))
	assert.LessOrEqual(t, len(longName), 253)
	assert.True(t, strings.HasSuffix(longName, "-access-token"))
}

func TestAccessTokenCacheIsPerConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := map[string]*corev1.Secret{}
	secrets := newFakeSecretController(ctrl, store)

	defaultCache := accessTokenCache{Secrets: secrets, ConfigName: "azuread"}
	euCache := accessTokenCache{Secrets: secrets, ConfigName: "azure-eu"}

	require.NoError(t, defaultCache.Export(context.Background(), fakeCacheData("default-token"), cache.ExportHints{}))
	require.NoError(t, euCache.Export(context.Background(), fakeCacheData("eu-token"), cache.ExportHints{}))

	assert.Contains(t, store, "azuread-access-token")
	assert.Contains(t, store, "azure-eu-access-token")

	var defaultToken, euToken fakeCacheData
	require.NoError(t, defaultCache.Replace(context.Background(), &defaultToken, cache.ReplaceHints{}))
	require.NoError(t, euCache.Replace(context.Background(), &euToken, cache.ReplaceHints{}))

	assert.Equal(t, fakeCacheData("default-token"), defaultToken)
	assert.Equal(t, fakeCacheData("eu-token"), euToken)
}

type fakeCacheData string

func (f fakeCacheData) Marshal() ([]byte, error) {
	return []byte(f), nil
}

func (f *fakeCacheData) Unmarshal(b []byte) error {
	*f = fakeCacheData(b)
	return nil
}

func newFakeSecretController(ctrl *gomock.Controller, store map[string]*corev1.Secret) wcorev1.SecretController {
	notFound := func(name string) error {
		return apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, name)
	}
	get := func(namespace, name string) (*corev1.Secret, error) {
		if namespace != common.SecretsNamespace {
			return nil, notFound(name)
		}
		if s, ok := store[name]; ok {
			return s, nil
		}
		return nil, notFound(name)
	}
	save := func(s *corev1.Secret) (*corev1.Secret, error) {
		s = s.DeepCopy()
		if s.Data == nil {
			s.Data = map[string][]byte{}
		}
		for k, v := range s.StringData {
			s.Data[k] = []byte(v)
		}
		store[s.Name] = s
		return s, nil
	}

	secretCache := wranglerfake.NewMockCacheInterface[*corev1.Secret](ctrl)
	secretCache.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(get).AnyTimes()

	secrets := wranglerfake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
	secrets.EXPECT().Cache().Return(secretCache).AnyTimes()
	secrets.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(namespace, name string, _ metav1.GetOptions) (*corev1.Secret, error) {
		return get(namespace, name)
	}).AnyTimes()
	secrets.EXPECT().Create(gomock.Any()).DoAndReturn(save).AnyTimes()
	secrets.EXPECT().Update(gomock.Any()).DoAndReturn(save).AnyTimes()

	return secrets
}
