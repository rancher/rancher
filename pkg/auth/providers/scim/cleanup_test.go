package scim

import (
	"fmt"
	"testing"

	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestCleanupSecrets(t *testing.T) {
	ctrl := gomock.NewController(t)
	provider := "okta"

	wantSelector := labels.Set{
		secretKindLabel:   scimAuthToken,
		authProviderLabel: provider,
	}.AsSelector()

	t.Run("no secrets to delete", func(t *testing.T) {
		secrets := fake.NewMockControllerInterface[*v1.Secret, *v1.SecretList](ctrl)
		secrets.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(namespace string, opts metav1.ListOptions) (*v1.SecretList, error) {
			assert.Equal(t, tokenSecretNamespace, namespace)
			assert.Equal(t, wantSelector.String(), opts.LabelSelector)

			return &v1.SecretList{}, nil
		}).Times(1)

		err := CleanupSecrets(secrets, provider)
		require.NoError(t, err)
	})

	t.Run("secrets deleted successfully", func(t *testing.T) {
		secret1 := v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "scim-token-1",
				Namespace: tokenSecretNamespace,
				Labels: map[string]string{
					secretKindLabel:   scimAuthToken,
					authProviderLabel: provider,
				},
			},
			Data: map[string][]byte{
				"token": []byte("test-token-1"),
			},
		}
		secret2 := v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "scim-token-2",
				Namespace: tokenSecretNamespace,
				Labels: map[string]string{
					secretKindLabel:   scimAuthToken,
					authProviderLabel: provider,
				},
			},
			Data: map[string][]byte{
				"token": []byte("test-token-2"),
			},
		}
		secret3 := v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "scim-token-3",
				Namespace: tokenSecretNamespace,
				Labels: map[string]string{
					secretKindLabel:   scimAuthToken,
					authProviderLabel: provider,
				},
			},
			Data: map[string][]byte{
				"token": []byte("test-token-3"),
			},
		}

		secrets := fake.NewMockControllerInterface[*v1.Secret, *v1.SecretList](ctrl)
		secrets.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(namespace string, opts metav1.ListOptions) (*v1.SecretList, error) {
			assert.Equal(t, tokenSecretNamespace, namespace)
			assert.Equal(t, wantSelector.String(), opts.LabelSelector)

			return &v1.SecretList{Items: []v1.Secret{secret1, secret2, secret3}}, nil
		}).Times(1)

		deletedSecrets := []string{}
		secrets.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(namespace, name string, opts *metav1.DeleteOptions) error {
			assert.Equal(t, tokenSecretNamespace, namespace)
			deletedSecrets = append(deletedSecrets, name)
			return nil
		}).Times(3)

		err := CleanupSecrets(secrets, provider)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"scim-token-1", "scim-token-2", "scim-token-3"}, deletedSecrets)
	})

	t.Run("list secrets fails", func(t *testing.T) {
		secrets := fake.NewMockControllerInterface[*v1.Secret, *v1.SecretList](ctrl)
		secrets.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(namespace string, opts metav1.ListOptions) (*v1.SecretList, error) {
			assert.Equal(t, tokenSecretNamespace, namespace)
			assert.Equal(t, wantSelector.String(), opts.LabelSelector)

			return nil, fmt.Errorf("failed to list secrets")
		}).Times(1)

		err := CleanupSecrets(secrets, provider)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "scim::Cleanup: failed to list token secrets for provider okta")
		assert.Contains(t, err.Error(), "failed to list secrets")
	})

	t.Run("delete secret fails", func(t *testing.T) {
		secret := v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "scim-token-1",
				Namespace: tokenSecretNamespace,
				Labels: map[string]string{
					secretKindLabel:   scimAuthToken,
					authProviderLabel: provider,
				},
			},
			Data: map[string][]byte{
				"token": []byte("test-token-1"),
			},
		}

		secrets := fake.NewMockControllerInterface[*v1.Secret, *v1.SecretList](ctrl)
		secrets.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(namespace string, opts metav1.ListOptions) (*v1.SecretList, error) {
			assert.Equal(t, tokenSecretNamespace, namespace)
			assert.Equal(t, wantSelector.String(), opts.LabelSelector)

			return &v1.SecretList{Items: []v1.Secret{secret}}, nil
		}).Times(1)
		secrets.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(namespace, name string, opts *metav1.DeleteOptions) error {
			assert.Equal(t, tokenSecretNamespace, namespace)
			assert.Equal(t, "scim-token-1", name)

			return fmt.Errorf("permission denied")
		}).Times(1)

		err := CleanupSecrets(secrets, provider)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "scim::Cleanup: failed to delete token secret scim-token-1 for provider okta")
		assert.Contains(t, err.Error(), "permission denied")
	})

	t.Run("delete secret not found is ignored", func(t *testing.T) {
		secret := v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "scim-token-1",
				Namespace: tokenSecretNamespace,
				Labels: map[string]string{
					secretKindLabel:   scimAuthToken,
					authProviderLabel: provider,
				},
			},
			Data: map[string][]byte{
				"token": []byte("test-token-1"),
			},
		}

		secrets := fake.NewMockControllerInterface[*v1.Secret, *v1.SecretList](ctrl)
		secrets.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(namespace string, opts metav1.ListOptions) (*v1.SecretList, error) {
			assert.Equal(t, tokenSecretNamespace, namespace)
			assert.Equal(t, wantSelector.String(), opts.LabelSelector)

			return &v1.SecretList{Items: []v1.Secret{secret}}, nil
		}).Times(1)
		secrets.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(namespace, name string, opts *metav1.DeleteOptions) error {
			assert.Equal(t, tokenSecretNamespace, namespace)
			assert.Equal(t, "scim-token-1", name)

			// Return NotFound error which should be ignored
			return apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, name)
		}).Times(1)

		err := CleanupSecrets(secrets, provider)
		require.NoError(t, err)
	})

	t.Run("partial deletion failure", func(t *testing.T) {
		secret1 := v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "scim-token-1",
				Namespace: tokenSecretNamespace,
				Labels: map[string]string{
					secretKindLabel:   scimAuthToken,
					authProviderLabel: provider,
				},
			},
			Data: map[string][]byte{
				"token": []byte("test-token-1"),
			},
		}
		secret2 := v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "scim-token-2",
				Namespace: tokenSecretNamespace,
				Labels: map[string]string{
					secretKindLabel:   scimAuthToken,
					authProviderLabel: provider,
				},
			},
			Data: map[string][]byte{
				"token": []byte("test-token-2"),
			},
		}

		secrets := fake.NewMockControllerInterface[*v1.Secret, *v1.SecretList](ctrl)
		secrets.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(namespace string, opts metav1.ListOptions) (*v1.SecretList, error) {
			assert.Equal(t, tokenSecretNamespace, namespace)
			assert.Equal(t, wantSelector.String(), opts.LabelSelector)

			return &v1.SecretList{Items: []v1.Secret{secret1, secret2}}, nil
		}).Times(1)

		// First delete succeeds
		secrets.EXPECT().Delete(gomock.Any(), "scim-token-1", gomock.Any()).Return(nil).Times(1)
		// Second delete fails
		secrets.EXPECT().Delete(gomock.Any(), "scim-token-2", gomock.Any()).Return(fmt.Errorf("server error")).Times(1)

		err := CleanupSecrets(secrets, provider)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "scim::Cleanup: failed to delete token secret scim-token-2 for provider okta")
		assert.Contains(t, err.Error(), "server error")
	})
}
