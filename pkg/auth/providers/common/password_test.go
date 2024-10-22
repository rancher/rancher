package common

import (
	"testing"

	clientv3 "github.com/rancher/rancher/pkg/client/generated/management/v3"
	wranglerfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
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
	var createdSecret *v1.Secret
	secretController.EXPECT().Create(gomock.Any()).DoAndReturn(func(secret *v1.Secret) (*v1.Secret, error) {
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
