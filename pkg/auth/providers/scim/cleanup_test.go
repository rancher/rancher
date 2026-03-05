package scim

import (
	"fmt"
	"testing"

	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
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

func TestCleanup(t *testing.T) {
	ctrl := gomock.NewController(t)
	provider := "okta"

	wantSecretSelector := labels.Set{
		secretKindLabel:   scimAuthToken,
		authProviderLabel: provider,
	}.AsSelector()

	wantGroupSelector := labels.Set{
		authProviderLabel: provider,
	}.AsSelector()

	t.Run("no secrets or groups to delete", func(t *testing.T) {
		secrets := fake.NewMockControllerInterface[*v1.Secret, *v1.SecretList](ctrl)
		secrets.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(namespace string, opts metav1.ListOptions) (*v1.SecretList, error) {
			assert.Equal(t, tokenSecretNamespace, namespace)
			assert.Equal(t, wantSecretSelector.String(), opts.LabelSelector)
			return &v1.SecretList{}, nil
		}).Times(1)

		groups := fake.NewMockNonNamespacedClientInterface[*apisv3.Group, *apisv3.GroupList](ctrl)
		groups.EXPECT().List(gomock.Any()).DoAndReturn(func(opts metav1.ListOptions) (*apisv3.GroupList, error) {
			assert.Equal(t, wantGroupSelector.String(), opts.LabelSelector)
			return &apisv3.GroupList{}, nil
		}).Times(1)

		err := Cleanup(secrets, groups, provider)
		require.NoError(t, err)
	})

	t.Run("secrets and groups deleted successfully", func(t *testing.T) {
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
			assert.Equal(t, wantSecretSelector.String(), opts.LabelSelector)

			return &v1.SecretList{Items: []v1.Secret{secret1, secret2, secret3}}, nil
		}).Times(1)

		deletedSecrets := []string{}
		secrets.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(namespace, name string, opts *metav1.DeleteOptions) error {
			assert.Equal(t, tokenSecretNamespace, namespace)
			deletedSecrets = append(deletedSecrets, name)
			return nil
		}).Times(3)

		group1 := apisv3.Group{ObjectMeta: metav1.ObjectMeta{Name: "grp-aaa"}}
		group2 := apisv3.Group{ObjectMeta: metav1.ObjectMeta{Name: "grp-bbb"}}
		group3 := apisv3.Group{ObjectMeta: metav1.ObjectMeta{Name: "grp-ccc"}}

		deletedGroups := []string{}
		groups := fake.NewMockNonNamespacedClientInterface[*apisv3.Group, *apisv3.GroupList](ctrl)
		groups.EXPECT().List(gomock.Any()).Return(&apisv3.GroupList{Items: []apisv3.Group{group1, group2, group3}}, nil)
		groups.EXPECT().Delete(gomock.Any(), gomock.Any()).DoAndReturn(func(name string, opts *metav1.DeleteOptions) error {
			deletedGroups = append(deletedGroups, name)
			return nil
		}).Times(3)

		err := Cleanup(secrets, groups, provider)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"scim-token-1", "scim-token-2", "scim-token-3"}, deletedSecrets)
		assert.ElementsMatch(t, []string{"grp-aaa", "grp-bbb", "grp-ccc"}, deletedGroups)
	})

	t.Run("list secrets fails", func(t *testing.T) {
		secrets := fake.NewMockControllerInterface[*v1.Secret, *v1.SecretList](ctrl)
		secrets.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(namespace string, opts metav1.ListOptions) (*v1.SecretList, error) {
			assert.Equal(t, tokenSecretNamespace, namespace)
			assert.Equal(t, wantSecretSelector.String(), opts.LabelSelector)

			return nil, fmt.Errorf("failed to list secrets")
		}).Times(1)

		err := Cleanup(secrets, nil, provider)
		require.Error(t, err)
		assert.ErrorContains(t, err, "scim::Cleanup: failed to list token secrets for provider okta")
		assert.ErrorContains(t, err, "failed to list secrets")
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
			assert.Equal(t, wantSecretSelector.String(), opts.LabelSelector)

			return &v1.SecretList{Items: []v1.Secret{secret}}, nil
		}).Times(1)
		secrets.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(namespace, name string, opts *metav1.DeleteOptions) error {
			assert.Equal(t, tokenSecretNamespace, namespace)
			assert.Equal(t, "scim-token-1", name)

			return fmt.Errorf("some error")
		}).Times(1)

		err := Cleanup(secrets, nil, provider)
		require.Error(t, err)
		assert.ErrorContains(t, err, "scim::Cleanup: failed to delete token secret scim-token-1 for provider okta")
		assert.ErrorContains(t, err, "some error")
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
			assert.Equal(t, wantSecretSelector.String(), opts.LabelSelector)

			return &v1.SecretList{Items: []v1.Secret{secret}}, nil
		}).Times(1)
		secrets.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(namespace, name string, opts *metav1.DeleteOptions) error {
			assert.Equal(t, tokenSecretNamespace, namespace)
			assert.Equal(t, "scim-token-1", name)

			// Return NotFound error which should be ignored
			return apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, name)
		}).Times(1)

		groups := fake.NewMockNonNamespacedClientInterface[*apisv3.Group, *apisv3.GroupList](ctrl)
		groups.EXPECT().List(gomock.Any()).Return(&apisv3.GroupList{}, nil)

		err := Cleanup(secrets, groups, provider)
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
			assert.Equal(t, wantSecretSelector.String(), opts.LabelSelector)

			return &v1.SecretList{Items: []v1.Secret{secret1, secret2}}, nil
		}).Times(1)

		// First delete succeeds
		secrets.EXPECT().Delete(gomock.Any(), "scim-token-1", gomock.Any()).Return(nil).Times(1)
		// Second delete fails
		secrets.EXPECT().Delete(gomock.Any(), "scim-token-2", gomock.Any()).Return(fmt.Errorf("server error")).Times(1)

		err := Cleanup(secrets, nil, provider)
		require.Error(t, err)
		assert.ErrorContains(t, err, "scim::Cleanup: failed to delete token secret scim-token-2 for provider okta")
		assert.ErrorContains(t, err, "server error")
	})

	t.Run("list groups fails", func(t *testing.T) {
		secrets := fake.NewMockControllerInterface[*v1.Secret, *v1.SecretList](ctrl)
		secrets.EXPECT().List(gomock.Any(), gomock.Any()).Return(&v1.SecretList{}, nil)

		groups := fake.NewMockNonNamespacedClientInterface[*apisv3.Group, *apisv3.GroupList](ctrl)
		groups.EXPECT().List(gomock.Any()).Return(nil, fmt.Errorf("failed to list groups"))

		err := Cleanup(secrets, groups, provider)
		require.Error(t, err)
		assert.ErrorContains(t, err, "scim::Cleanup: failed to list groups for provider okta")
		assert.ErrorContains(t, err, "failed to list groups")
	})

	t.Run("delete group fails", func(t *testing.T) {
		group := apisv3.Group{ObjectMeta: metav1.ObjectMeta{Name: "grp-aaa"}}

		secrets := fake.NewMockControllerInterface[*v1.Secret, *v1.SecretList](ctrl)
		secrets.EXPECT().List(gomock.Any(), gomock.Any()).Return(&v1.SecretList{}, nil)

		groups := fake.NewMockNonNamespacedClientInterface[*apisv3.Group, *apisv3.GroupList](ctrl)
		groups.EXPECT().List(gomock.Any()).Return(&apisv3.GroupList{Items: []apisv3.Group{group}}, nil)
		groups.EXPECT().Delete("grp-aaa", gomock.Any()).Return(fmt.Errorf("some error"))

		err := Cleanup(secrets, groups, provider)
		require.Error(t, err)
		assert.ErrorContains(t, err, "scim::Cleanup: failed to delete group grp-aaa for provider okta")
		assert.ErrorContains(t, err, "some error")
	})

	t.Run("delete group not found is ignored", func(t *testing.T) {
		group := apisv3.Group{ObjectMeta: metav1.ObjectMeta{Name: "grp-aaa"}}

		secrets := fake.NewMockControllerInterface[*v1.Secret, *v1.SecretList](ctrl)
		secrets.EXPECT().List(gomock.Any(), gomock.Any()).Return(&v1.SecretList{}, nil)

		groups := fake.NewMockNonNamespacedClientInterface[*apisv3.Group, *apisv3.GroupList](ctrl)
		groups.EXPECT().List(gomock.Any()).Return(&apisv3.GroupList{Items: []apisv3.Group{group}}, nil)
		groups.EXPECT().Delete("grp-aaa", gomock.Any()).Return(
			apierrors.NewNotFound(schema.GroupResource{Resource: "groups"}, "grp-aaa"),
		)

		err := Cleanup(secrets, groups, provider)
		require.NoError(t, err)
	})
}
