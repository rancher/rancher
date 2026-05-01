package cloudcredential

import (
	"context"
	"fmt"
	"strings"
	"testing"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestStoreCreate(t *testing.T) {
	t.Parallel()

	t.Run("rejects non cloudcredential objects", func(t *testing.T) {
		h := newStoreHarness(t, adminOnlyAuthorizer())
		ctx := ctxWithUser(adminUser)

		_, err := h.store.Create(ctx, &metav1.Status{}, nil, nil)
		require.Error(t, err)
		assert.True(t, apierrors.IsInternalError(err))
	})

	t.Run("fails create validation", func(t *testing.T) {
		h := newStoreHarness(t, adminOnlyAuthorizer())
		ctx := ctxWithUser(adminUser)
		credential := newCredential(testCredName)
		validateErr := fmt.Errorf("validation failed")

		_, err := h.store.Create(ctx, credential, func(context.Context, runtime.Object) error {
			return validateErr
		}, nil)
		require.ErrorIs(t, err, validateErr)
	})

	t.Run("uses requesting user as owner", func(t *testing.T) {
		h := newStoreHarness(t, adminOnlyAuthorizer())
		ctx := ctxWithUser(regularUser)
		credential := newCredential(testCredName)

		h.secretCache.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			Return(([]*corev1.Secret)(nil), nil)
		h.expectNamespaceExists()
		h.secretClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
			require.Equal(t, regularUser, secret.Annotations[CreatorIDAnnotation])
			require.Equal(t, sanitizeLabelValue(regularUser), secret.Labels[CloudCredentialOwnerLabel])
			secret.Name = "secret-created"
			secret.Labels[CloudCredentialNameLabel] = credential.Name
			return secret, nil
		})

		created, err := h.store.Create(ctx, credential, nil, nil)
		require.NoError(t, err)
		require.IsType(t, &ext.CloudCredential{}, created)
	})

	t.Run("creates backing secret with creatorId annotation", func(t *testing.T) {
		h := newStoreHarness(t, adminOnlyAuthorizer())
		ctx := ctxWithUser(adminUser)
		credential := newCredential(testCredName)

		h.secretCache.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			Return(([]*corev1.Secret)(nil), nil)
		h.expectNamespaceExists()

		h.secretClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
			require.Equal(t, adminUser, secret.Annotations[CreatorIDAnnotation])
			require.Equal(t, CredentialNamespace, secret.Namespace)
			require.Equal(t, SecretTypePrefix+credential.Spec.Type, string(secret.Type))
			secret.Name = "secret-created"
			secret.Labels[CloudCredentialNameLabel] = credential.Name
			return secret, nil
		})

		created, err := h.store.Create(ctx, credential, nil, nil)
		require.NoError(t, err)
		require.IsType(t, &ext.CloudCredential{}, created)
		result := created.(*ext.CloudCredential)
		assert.Equal(t, credential.Name, result.Name)
		assert.NotNil(t, result.Status.Secret)
		assert.Equal(t, "secret-created", result.Status.Secret.Name)
	})
}

func TestSystemStoreCreateValidations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		credential  *ext.CloudCredential
		expectedErr string
	}{
		{
			name: "missing type",
			credential: func() *ext.CloudCredential {
				c := newCredential(testCredName)
				c.Spec.Type = ""
				return c
			}(),
			expectedErr: "spec.type is required",
		},
		{
			name: "missing name",
			credential: func() *ext.CloudCredential {
				c := newCredential("")
				return c
			}(),
			expectedErr: "metadata.name is required",
		},
		{
			name: "invalid managed fields",
			credential: func() *ext.CloudCredential {
				c := newCredential(testCredName)
				c.ManagedFields = []metav1.ManagedFieldsEntry{
					{
						FieldsV1: &metav1.FieldsV1{Raw: []byte("{invalid")},
					},
				}
				return c
			}(),
			expectedErr: "failed to map credential managed-fields",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newSystemStoreHarness(t)
			if tt.name == "invalid managed fields" {
				h.secretCache.EXPECT().
					List(CredentialNamespace, gomock.Any()).
					Return(([]*corev1.Secret)(nil), nil)
			}
			_, err := h.store.SystemStore.Create(context.Background(), tt.credential, nil, adminUser)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestSystemStoreCreateDryRun(t *testing.T) {
	t.Parallel()

	h := newSystemStoreHarness(t)
	credential := newCredential(testCredName)

	result, err := h.store.SystemStore.Create(context.Background(), credential, &metav1.CreateOptions{
		DryRun: []string{metav1.DryRunAll},
	}, adminUser)
	require.NoError(t, err)
	assert.Nil(t, result.Spec.Credentials)
	require.NotNil(t, result.Status.Secret)
	assert.Equal(t, CredentialNamespace, result.Status.Secret.Namespace)
	assert.True(t, strings.HasPrefix(result.Status.Secret.Name, GeneratePrefix+credential.Name))
}

func TestSystemStoreCreateNamespaceAndSecretFailures(t *testing.T) {
	t.Parallel()

	t.Run("returns already exists when credential secret already exists", func(t *testing.T) {
		h := newSystemStoreHarness(t)
		credential := newCredential(testCredName)

		h.secretCache.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			Return(secretPointers(corev1.Secret{}), nil)

		created, err := h.store.SystemStore.Create(context.Background(), credential, nil, adminUser)
		require.Error(t, err)
		assert.Nil(t, created)
		assert.True(t, apierrors.IsAlreadyExists(err))
	})

	t.Run("fails when namespace ensure errors", func(t *testing.T) {
		h := newSystemStoreHarness(t)
		credential := newCredential(testCredName)
		h.secretCache.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			Return(([]*corev1.Secret)(nil), nil)
		h.namespaceCache.EXPECT().Get(CredentialNamespace).Return(nil, fmt.Errorf(genericErr))

		_, err := h.store.SystemStore.Create(context.Background(), credential, nil, adminUser)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error ensuring namespace")
	})

	t.Run("propagates secret client error", func(t *testing.T) {
		h := newSystemStoreHarness(t)
		credential := newCredential(testCredName)
		h.secretCache.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			Return(([]*corev1.Secret)(nil), nil)
		h.expectNamespaceExists()
		h.secretClient.EXPECT().Create(gomock.Any()).Return(nil, fmt.Errorf(genericErr))

		_, err := h.store.SystemStore.Create(context.Background(), credential, nil, adminUser)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to store cloud credential")
	})
}

func TestSystemStoreCreateCleansUpOnFromSecretFailure(t *testing.T) {
	t.Parallel()

	h := newSystemStoreHarness(t)
	credential := newCredential(testCredName)
	h.secretCache.EXPECT().
		List(CredentialNamespace, gomock.Any()).
		Return(([]*corev1.Secret)(nil), nil)
	h.expectNamespaceExists()

	createdSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret-created",
			Namespace: CredentialNamespace,
			Labels: map[string]string{
				CloudCredentialNameLabel: credential.Name,
			},
			ManagedFields: []metav1.ManagedFieldsEntry{
				{
					FieldsV1: &metav1.FieldsV1{Raw: []byte("{")},
				},
			},
		},
		Type: corev1.SecretType(SecretTypePrefix + credential.Spec.Type),
	}

	h.secretClient.EXPECT().Create(gomock.Any()).Return(createdSecret, nil)
	h.secretClient.EXPECT().Delete(CredentialNamespace, createdSecret.Name, gomock.Any()).Return(nil)

	_, err := h.store.SystemStore.Create(context.Background(), credential, nil, adminUser)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to regenerate cloud credential")
}

func TestCreateWithVisibleFields(t *testing.T) {
	t.Parallel()

	t.Run("creates credential with visibleFields and returns publicData", func(t *testing.T) {
		h := newStoreHarness(t, adminOnlyAuthorizer())
		ctx := ctxWithUser(adminUser)
		credential := newCredential(testCredName)
		credential.Spec.VisibleFields = []string{"accessKey"}

		h.secretCache.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			Return(([]*corev1.Secret)(nil), nil)
		h.expectNamespaceExists()

		h.secretClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
			require.Contains(t, secret.Data, FieldVisibleFields)
			require.Contains(t, secret.Data, "accessKey")
			secret.Name = "secret-created"
			secret.Labels[CloudCredentialNameLabel] = credential.Name
			return secret, nil
		})

		created, err := h.store.Create(ctx, credential, nil, nil)
		require.NoError(t, err)
		result := created.(*ext.CloudCredential)
		// VisibleFields is write-only — not returned in spec
		assert.Empty(t, result.Spec.VisibleFields)
		// PublicData should be populated from the VisibleFields stored in the secret
		require.NotNil(t, result.Status.PublicData)
		assert.Equal(t, "key", result.Status.PublicData["accessKey"])
		assert.NotContains(t, result.Status.PublicData, "secretKey")
	})
}
