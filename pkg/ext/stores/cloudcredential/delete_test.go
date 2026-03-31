package cloudcredential

import (
	"context"
	"fmt"
	"testing"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestStoreDelete(t *testing.T) {
	t.Parallel()

	t.Run("requires admin permissions", func(t *testing.T) {
		h := newStoreHarness(t, adminOnlyAuthorizer())
		secret := *secretForCredential(newCredential(testCredName))
		h.expectSecretListForName(testCredName, secret)
		_, _, err := h.store.Delete(ctxWithUser(regularUser), testCredName, nil, nil)
		require.Error(t, err)
		assert.True(t, apierrors.IsForbidden(err))
	})

	t.Run("delete validation errors are returned", func(t *testing.T) {
		h := newStoreHarness(t, adminOnlyAuthorizer())
		secret := *secretForCredential(newCredential(testCredName))
		h.expectSecretListForName(testCredName, secret)

		validateErr := fmt.Errorf("delete denied")
		_, _, err := h.store.Delete(ctxWithUser(adminUser), testCredName, func(context.Context, runtime.Object) error {
			return validateErr
		}, nil)
		require.ErrorIs(t, err, validateErr)
	})

	t.Run("rewrites uid precondition and deletes backing secret", func(t *testing.T) {
		h := newStoreHarness(t, adminOnlyAuthorizer())
		cred := newCredential(testCredName)
		secret := *secretForCredential(cred)
		h.expectSecretListForName(testCredName, secret)
		h.secretClient.EXPECT().Delete(CredentialNamespace, secret.Name, gomock.Any()).Return(nil)

		preconditionUID := secret.UID
		opts := &metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &preconditionUID,
			},
		}

		obj, completed, err := h.store.Delete(ctxWithUser(adminUser), testCredName, nil, opts)
		require.NoError(t, err)
		assert.True(t, completed)
		require.NotNil(t, obj)
		result := obj.(*ext.CloudCredential)
		assert.Equal(t, cred.Name, result.Name)
		require.NotNil(t, opts.Preconditions)
		assert.Equal(t, secret.UID, *opts.Preconditions.UID)
	})
}

func TestSystemStoreDelete(t *testing.T) {
	t.Parallel()

	t.Run("ignores not found errors", func(t *testing.T) {
		h := newSystemStoreHarness(t)
		h.secretClient.EXPECT().Delete(CredentialNamespace, "missing", gomock.Any()).
			Return(apierrors.NewNotFound(GVR.GroupResource(), "missing"))
		err := h.store.SystemStore.Delete("missing", &metav1.DeleteOptions{})
		require.NoError(t, err)
	})

	t.Run("wraps delete errors", func(t *testing.T) {
		h := newSystemStoreHarness(t)
		h.secretClient.EXPECT().Delete(CredentialNamespace, "name", gomock.Any()).
			Return(fmt.Errorf(genericErr))
		err := h.store.SystemStore.Delete("name", &metav1.DeleteOptions{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete cloud credential")
	})

	t.Run("returns nil on success", func(t *testing.T) {
		h := newSystemStoreHarness(t)
		h.secretClient.EXPECT().Delete(CredentialNamespace, "name", gomock.Any()).Return(nil)
		require.NoError(t, h.store.SystemStore.Delete("name", &metav1.DeleteOptions{}))
	})
}

func TestDeleteCollection(t *testing.T) {
	t.Parallel()

	t.Run("non-admin delete collection uses owner filtering", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		ctx := ctxWithUser(readOnlyUser)

		h.secretClient.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			DoAndReturn(func(ns string, opts metav1.ListOptions) (*corev1.SecretList, error) {
				assert.Contains(t, opts.LabelSelector, LabelCloudCredentialOwner+"="+readOnlyUser)
				return &corev1.SecretList{Items: []corev1.Secret{}}, nil
			})

		result, err := h.store.DeleteCollection(ctx, nil, nil, &metainternalversion.ListOptions{})
		require.NoError(t, err)
		credList := result.(*ext.CloudCredentialList)
		assert.Empty(t, credList.Items)
	})

	t.Run("admin deletes multiple credentials", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		ctx := ctxWithUser(adminUser)

		secret1 := *secretForCredential(newCredential("cred1"))
		secret2 := *secretForCredential(newCredential("cred2"))

		h.secretClient.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			Return(&corev1.SecretList{Items: []corev1.Secret{secret1, secret2}}, nil)

		h.secretClient.EXPECT().Delete(CredentialNamespace, secret1.Name, gomock.Any()).Return(nil)
		h.secretClient.EXPECT().Delete(CredentialNamespace, secret2.Name, gomock.Any()).Return(nil)

		result, err := h.store.DeleteCollection(ctx, nil, nil, &metainternalversion.ListOptions{})
		require.NoError(t, err)
		credList := result.(*ext.CloudCredentialList)
		assert.Len(t, credList.Items, 2)
	})

	t.Run("empty list returns empty result", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		ctx := ctxWithUser(adminUser)

		h.secretClient.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			Return(&corev1.SecretList{Items: []corev1.Secret{}}, nil)

		result, err := h.store.DeleteCollection(ctx, nil, nil, &metainternalversion.ListOptions{})
		require.NoError(t, err)
		credList := result.(*ext.CloudCredentialList)
		assert.Empty(t, credList.Items)
	})

	t.Run("validation callback rejects", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		ctx := ctxWithUser(adminUser)

		secret := *secretForCredential(newCredential("cred1"))
		h.secretClient.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			Return(&corev1.SecretList{Items: []corev1.Secret{secret}}, nil)

		validator := func(_ context.Context, _ runtime.Object) error {
			return apierrors.NewForbidden(GVR.GroupResource(), "cred1", fmt.Errorf("rejected"))
		}

		_, err := h.store.DeleteCollection(ctx, validator, nil, &metainternalversion.ListOptions{})
		require.Error(t, err)
		assert.True(t, apierrors.IsForbidden(err))
	})
}

// ============================================================================
// UpdateStatus tests
// ============================================================================

func TestDeletePermissions(t *testing.T) {
	t.Parallel()

	t.Run("non-admin cannot delete other users credential", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		ctx := ctxWithUser(readOnlyUser)

		// Secret owned by admin
		secret := *secretForCredential(newCredential(testCredName))
		h.expectSecretListForName(testCredName, secret)

		_, _, err := h.store.Delete(ctx, testCredName, nil, nil)
		require.Error(t, err)
		assert.True(t, apierrors.IsForbidden(err))
	})
}

// ============================================================================
// SystemStore.list error path tests
// ============================================================================

func TestRBACDelete(t *testing.T) {
	t.Parallel()

	t.Run("admin can delete", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		secret := *secretForCredential(newCredential(testCredName))
		h.expectSecretListForName(testCredName, secret)
		h.secretClient.EXPECT().Delete(CredentialNamespace, secret.Name, gomock.Any()).Return(nil)

		obj, completed, err := h.store.Delete(ctxWithUser(adminUser), testCredName, nil, nil)
		require.NoError(t, err)
		assert.True(t, completed)
		assert.Equal(t, testCredName, obj.(*ext.CloudCredential).Name)
	})

	t.Run("read-only user cannot delete", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		secret := *secretForCredential(newCredential(testCredName))
		h.expectSecretListForName(testCredName, secret)

		_, _, err := h.store.Delete(ctxWithUser(readOnlyUser), testCredName, nil, nil)
		require.Error(t, err)
		assert.True(t, apierrors.IsForbidden(err))
	})

	t.Run("full-access non-admin can delete", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		secret := *secretForCredential(newCredential(testCredName))
		setSecretOwner(&secret, fullAccessUser)
		h.expectSecretListForName(testCredName, secret)
		h.secretClient.EXPECT().Delete(CredentialNamespace, secret.Name, gomock.Any()).Return(nil)

		obj, completed, err := h.store.Delete(ctxWithUser(fullAccessUser), testCredName, nil, nil)
		require.NoError(t, err)
		assert.True(t, completed)
		assert.Equal(t, testCredName, obj.(*ext.CloudCredential).Name)
	})

	t.Run("no-access user cannot delete", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		secret := *secretForCredential(newCredential(testCredName))
		h.expectSecretListForName(testCredName, secret)

		_, _, err := h.store.Delete(ctxWithUser(noAccessUser), testCredName, nil, nil)
		require.Error(t, err)
		assert.True(t, apierrors.IsForbidden(err))
	})

	t.Run("missing user context errors", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())

		_, _, err := h.store.Delete(context.Background(), testCredName, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no user info")
	})

	t.Run("authorizer error is propagated", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, errorAuthorizer())

		_, _, err := h.store.Delete(ctxWithUser(adminUser), testCredName, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "authorizer error")
	})
}
