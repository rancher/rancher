package common

import (
	"fmt"
	"strings"
	"testing"

	clientv3 "github.com/rancher/rancher/pkg/client/generated/management/v3"
	wranglerfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	appSecretKey   = "applicationSecret"
	appSecretValue = "superSecret"
)

var tests = []struct {
	in  string
	out string
}{
	{in: "potato", out: "potato"},
	{in: SecretsNamespace + "-foo", out: SecretsNamespace + "-foo"},
	{in: SecretsNamespace + ":bar", out: appSecretValue},
	{in: "bad:thing", out: "bad:thing"},
	{in: SecretsNamespace + ":baz", out: "error"}, // expecting an error or different output for 'baz'

}

func TestReadFromSecret(t *testing.T) {
	ctrl := gomock.NewController(t)

	secretController := wranglerfake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
	secretController.EXPECT().Get("cattle-global-data", "bar", gomock.Any()).Return(&corev1.Secret{
		Data: map[string][]byte{
			appSecretKey: []byte(appSecretValue),
		},
	}, nil).AnyTimes()

	// If the secret name is not "bar" return an error
	secretController.EXPECT().Get(gomock.Any(), gomock.Not("bar"), gomock.Any()).Return(nil, apierrors.NewNotFound(schema.GroupResource{}, "secret not found")).AnyTimes()

	for _, pair := range tests {
		info, err := ReadFromSecret(secretController, pair.in, appSecretKey)
		if pair.out == "error" {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, pair.out, info)
		}
	}
}

func TestNameForSecret(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shibbolethconfig-serviceaccountpassword",
			Namespace: "cattle-global-data",
		},
		StringData: map[string]string{
			"serviceaccountpassword": "test-password",
		},
		Type: corev1.SecretTypeOpaque,
	}

	want := "cattle-global-data:shibbolethconfig-serviceaccountpassword"
	if n := NameForSecret(secret); n != want {
		t.Errorf("NameForSecret() got %s, want t%s", n, want)
	}
}

func TestSavePasswordSecret(t *testing.T) {
	wantSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shibbolethconfig-serviceaccountpassword",
			Namespace: "cattle-global-data",
		},
		StringData: map[string]string{
			"serviceaccountpassword": "test-password",
		},
		Type: corev1.SecretTypeOpaque,
	}

	ctrl := gomock.NewController(t)
	secretController := wranglerfake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
	var createdSecret *corev1.Secret
	secretController.EXPECT().Create(gomock.Any()).DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
		createdSecret = secret
		return secret, nil
	})
	secretsCache := wranglerfake.NewMockCacheInterface[*corev1.Secret](ctrl)
	secretsCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, apierrors.NewNotFound(schema.GroupResource{}, "test-password"))
	secretController.EXPECT().Cache().Return(secretsCache)

	name, err := SavePasswordSecret(secretController, "test-password",
		clientv3.LdapConfigFieldServiceAccountPassword,
		"shibbolethConfig")
	assert.NoError(t, err)
	assert.Equal(t, wantSecret.Namespace+":"+wantSecret.Name, name)
	assert.Equal(t, wantSecret, createdSecret)
}

func TestCreateOrUpdateSecretsNoUpdateWhenUnchanged(t *testing.T) {
	const (
		field    = "password"
		authType = "ldapconfig"
		value    = "s3cr3t"
	)
	secretName := fmt.Sprintf("%s-%s", authType, field)

	ctrl := gomock.NewController(t)
	secretController := wranglerfake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
	secretsCache := wranglerfake.NewMockCacheInterface[*corev1.Secret](ctrl)

	// The secret already exists and its Data already holds the correct value.
	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: SecretsNamespace,
		},
		Data: map[string][]byte{field: []byte(value)},
		Type: corev1.SecretTypeOpaque,
	}
	secretsCache.EXPECT().Get(SecretsNamespace, secretName).Return(existing, nil)
	secretController.EXPECT().Cache().Return(secretsCache)

	// Update must NOT be called when the stored value is already correct.
	secretController.EXPECT().Update(gomock.Any()).Times(0)

	got, err := CreateOrUpdateSecrets(secretController, value, field, authType)
	assert.NoError(t, err)
	assert.Equal(t, SecretsNamespace+":"+secretName, got)
}

func TestProviderNameFromType(t *testing.T) {
	for configType, want := range map[string]string{
		clientv3.GithubConfigType:          "github",
		clientv3.AzureADConfigType:         "azuread",
		clientv3.KeyCloakOIDCConfigType:    "keycloakoidc",
		clientv3.GenericSAMLConfigType:     "genericsaml",
		clientv3.ActiveDirectoryConfigType: "activedirectory",
	} {
		assert.Equal(t, want, ProviderNameFromType(configType), configType)
	}
}

func TestSecretNamePrefix(t *testing.T) {
	tests := []struct {
		name       string
		configName string
		configType string
		want       string
	}{
		{name: "default config", configName: "github", configType: clientv3.GithubConfigType, want: "githubconfig"},
		{name: "no config name", configName: "", configType: clientv3.GithubConfigType, want: "githubconfig"},
		{name: "default config with mixed case type", configName: "keycloakoidc", configType: clientv3.KeyCloakOIDCConfigType, want: "keycloakoidcconfig"},
		{name: "additional config", configName: "github-eu", configType: clientv3.GithubConfigType, want: "githubconfig-github-eu"},
		{name: "additional config named after another provider", configName: "azuread", configType: clientv3.GithubConfigType, want: "githubconfig-azuread"},
		{name: "additional config with a long name", configName: strings.Repeat("a", 253), configType: clientv3.GithubConfigType, want: "githubconfig-" + strings.Repeat("a", 44) + "-d3f66"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SecretNamePrefix(tt.configName, tt.configType)
			assert.Equal(t, tt.want, got)
			assert.LessOrEqual(t, len(got), 63)
		})
	}
}

func TestSecretNamePrefixLongNamesAreDistinct(t *testing.T) {
	longName := strings.Repeat("a", 100)

	assert.NotEqual(t,
		SecretNamePrefix(longName+"-eu", clientv3.GithubConfigType),
		SecretNamePrefix(longName+"-us", clientv3.GithubConfigType),
	)
}
