package secrets

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestCleanupClientSecretsNilConfig(t *testing.T) {
	secrets := getSecretInterfaceMock(map[string]*corev1.Secret{})
	err := CleanupClientSecrets(secrets, nil)
	require.Error(t, err)
}

func TestCleanupClientSecretsUnknownConfig(t *testing.T) {
	secrets := getSecretInterfaceMock(map[string]*corev1.Secret{})

	config := &v3.AuthConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "unknownConfig"},
		Enabled:    true,
	}

	err := CleanupClientSecrets(secrets, config)
	require.Error(t, err)
}

func TestCleanupClientSecretsKnownConfig(t *testing.T) {
	config := &v3.AuthConfig{
		Type:       client.GoogleOauthConfigType,
		ObjectMeta: metav1.ObjectMeta{Name: "googleoauth"},
		Enabled:    true,
	}

	const (
		oauthCredential     = "oauthcredential"
		serviceAccountToken = "serviceaccountcredential"
	)
	secretName1 := fmt.Sprintf("%s-%s", strings.ToLower(config.Type), oauthCredential)
	secretName2 := fmt.Sprintf("%s-%s", strings.ToLower(config.Type), serviceAccountToken)
	oauthSecretName := "user123-secret"

	initialStore := map[string]*corev1.Secret{}
	secrets := getSecretInterfaceMock(initialStore)

	_, err := secrets.Create(&v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName1,
			Namespace: common.SecretsNamespace,
		},
	})
	assert.NoError(t, err)

	_, err = secrets.Create(&v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName2,
			Namespace: common.SecretsNamespace,
		},
	})
	assert.NoError(t, err)

	_, err = secrets.Create(&v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      oauthSecretName,
			Namespace: tokens.SecretNamespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			config.Name: []byte("my user token"),
		},
	})
	assert.NoError(t, err)

	s, err := secrets.GetNamespaced(common.SecretsNamespace, secretName1, metav1.GetOptions{})
	assert.NoErrorf(t, err, "expected to find the secret %s belonging to the disabled auth provider", secretName1)

	s, err = secrets.GetNamespaced(common.SecretsNamespace, secretName2, metav1.GetOptions{})
	assert.NoErrorf(t, err, "expected to find the secret %s belonging to the disabled auth provider", secretName2)

	s, err = secrets.GetNamespaced(tokens.SecretNamespace, oauthSecretName, metav1.GetOptions{})
	assert.NoErrorf(t, err, "expected to find the secret %s belonging to the disabled auth provider", oauthSecretName)

	err = CleanupClientSecrets(secrets, config)
	assert.NoError(t, err)

	t.Run("Cleanup deletes provider secrets", func(t *testing.T) {
		s, err = secrets.GetNamespaced(common.SecretsNamespace, secretName1, metav1.GetOptions{})
		assert.Errorf(t, err, "expected to not find the secret %s belonging to the disabled auth provider", secretName1)
		assert.Nil(t, s, "expected the secret to be nil")

		s, err = secrets.GetNamespaced(common.SecretsNamespace, secretName2, metav1.GetOptions{})
		assert.Errorf(t, err, "expected to not find the secret %s belonging to the disabled auth provider", secretName2)
		assert.Nil(t, s, "expected the secret to be nil")
	})

	t.Run("Cleanup deletes OAuth secrets", func(t *testing.T) {
		s, err = secrets.GetNamespaced(tokens.SecretNamespace, oauthSecretName, metav1.GetOptions{})
		assert.Errorf(t, err, "expected to not find the secret %s belonging to the disabled auth provider", oauthSecretName)
		assert.Nil(t, s, "expected the secret to be nil")
	})
}

func TestCleanupDeprecatedSecretsKnownConfig(t *testing.T) {
	config := &v3.AuthConfig{
		Type:       client.AzureADConfigType,
		ObjectMeta: metav1.ObjectMeta{Name: "azuread"},
		Enabled:    true,
	}

	secretName := fmt.Sprintf("%s-%s", strings.ToLower(config.Name), "access-token")
	initialStore := map[string]*corev1.Secret{}
	secrets := getSecretInterfaceMock(initialStore)

	_, err := secrets.Create(&v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: common.SecretsNamespace,
		},
	})
	assert.NoError(t, err)

	s, err := secrets.GetNamespaced(common.SecretsNamespace, secretName, metav1.GetOptions{})
	assert.NoErrorf(t, err, "expected to find the secret %s belonging to the disabled auth provider", secretName)

	err = CleanupClientSecrets(secrets, config)
	assert.NoError(t, err)

	s, err = secrets.GetNamespaced(common.SecretsNamespace, secretName, metav1.GetOptions{})
	assert.Errorf(t, err, "expected to not find the secret %s belonging to the disabled auth provider", secretName)
	assert.Nil(t, s, "expected the secret to be nil")
}

func getSecretInterfaceMock(store map[string]*corev1.Secret) v1.SecretInterface {
	secretInterfaceMock := &fakes.SecretInterfaceMock{}

	secretInterfaceMock.CreateFunc = func(secret *corev1.Secret) (*corev1.Secret, error) {
		if secret.Name == "" {
			uniqueIdentifier := md5.Sum([]byte(time.Now().String()))
			secret.Name = hex.EncodeToString(uniqueIdentifier[:])
		}
		store[fmt.Sprintf("%s:%s", secret.Namespace, secret.Name)] = secret
		return secret, nil
	}

	secretInterfaceMock.GetNamespacedFunc = func(namespace string, name string, opts metav1.GetOptions) (*corev1.Secret, error) {
		secret, ok := store[fmt.Sprintf("%s:%s", namespace, name)]
		if ok {
			return secret, nil
		}
		return nil, errors.New("secret not found")
	}

	secretInterfaceMock.DeleteNamespacedFunc = func(namespace string, name string, options *metav1.DeleteOptions) error {
		key := fmt.Sprintf("%s:%s", namespace, name)
		if _, ok := store[key]; !ok {
			return apierrors.NewNotFound(schema.GroupResource{
				Group:    v1.SecretGroupVersionKind.Group,
				Resource: v1.SecretGroupVersionResource.Resource,
			}, key)
		}
		delete(store, key)
		return nil
	}

	secretInterfaceMock.ListNamespacedFunc = func(namespace string, opts metav1.ListOptions) (*corev1.SecretList, error) {
		var secrets []corev1.Secret
		for _, s := range store {
			secrets = append(secrets, *s)
		}
		return &corev1.SecretList{Items: secrets}, nil
	}

	return secretInterfaceMock
}

func TestCleanupClientSecretsOKTAConfig(t *testing.T) {
	config := &v3.AuthConfig{
		Type:       client.OKTAConfigType,
		ObjectMeta: metav1.ObjectMeta{Name: "okta"},
		Enabled:    true,
	}

	secretName1 := fmt.Sprintf("%s-%s", strings.ToLower(config.Type), "spkey")
	secretName2 := fmt.Sprintf("%s-%s", strings.ToLower(config.Type), "serviceaccountpassword")
	oauthSecretName := "user123-secret"

	initialStore := map[string]*corev1.Secret{}
	secrets := getSecretInterfaceMock(initialStore)

	_, err := secrets.Create(&v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName1,
			Namespace: common.SecretsNamespace,
		},
	})
	assert.NoError(t, err)

	_, err = secrets.Create(&v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName2,
			Namespace: common.SecretsNamespace,
		},
	})
	assert.NoError(t, err)

	_, err = secrets.Create(&v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      oauthSecretName,
			Namespace: tokens.SecretNamespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			config.Name: []byte("my user token"),
		},
	})
	assert.NoError(t, err)

	s, err := secrets.GetNamespaced(common.SecretsNamespace, secretName1, metav1.GetOptions{})
	assert.NoErrorf(t, err, "expected to find the secret %s belonging to the disabled auth provider", secretName1)

	s, err = secrets.GetNamespaced(common.SecretsNamespace, secretName2, metav1.GetOptions{})
	assert.NoErrorf(t, err, "expected to find the secret %s belonging to the disabled auth provider", secretName2)

	s, err = secrets.GetNamespaced(tokens.SecretNamespace, oauthSecretName, metav1.GetOptions{})
	assert.NoErrorf(t, err, "expected to find the secret %s belonging to the disabled auth provider", oauthSecretName)

	err = CleanupClientSecrets(secrets, config)
	assert.NoError(t, err)

	t.Run("Cleanup deletes provider secrets", func(t *testing.T) {
		s, err = secrets.GetNamespaced(common.SecretsNamespace, secretName1, metav1.GetOptions{})
		assert.Errorf(t, err, "expected to not find the secret %s belonging to the disabled auth provider", secretName1)
		assert.Nil(t, s, "expected the secret to be nil")

		s, err = secrets.GetNamespaced(common.SecretsNamespace, secretName2, metav1.GetOptions{})
		assert.Errorf(t, err, "expected to not find the secret %s belonging to the disabled auth provider", secretName2)
		assert.Nil(t, s, "expected the secret to be nil")
	})
}
