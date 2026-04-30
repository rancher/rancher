package cloudcredential

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func TestStoreUpdate(t *testing.T) {
	t.Parallel()

	t.Run("requires admin permissions", func(t *testing.T) {
		h := newStoreHarness(t, adminOnlyAuthorizer())
		secret := *secretForCredential(newCredential(testCredName))
		h.expectSecretListForName(testCredName, secret)
		objInfo := &fakeUpdatedObjectInfo{obj: newCredential(testCredName)}
		_, _, err := h.store.Update(ctxWithUser(regularUser), testCredName, objInfo, nil, nil, false, nil)
		require.Error(t, err)
		assert.True(t, apierrors.IsForbidden(err))
	})

	t.Run("updated object must be a cloudcredential", func(t *testing.T) {
		h := newStoreHarness(t, adminOnlyAuthorizer())
		secret := *secretForCredential(newCredential(testCredName))
		h.expectSecretListForName(testCredName, secret)

		objInfo := &fakeUpdatedObjectInfo{obj: &metav1.Status{}}
		_, _, err := h.store.Update(ctxWithUser(adminUser), testCredName, objInfo, nil, nil, false, nil)
		require.Error(t, err)
		assert.True(t, apierrors.IsBadRequest(err))
	})

	t.Run("update validation errors are surfaced", func(t *testing.T) {
		h := newStoreHarness(t, adminOnlyAuthorizer())
		secret := *secretForCredential(newCredential(testCredName))
		h.expectSecretListForName(testCredName, secret)

		objInfo := &fakeUpdatedObjectInfo{obj: newCredential(testCredName)}
		validateErr := fmt.Errorf("invalid update")
		_, _, err := h.store.Update(ctxWithUser(adminUser), testCredName, objInfo, nil, func(context.Context, runtime.Object, runtime.Object) error {
			return validateErr
		}, false, nil)
		require.Error(t, err)
		assert.True(t, apierrors.IsBadRequest(err))
		assert.Contains(t, err.Error(), validateErr.Error())
	})

	t.Run("updates secret and returns new credential", func(t *testing.T) {
		h := newStoreHarness(t, adminOnlyAuthorizer())
		secret := secretForCredential(newCredential(testCredName))
		h.expectSecretListForName(testCredName, *secret)

		updated, err := fromSecret(secret.DeepCopy(), nil)
		require.NoError(t, err)
		updated.Spec.Description = "updated description"

		h.secretClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
			assert.Equal(t, secret.Name, s.Name)
			assert.Equal(t, adminUser, s.Annotations[CreatorIDAnnotation])
			s.ResourceVersion = "2"
			return s, nil
		})

		objInfo := &fakeUpdatedObjectInfo{obj: updated}
		result, _, err := h.store.Update(ctxWithUser(adminUser), testCredName, objInfo, nil, nil, false, &metav1.UpdateOptions{})
		require.NoError(t, err)
		require.IsType(t, &ext.CloudCredential{}, result)
		cred := result.(*ext.CloudCredential)
		assert.Equal(t, "2", cred.ResourceVersion)
		assert.Equal(t, "updated description", cred.Spec.Description)
	})
}

func TestSystemStoreUpdate(t *testing.T) {
	t.Parallel()

	baseCredential := newCredential(testCredName)
	baseSecret := secretForCredential(baseCredential)
	oldCredential, err := fromSecret(baseSecret.DeepCopy(), nil)
	require.NoError(t, err)

	t.Run("rejects UID, type, and name mutations", func(t *testing.T) {
		h := newSystemStoreHarness(t)

		tests := []struct {
			name        string
			mutate      func(*ext.CloudCredential)
			expectedErr string
		}{
			{
				name:        "uid change",
				mutate:      func(c *ext.CloudCredential) { c.UID = typesUID("new") },
				expectedErr: "meta.UID is immutable",
			},
			{
				name:        "type change",
				mutate:      func(c *ext.CloudCredential) { c.Spec.Type = "azure" },
				expectedErr: "spec.type is immutable",
			},
			{
				name:        "name change",
				mutate:      func(c *ext.CloudCredential) { c.Name = "new-name" },
				expectedErr: "metadata.name is immutable",
			},
		}

		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				updated := oldCredential.DeepCopy()
				tt.mutate(updated)
				_, err := h.store.SystemStore.Update(baseSecret.DeepCopy(), oldCredential, updated, nil, adminUser)
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			})
		}
	})

	t.Run("dry-run omits credentials", func(t *testing.T) {
		h := newSystemStoreHarness(t)
		update := oldCredential.DeepCopy()
		update.Spec.Credentials = map[string]string{"foo": "bar"}

		result, err := h.store.SystemStore.Update(baseSecret.DeepCopy(), oldCredential, update, &metav1.UpdateOptions{
			DryRun: []string{metav1.DryRunAll},
		}, adminUser)
		require.NoError(t, err)
		assert.Nil(t, result.Spec.Credentials)
	})

	t.Run("fails when toSecret conversion errors", func(t *testing.T) {
		h := newSystemStoreHarness(t)
		update := oldCredential.DeepCopy()
		update.ManagedFields = []metav1.ManagedFieldsEntry{
			{
				FieldsV1: &metav1.FieldsV1{Raw: []byte("{invalid")},
			},
		}

		_, err := h.store.SystemStore.Update(baseSecret.DeepCopy(), oldCredential, update, nil, adminUser)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to convert credential for storage")
	})

	t.Run("propagates secret client errors", func(t *testing.T) {
		h := newSystemStoreHarness(t)
		update := oldCredential.DeepCopy()
		h.secretClient.EXPECT().Update(gomock.Any()).Return(nil, fmt.Errorf(genericErr))

		_, err := h.store.SystemStore.Update(baseSecret.DeepCopy(), oldCredential, update, nil, adminUser)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to save updated cloud credential")
	})

	t.Run("fails when updated secret cannot be converted back", func(t *testing.T) {
		h := newSystemStoreHarness(t)
		update := oldCredential.DeepCopy()
		brokenSecret := baseSecret.DeepCopy()
		brokenSecret.ManagedFields = []metav1.ManagedFieldsEntry{
			{FieldsV1: &metav1.FieldsV1{Raw: []byte("{")}},
		}
		h.secretClient.EXPECT().Update(gomock.Any()).Return(brokenSecret, nil)

		_, err := h.store.SystemStore.Update(baseSecret.DeepCopy(), oldCredential, update, nil, adminUser)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to regenerate cloud credential")
	})

	t.Run("successfully updates secret", func(t *testing.T) {
		h := newSystemStoreHarness(t)
		update := oldCredential.DeepCopy()
		update.Spec.Description = "new description"

		h.secretClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
			assert.Equal(t, baseSecret.Name, secret.Name)
			secret.ResourceVersion = "5"
			return secret, nil
		})

		result, err := h.store.SystemStore.Update(baseSecret.DeepCopy(), oldCredential, update, nil, adminUser)
		require.NoError(t, err)
		assert.Equal(t, "5", result.ResourceVersion)
		assert.Equal(t, "new description", result.Spec.Description)
	})
}

func TestUpdateWithVisibleFields(t *testing.T) {
	t.Parallel()

	t.Run("update adds visibleFields and returns publicData", func(t *testing.T) {
		h := newStoreHarness(t, adminOnlyAuthorizer())
		secret := secretForCredential(newCredential(testCredName))
		h.expectSecretListForName(testCredName, *secret)

		updated, err := fromSecret(secret.DeepCopy(), nil)
		require.NoError(t, err)
		updated.Spec.VisibleFields = []string{"accessKey"}
		updated.Spec.Credentials = map[string]string{
			"accessKey": "new-key",
			"secretKey": "new-secret",
		}

		h.secretClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
			require.Contains(t, s.Data, FieldVisibleFields)
			s.ResourceVersion = "2"
			return s, nil
		})

		objInfo := &fakeUpdatedObjectInfo{obj: updated}
		result, _, err := h.store.Update(ctxWithUser(adminUser), testCredName, objInfo, nil, nil, false, &metav1.UpdateOptions{})
		require.NoError(t, err)
		cred := result.(*ext.CloudCredential)
		// VisibleFields is write-only — not returned in spec
		assert.Empty(t, cred.Spec.VisibleFields)
		// PublicData should be populated from the VisibleFields stored in the secret
		require.NotNil(t, cred.Status.PublicData)
		assert.Equal(t, "new-key", cred.Status.PublicData["accessKey"])
		assert.NotContains(t, cred.Status.PublicData, "secretKey")
	})
}

func TestUpdateStatus(t *testing.T) {
	t.Parallel()

	t.Run("updates with conditions", func(t *testing.T) {
		t.Parallel()
		h := newSystemStoreHarness(t)

		secret := secretForCredential(newCredential(testCredName))
		h.expectSecretListForNameAndNamespace(testCredName, "ns-default", *secret)

		h.secretClient.EXPECT().
			Patch(CredentialNamespace, secret.Name, types.JSONPatchType, gomock.Any()).
			DoAndReturn(func(ns, name string, pt types.PatchType, data []byte, subresources ...string) (*corev1.Secret, error) {
				var patches []map[string]any
				require.NoError(t, json.Unmarshal(data, &patches))
				assert.Len(t, patches, 1)
				assert.Equal(t, "replace", patches[0]["op"])
				assert.Equal(t, "/data/"+FieldConditions, patches[0]["path"])
				return secret, nil
			})

		err := h.store.SystemStore.UpdateStatus("ns-default", testCredName, ext.CloudCredentialStatus{
			Conditions: []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Valid"},
			},
		})
		require.NoError(t, err)
	})

	t.Run("no-op when no conditions", func(t *testing.T) {
		t.Parallel()
		h := newSystemStoreHarness(t)

		secret := secretForCredential(newCredential(testCredName))
		h.expectSecretListForNameAndNamespace(testCredName, "ns-default", *secret)

		// No Patch call expected since there's nothing to update
		err := h.store.SystemStore.UpdateStatus("ns-default", testCredName, ext.CloudCredentialStatus{})
		require.NoError(t, err)
	})

	t.Run("secret not found returns error", func(t *testing.T) {
		t.Parallel()
		h := newSystemStoreHarness(t)

		h.expectSecretListForNameAndNamespace(testCredName, "ns-default")

		err := h.store.SystemStore.UpdateStatus("ns-default", testCredName, ext.CloudCredentialStatus{})
		require.Error(t, err)
	})

	t.Run("patch error propagated", func(t *testing.T) {
		t.Parallel()
		h := newSystemStoreHarness(t)

		secret := secretForCredential(newCredential(testCredName))
		h.expectSecretListForNameAndNamespace(testCredName, "ns-default", *secret)

		h.secretClient.EXPECT().
			Patch(CredentialNamespace, secret.Name, types.JSONPatchType, gomock.Any()).
			Return(nil, fmt.Errorf("patch failed"))

		err := h.store.SystemStore.UpdateStatus("ns-default", testCredName, ext.CloudCredentialStatus{
			Conditions: []metav1.Condition{
				{Type: "Valid", Status: metav1.ConditionTrue, Reason: "Validated"},
			},
		})
		require.Error(t, err)
	})

	t.Run("uses namespace to select matching secret", func(t *testing.T) {
		t.Parallel()
		h := newSystemStoreHarness(t)

		first := *secretForCredential(newCredential(testCredName))
		first.Labels[CloudCredentialNamespaceLabel] = "ns-a"
		second := *secretForCredential(newCredential(testCredName))
		second.Name = "second-secret"
		second.Labels[CloudCredentialNamespaceLabel] = "ns-b"
		h.expectSecretListForNameAndNamespace(testCredName, "ns-b", first, second)

		h.secretClient.EXPECT().
			Patch(CredentialNamespace, second.Name, types.JSONPatchType, gomock.Any()).
			Return(&second, nil)

		err := h.store.SystemStore.UpdateStatus("ns-b", testCredName, ext.CloudCredentialStatus{
			Conditions: []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Valid"},
			},
		})
		require.NoError(t, err)
	})

	t.Run("missing namespace returns not found", func(t *testing.T) {
		t.Parallel()
		h := newSystemStoreHarness(t)

		err := h.store.SystemStore.UpdateStatus("", testCredName, ext.CloudCredentialStatus{
			Conditions: []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Valid"},
			},
		})
		require.Error(t, err)
		assert.True(t, apierrors.IsNotFound(errors.Unwrap(err)))
	})
}

// ============================================================================
// Watch tests
// ============================================================================

func TestRBACUpdate(t *testing.T) {
	t.Parallel()

	setupForUpdate := func(h *storeTestHarness, owner string) *ext.CloudCredential {
		secret := secretForCredential(newCredential(testCredName))
		if owner != "" {
			setSecretOwner(secret, owner)
		}
		h.expectSecretListForName(testCredName, *secret)
		updated, err := fromSecret(secret.DeepCopy(), nil)
		require.NoError(t, err)
		updated.Spec.Description = "updated"
		return updated
	}

	t.Run("admin can update", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		updated := setupForUpdate(h, "")

		h.secretClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
			s.ResourceVersion = "2"
			return s, nil
		})

		objInfo := &fakeUpdatedObjectInfo{obj: updated}
		result, _, err := h.store.Update(ctxWithUser(adminUser), testCredName, objInfo, nil, nil, false, &metav1.UpdateOptions{})
		require.NoError(t, err)
		cred := result.(*ext.CloudCredential)
		assert.Equal(t, "updated", cred.Spec.Description)
	})

	t.Run("read-only user cannot update", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		updated := setupForUpdate(h, "")
		objInfo := &fakeUpdatedObjectInfo{obj: updated}

		_, _, err := h.store.Update(ctxWithUser(readOnlyUser), testCredName, objInfo, nil, nil, false, nil)
		require.Error(t, err)
		assert.True(t, apierrors.IsForbidden(err))
	})

	t.Run("full-access non-admin can update", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		updated := setupForUpdate(h, fullAccessUser)

		h.secretClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
			s.ResourceVersion = "2"
			return s, nil
		})

		objInfo := &fakeUpdatedObjectInfo{obj: updated}
		result, _, err := h.store.Update(ctxWithUser(fullAccessUser), testCredName, objInfo, nil, nil, false, &metav1.UpdateOptions{})
		require.NoError(t, err)
		cred := result.(*ext.CloudCredential)
		assert.Equal(t, "updated", cred.Spec.Description)
	})

	t.Run("no-access user cannot update", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		updated := setupForUpdate(h, "")
		objInfo := &fakeUpdatedObjectInfo{obj: updated}

		_, _, err := h.store.Update(ctxWithUser(noAccessUser), testCredName, objInfo, nil, nil, false, nil)
		require.Error(t, err)
		assert.True(t, apierrors.IsForbidden(err))
	})

	t.Run("missing user context errors", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		objInfo := &fakeUpdatedObjectInfo{obj: newCredential(testCredName)}

		_, _, err := h.store.Update(context.Background(), testCredName, objInfo, nil, nil, false, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no user info")
	})

	t.Run("authorizer error is propagated", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, errorAuthorizer())
		objInfo := &fakeUpdatedObjectInfo{obj: newCredential(testCredName)}

		_, _, err := h.store.Update(ctxWithUser(adminUser), testCredName, objInfo, nil, nil, false, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "authorizer error")
	})
}
