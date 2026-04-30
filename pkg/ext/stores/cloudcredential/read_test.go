package cloudcredential

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/endpoints/request"
)

func TestSystemStoreGetSecret(t *testing.T) {
	t.Parallel()

	t.Run("returns matching secret from cache list", func(t *testing.T) {
		h := newSystemStoreHarness(t)
		secret := *secretForCredential(newCredential(testCredName))
		h.expectSecretListForName(testCredName, secret)

		result, err := h.store.SystemStore.GetSecret(testCredName, "")
		require.NoError(t, err)
		assert.Equal(t, secret.Name, result.Name)
	})

	t.Run("returns internal error on cache list failure", func(t *testing.T) {
		h := newSystemStoreHarness(t)
		h.secretCache.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			Return(nil, fmt.Errorf(genericErr))

		_, err := h.store.SystemStore.GetSecret(testCredName, "")
		require.Error(t, err)
		assert.True(t, apierrors.IsInternalError(err))
	})

	t.Run("returns not found when listed secrets do not match cloud credential type", func(t *testing.T) {
		h := newSystemStoreHarness(t)
		secret := *secretForCredential(newCredential(testCredName))
		secret.Type = corev1.SecretTypeOpaque
		h.expectSecretListForName(testCredName, secret)

		_, err := h.store.SystemStore.GetSecret(testCredName, "")
		require.Error(t, err)
		assert.True(t, apierrors.IsNotFound(err))
	})

	t.Run("returns internal error on list failure", func(t *testing.T) {
		h := newSystemStoreHarness(t)
		h.secretCache.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			Return(nil, fmt.Errorf(genericErr))

		_, err := h.store.SystemStore.GetSecret(testCredName, "")
		require.Error(t, err)
		assert.True(t, apierrors.IsInternalError(err))
	})

	t.Run("returns not found when no cloudcredential secrets match", func(t *testing.T) {
		h := newSystemStoreHarness(t)
		secret := *secretForCredential(newCredential(testCredName))
		secret.Type = corev1.SecretTypeOpaque
		h.expectSecretListForName(testCredName, secret)

		_, err := h.store.SystemStore.GetSecret(testCredName, "")
		require.Error(t, err)
		assert.True(t, apierrors.IsNotFound(err))
	})

	t.Run("returns the first matching secret", func(t *testing.T) {
		h := newSystemStoreHarness(t)
		first := *secretForCredential(newCredential(testCredName))
		second := *secretForCredential(newCredential(testCredName))
		second.Name = "second-secret"

		h.expectSecretListForName(testCredName, first, second)

		result, err := h.store.SystemStore.GetSecret(testCredName, "")
		require.NoError(t, err)
		assert.Equal(t, first.Name, result.Name)
	})

	t.Run("filters listed secrets by request namespace", func(t *testing.T) {
		h := newSystemStoreHarness(t)
		first := *secretForCredential(newCredential(testCredName))
		first.Labels[CloudCredentialNamespaceLabel] = "ns-a"
		second := *secretForCredential(newCredential(testCredName))
		second.Name = "second-secret"
		second.Labels[CloudCredentialNamespaceLabel] = "ns-b"
		h.expectSecretListForNameAndNamespace(testCredName, "ns-b", first, second)

		result, err := h.store.SystemStore.GetSecret(testCredName, "ns-b")
		require.NoError(t, err)
		assert.Equal(t, second.Name, result.Name)
	})
}

func TestStoreGet(t *testing.T) {
	t.Parallel()

	t.Run("returns not found for non-owner", func(t *testing.T) {
		h := newStoreHarness(t, adminOnlyAuthorizer())
		secret := *secretForCredential(newCredential(testCredName))
		h.expectSecretListForName(testCredName, secret)

		_, err := h.store.Get(ctxWithUser(regularUser), testCredName, nil)
		require.Error(t, err)
		assert.True(t, apierrors.IsNotFound(err))
	})

	t.Run("returns converted credential", func(t *testing.T) {
		h := newStoreHarness(t, adminOnlyAuthorizer())
		secret := *secretForCredential(newCredential(testCredName))
		h.expectSecretListForName(testCredName, secret)

		obj, err := h.store.Get(ctxWithUser(adminUser), testCredName, nil)
		require.NoError(t, err)
		cred, ok := obj.(*ext.CloudCredential)
		require.True(t, ok)
		assert.Equal(t, testCredName, cred.Name)
		assert.Nil(t, cred.Spec.Credentials)
	})
}

func TestStoreList(t *testing.T) {
	t.Parallel()

	t.Run("with namespace adds label selector", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		ctx := ctxWithUser(adminUser)
		ctx = request.WithNamespace(ctx, "user-ns")

		h.secretClient.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			DoAndReturn(func(ns string, opts metav1.ListOptions) (*corev1.SecretList, error) {
				assert.Contains(t, opts.LabelSelector, CloudCredentialNamespaceLabel+"=user-ns")
				assert.Contains(t, opts.LabelSelector, CloudCredentialLabel+"=true")
				return &corev1.SecretList{}, nil
			})

		_, err := h.store.List(ctx, &metainternalversion.ListOptions{})
		require.NoError(t, err)
	})

	t.Run("without namespace does not add namespace filter", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		ctx := ctxWithUser(adminUser)

		h.secretClient.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			DoAndReturn(func(ns string, opts metav1.ListOptions) (*corev1.SecretList, error) {
				assert.NotContains(t, opts.LabelSelector, CloudCredentialNamespaceLabel)
				assert.Contains(t, opts.LabelSelector, CloudCredentialLabel+"=true")
				return &corev1.SecretList{}, nil
			})

		_, err := h.store.List(ctx, &metainternalversion.ListOptions{})
		require.NoError(t, err)
	})
}

// ============================================================================
// ConvertToTable / print tests
// ============================================================================

func TestGetReturnsVisibleFieldsAndPublicData(t *testing.T) {
	t.Parallel()

	h := newStoreHarness(t, adminOnlyAuthorizer())
	secret := secretForCredential(newCredential(testCredName))
	visible := []string{"accessKey"}
	visibleJSON, err := json.Marshal(visible)
	require.NoError(t, err)
	secret.Data[FieldVisibleFields] = visibleJSON
	secret.Data["accessKey"] = []byte("my-key")
	secret.Data["secretKey"] = []byte("super-secret")

	h.expectSecretListForName(testCredName, *secret)

	obj, err := h.store.Get(ctxWithUser(adminUser), testCredName, nil)
	require.NoError(t, err)
	cred := obj.(*ext.CloudCredential)
	// VisibleFields is write-only — not returned in spec
	assert.Empty(t, cred.Spec.VisibleFields)
	// PublicData should be populated from the VisibleFields stored in the secret
	require.NotNil(t, cred.Status.PublicData)
	assert.Equal(t, "my-key", cred.Status.PublicData["accessKey"])
	assert.NotContains(t, cred.Status.PublicData, "secretKey")
	assert.Nil(t, cred.Spec.Credentials, "credentials should never be returned on get")
}

// rbacAuthorizer returns an authorizer for comprehensive RBAC testing.
// - adminUser: has "*" (wildcard/admin) access
// - readOnlyUser: has "get" and "list" and "watch" access only
// - fullAccessUser: has all specific verbs but NOT "*"
// - noAccessUser: has no access at all
func TestResolvePublicData(t *testing.T) {
	t.Parallel()

	t.Run("nil cache returns nil", func(t *testing.T) {
		t.Parallel()
		secret := &corev1.Secret{Data: map[string][]byte{
			"accessKey": []byte("key"),
		}}
		result := resolvePublicData(secret, "amazonec2", nil)
		assert.Nil(t, result)
	})

	t.Run("schema-based excludes password fields", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		dsCache := fake.NewMockNonNamespacedCacheInterface[*v3.DynamicSchema](ctrl)
		dsCache.EXPECT().Get("amazonec2"+CredentialConfigSuffix).Return(&v3.DynamicSchema{
			Spec: v3.DynamicSchemaSpec{
				ResourceFields: map[string]v3.Field{
					"accessKey": {Type: "string"},
					"secretKey": {Type: "password"},
					"region":    {Type: "string"},
				},
			},
		}, nil)

		secret := &corev1.Secret{Data: map[string][]byte{
			"accessKey": []byte("AKIA123"),
			"secretKey": []byte("supersecret"),
			"region":    []byte("us-east-1"),
		}}

		result := resolvePublicData(secret, "amazonec2", dsCache)
		require.NotNil(t, result)
		assert.Equal(t, "AKIA123", result["accessKey"])
		assert.Equal(t, "us-east-1", result["region"])
		assert.NotContains(t, result, "secretKey")
	})

	t.Run("schema not found returns nil", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		dsCache := fake.NewMockNonNamespacedCacheInterface[*v3.DynamicSchema](ctrl)
		dsCache.EXPECT().Get(gomock.Any()).Return(nil, apierrors.NewNotFound(
			schema.GroupResource{Resource: "dynamicschemas"}, "test"))

		secret := &corev1.Secret{Data: map[string][]byte{
			"field1": []byte("value"),
		}}

		result := resolvePublicData(secret, "test", dsCache)
		assert.Nil(t, result)
	})

	t.Run("visible fields override takes precedence over schema", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		dsCache := fake.NewMockNonNamespacedCacheInterface[*v3.DynamicSchema](ctrl)
		// Schema should NOT be queried when VisibleFields are present

		visible := []string{"accessKey"}
		visibleJSON, _ := json.Marshal(visible)

		secret := &corev1.Secret{Data: map[string][]byte{
			FieldVisibleFields: visibleJSON,
			"accessKey":        []byte("AKIA123"),
			"secretKey":        []byte("supersecret"),
		}}

		result := resolvePublicData(secret, "amazonec2", dsCache)
		require.NotNil(t, result)
		assert.Equal(t, "AKIA123", result["accessKey"])
		assert.NotContains(t, result, "secretKey")
	})

	t.Run("visible field not in secret data is skipped", func(t *testing.T) {
		t.Parallel()
		visible := []string{"field-that-doesnt-exist", "accessKey"}
		visibleJSON, _ := json.Marshal(visible)

		secret := &corev1.Secret{Data: map[string][]byte{
			FieldVisibleFields: visibleJSON,
			"accessKey":        []byte("AKIA123"),
		}}

		result := resolvePublicData(secret, "amazonec2", nil)
		require.NotNil(t, result)
		assert.Len(t, result, 1)
		assert.Equal(t, "AKIA123", result["accessKey"])
	})

	t.Run("empty visible fields falls through to schema", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		dsCache := fake.NewMockNonNamespacedCacheInterface[*v3.DynamicSchema](ctrl)
		dsCache.EXPECT().Get("test"+CredentialConfigSuffix).Return(&v3.DynamicSchema{
			Spec: v3.DynamicSchemaSpec{
				ResourceFields: map[string]v3.Field{
					"field1": {Type: "string"},
				},
			},
		}, nil)

		visibleJSON, _ := json.Marshal([]string{})
		secret := &corev1.Secret{Data: map[string][]byte{
			FieldVisibleFields: visibleJSON,
			"field1":           []byte("value1"),
		}}

		result := resolvePublicData(secret, "test", dsCache)
		require.NotNil(t, result)
		assert.Equal(t, "value1", result["field1"])
	})

	t.Run("PublicFields on schema takes precedence over password heuristic", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		dsCache := fake.NewMockNonNamespacedCacheInterface[*v3.DynamicSchema](ctrl)
		dsCache.EXPECT().Get("amazonec2"+CredentialConfigSuffix).Return(&v3.DynamicSchema{
			Spec: v3.DynamicSchemaSpec{
				PublicFields: []string{"accessKey", "defaultRegion"},
				ResourceFields: map[string]v3.Field{
					"accessKey":     {Type: "string"},
					"secretKey":     {Type: "password"},
					"defaultRegion": {Type: "string"},
				},
			},
		}, nil)

		secret := &corev1.Secret{Data: map[string][]byte{
			"accessKey":     []byte("AKIA123"),
			"secretKey":     []byte("supersecret"),
			"defaultRegion": []byte("us-west-2"),
		}}

		result := resolvePublicData(secret, "amazonec2", dsCache)
		require.NotNil(t, result)
		assert.Equal(t, "AKIA123", result["accessKey"])
		assert.Equal(t, "us-west-2", result["defaultRegion"])
		assert.NotContains(t, result, "secretKey")
	})

	t.Run("PublicFields excludes fields not in the list even if non-password", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		dsCache := fake.NewMockNonNamespacedCacheInterface[*v3.DynamicSchema](ctrl)
		dsCache.EXPECT().Get("test"+CredentialConfigSuffix).Return(&v3.DynamicSchema{
			Spec: v3.DynamicSchemaSpec{
				PublicFields: []string{"field1"},
				ResourceFields: map[string]v3.Field{
					"field1": {Type: "string"},
					"field2": {Type: "string"},
					"field3": {Type: "password"},
				},
			},
		}, nil)

		secret := &corev1.Secret{Data: map[string][]byte{
			"field1": []byte("value1"),
			"field2": []byte("value2"),
			"field3": []byte("value3"),
		}}

		result := resolvePublicData(secret, "test", dsCache)
		require.NotNil(t, result)
		assert.Len(t, result, 1)
		assert.Equal(t, "value1", result["field1"])
		assert.NotContains(t, result, "field2")
		assert.NotContains(t, result, "field3")
	})

	t.Run("empty PublicFields falls back to password heuristic", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		dsCache := fake.NewMockNonNamespacedCacheInterface[*v3.DynamicSchema](ctrl)
		dsCache.EXPECT().Get("test"+CredentialConfigSuffix).Return(&v3.DynamicSchema{
			Spec: v3.DynamicSchemaSpec{
				ResourceFields: map[string]v3.Field{
					"username": {Type: "string"},
					"password": {Type: "password"},
				},
			},
		}, nil)

		secret := &corev1.Secret{Data: map[string][]byte{
			"username": []byte("admin"),
			"password": []byte("secret"),
		}}

		result := resolvePublicData(secret, "test", dsCache)
		require.NotNil(t, result)
		assert.Equal(t, "admin", result["username"])
		assert.NotContains(t, result, "password")
	})
}

// ============================================================================
// Store.List wrapper tests
// ============================================================================

func TestRBACGet(t *testing.T) {
	t.Parallel()

	t.Run("admin can get", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		secret := *secretForCredential(newCredential(testCredName))
		h.expectSecretListForName(testCredName, secret)

		obj, err := h.store.Get(ctxWithUser(adminUser), testCredName, nil)
		require.NoError(t, err)
		cred := obj.(*ext.CloudCredential)
		assert.Equal(t, testCredName, cred.Name)
	})

	t.Run("read-only user can get own credential", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		secret := *secretForCredential(newCredential(testCredName))
		setSecretOwner(&secret, readOnlyUser)
		h.expectSecretListForName(testCredName, secret)

		obj, err := h.store.Get(ctxWithUser(readOnlyUser), testCredName, nil)
		require.NoError(t, err)
		cred := obj.(*ext.CloudCredential)
		assert.Equal(t, testCredName, cred.Name)
	})

	t.Run("full-access non-admin can get own credential", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		secret := *secretForCredential(newCredential(testCredName))
		setSecretOwner(&secret, fullAccessUser)
		h.expectSecretListForName(testCredName, secret)

		obj, err := h.store.Get(ctxWithUser(fullAccessUser), testCredName, nil)
		require.NoError(t, err)
		cred := obj.(*ext.CloudCredential)
		assert.Equal(t, testCredName, cred.Name)
	})

	t.Run("non-owner cannot get another user's credential", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		secret := *secretForCredential(newCredential(testCredName))
		h.expectSecretListForName(testCredName, secret)

		_, err := h.store.Get(ctxWithUser(noAccessUser), testCredName, nil)
		require.Error(t, err)
		assert.True(t, apierrors.IsNotFound(err))
	})

	t.Run("missing user context errors", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())

		_, err := h.store.Get(context.Background(), testCredName, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no user info")
	})

	t.Run("authorizer error is propagated", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, errorAuthorizer())

		_, err := h.store.Get(ctxWithUser(adminUser), testCredName, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "authorizer error")
	})
}

func TestRBACList(t *testing.T) {
	t.Parallel()

	setupListSecrets := func(h *storeTestHarness, secrets ...corev1.Secret) {
		h.secretClient.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			Return(&corev1.SecretList{Items: secrets}, nil)
	}

	t.Run("admin can list", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		secret := *secretForCredential(newCredential(testCredName))
		setupListSecrets(h, secret)

		obj, err := h.store.list(ctxWithUser(adminUser), &metav1.ListOptions{})
		require.NoError(t, err)
		assert.Len(t, obj.Items, 1)
	})

	t.Run("read-only user can list own credentials", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		secret := *secretForCredential(newCredential(testCredName))
		setSecretOwner(&secret, readOnlyUser)
		setupListSecrets(h, secret)

		obj, err := h.store.list(ctxWithUser(readOnlyUser), &metav1.ListOptions{})
		require.NoError(t, err)
		assert.Len(t, obj.Items, 1)
	})

	t.Run("full-access non-admin can list own credentials", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		secret := *secretForCredential(newCredential(testCredName))
		setSecretOwner(&secret, fullAccessUser)
		setupListSecrets(h, secret)

		obj, err := h.store.list(ctxWithUser(fullAccessUser), &metav1.ListOptions{})
		require.NoError(t, err)
		assert.Len(t, obj.Items, 1)
	})

	t.Run("non-admin cannot see credentials owned by others", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())

		// Verify the List call includes the owner label selector for non-admin,
		// and return empty list (simulating API server filtering)
		h.secretClient.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			DoAndReturn(func(ns string, opts metav1.ListOptions) (*corev1.SecretList, error) {
				assert.Contains(t, opts.LabelSelector, CloudCredentialOwnerLabel+"="+readOnlyUser,
					"list should include owner label selector for non-admin users")
				return &corev1.SecretList{Items: []corev1.Secret{}}, nil
			})

		obj, err := h.store.list(ctxWithUser(readOnlyUser), &metav1.ListOptions{})
		require.NoError(t, err)
		assert.Len(t, obj.Items, 0)
	})

	t.Run("non-admin list applies owner filtering", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		h.secretClient.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			DoAndReturn(func(ns string, opts metav1.ListOptions) (*corev1.SecretList, error) {
				assert.Contains(t, opts.LabelSelector, CloudCredentialOwnerLabel+"="+noAccessUser)
				return &corev1.SecretList{Items: []corev1.Secret{}}, nil
			})

		obj, err := h.store.list(ctxWithUser(noAccessUser), &metav1.ListOptions{})
		require.NoError(t, err)
		assert.Len(t, obj.Items, 0)
	})

	t.Run("missing user context errors", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())

		_, err := h.store.list(context.Background(), &metav1.ListOptions{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no user info")
	})

	t.Run("authorizer error is propagated", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, errorAuthorizer())

		_, err := h.store.list(ctxWithUser(adminUser), &metav1.ListOptions{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "authorizer error")
	})
}

func TestRBACWatch(t *testing.T) {
	t.Parallel()

	t.Run("admin gets a watcher with secret watch", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		ctx, cancel := context.WithCancel(ctxWithUser(adminUser))
		defer cancel()

		fakeWatcher := watch.NewFake()
		h.secretClient.EXPECT().Watch(CredentialNamespace, gomock.Any()).Return(fakeWatcher, nil)

		w, err := h.store.watch(ctx, &metav1.ListOptions{})
		require.NoError(t, err)
		require.NotNil(t, w)
		fakeWatcher.Stop()
		w.Stop()
	})

	t.Run("read-only user gets a watcher with secret watch", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		ctx, cancel := context.WithCancel(ctxWithUser(readOnlyUser))
		defer cancel()

		fakeWatcher := watch.NewFake()
		h.secretClient.EXPECT().Watch(CredentialNamespace, gomock.Any()).Return(fakeWatcher, nil)

		w, err := h.store.watch(ctx, &metav1.ListOptions{})
		require.NoError(t, err)
		require.NotNil(t, w)
		fakeWatcher.Stop()
		w.Stop()
	})

	t.Run("full-access non-admin gets a watcher with secret watch", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		ctx, cancel := context.WithCancel(ctxWithUser(fullAccessUser))
		defer cancel()

		fakeWatcher := watch.NewFake()
		h.secretClient.EXPECT().Watch(CredentialNamespace, gomock.Any()).Return(fakeWatcher, nil)

		w, err := h.store.watch(ctx, &metav1.ListOptions{})
		require.NoError(t, err)
		require.NotNil(t, w)
		fakeWatcher.Stop()
		w.Stop()
	})

	t.Run("non-admin user gets a watcher with owner filtering", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		ctx, cancel := context.WithCancel(ctxWithUser(noAccessUser))
		defer cancel()

		fakeWatcher := watch.NewFake()
		h.secretClient.EXPECT().
			Watch(CredentialNamespace, gomock.Any()).
			DoAndReturn(func(ns string, opts metav1.ListOptions) (watch.Interface, error) {
				assert.Contains(t, opts.LabelSelector, CloudCredentialOwnerLabel+"="+noAccessUser)
				return fakeWatcher, nil
			})

		w, err := h.store.watch(ctx, &metav1.ListOptions{})
		require.NoError(t, err)
		require.NotNil(t, w)
		fakeWatcher.Stop()
		w.Stop()
	})

	t.Run("missing user context errors", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())

		_, err := h.store.watch(context.Background(), &metav1.ListOptions{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no user info")
	})

	t.Run("authorizer error is propagated", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, errorAuthorizer())

		_, err := h.store.watch(ctxWithUser(adminUser), &metav1.ListOptions{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "authorizer error")
	})
}

func TestWatch(t *testing.T) {
	t.Parallel()

	t.Run("non-admin watch applies owner filtering", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		ctx, cancel := context.WithCancel(ctxWithUser(noAccessUser))
		defer cancel()

		fakeWatcher := watch.NewFake()
		h.secretClient.EXPECT().
			Watch(CredentialNamespace, gomock.Any()).
			DoAndReturn(func(ns string, opts metav1.ListOptions) (watch.Interface, error) {
				assert.Contains(t, opts.LabelSelector, CloudCredentialOwnerLabel+"="+noAccessUser)
				return fakeWatcher, nil
			})

		w, err := h.store.watch(ctx, &metav1.ListOptions{})
		require.NoError(t, err)
		require.NotNil(t, w)
		w.Stop()
		fakeWatcher.Stop()
	})

	t.Run("admin receives credential events", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		ctx := ctxWithUser(adminUser)

		secret := secretForCredential(newCredential(testCredName))
		fakeWatcher := watch.NewFake()

		h.secretClient.EXPECT().
			Watch(CredentialNamespace, gomock.Any()).
			Return(fakeWatcher, nil)

		w, err := h.store.watch(ctx, &metav1.ListOptions{})
		require.NoError(t, err)

		// Send an Added event
		go fakeWatcher.Add(secret)

		event := <-w.ResultChan()
		assert.Equal(t, watch.Added, event.Type)
		cred, ok := event.Object.(*ext.CloudCredential)
		assert.True(t, ok)
		assert.Equal(t, testCredName, cred.Name)

		w.Stop()
		fakeWatcher.Stop()
	})

	t.Run("non-admin only sees own events", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		ctx := ctxWithUser(readOnlyUser)

		ownSecret := secretForCredential(newCredential("own-cred"))
		setSecretOwner(ownSecret, readOnlyUser)

		fakeWatcher := watch.NewFake()

		// Verify that the Watch call includes the owner label selector
		h.secretClient.EXPECT().
			Watch(CredentialNamespace, gomock.Any()).
			DoAndReturn(func(ns string, opts metav1.ListOptions) (watch.Interface, error) {
				assert.Contains(t, opts.LabelSelector, CloudCredentialOwnerLabel+"="+readOnlyUser,
					"watch should include owner label selector for non-admin users")
				return fakeWatcher, nil
			})

		w, err := h.store.watch(ctx, &metav1.ListOptions{})
		require.NoError(t, err)

		// Only own events are sent by the API server (filtered by label selector)
		go func() {
			fakeWatcher.Add(ownSecret)
		}()

		event := <-w.ResultChan()
		assert.Equal(t, watch.Added, event.Type)
		cred := event.Object.(*ext.CloudCredential)
		assert.Equal(t, "own-cred", cred.Name)

		w.Stop()
		fakeWatcher.Stop()
	})

	t.Run("bookmark event preserves resource version", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		ctx := ctxWithUser(adminUser)

		fakeWatcher := watch.NewFake()

		h.secretClient.EXPECT().
			Watch(CredentialNamespace, gomock.Any()).
			Return(fakeWatcher, nil)

		w, err := h.store.watch(ctx, &metav1.ListOptions{})
		require.NoError(t, err)

		go fakeWatcher.Action(watch.Bookmark, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{ResourceVersion: "12345"},
		})

		event := <-w.ResultChan()
		assert.Equal(t, watch.Bookmark, event.Type)
		cred := event.Object.(*ext.CloudCredential)
		assert.Equal(t, "12345", cred.ResourceVersion)

		w.Stop()
		fakeWatcher.Stop()
	})

	t.Run("watch error returns internal error", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		ctx := ctxWithUser(adminUser)

		h.secretClient.EXPECT().
			Watch(CredentialNamespace, gomock.Any()).
			Return(nil, fmt.Errorf("watch failed"))

		_, err := h.store.watch(ctx, &metav1.ListOptions{})
		require.Error(t, err)
		assert.True(t, apierrors.IsInternalError(err))
	})
}

// ============================================================================
// resolvePublicData tests
// ============================================================================

func TestSystemStoreListErrors(t *testing.T) {
	t.Parallel()

	t.Run("ResourceExpired error is re-thrown", func(t *testing.T) {
		t.Parallel()
		h := newSystemStoreHarness(t)
		h.secretClient.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			Return(nil, apierrors.NewResourceExpired("resource version too old"))

		_, err := h.store.SystemStore.list(&metav1.ListOptions{})
		require.Error(t, err)
		assert.True(t, apierrors.IsResourceExpired(err))
	})

	t.Run("Gone error is re-thrown as ResourceExpired", func(t *testing.T) {
		t.Parallel()
		h := newSystemStoreHarness(t)
		h.secretClient.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			Return(nil, apierrors.NewGone("gone"))

		_, err := h.store.SystemStore.list(&metav1.ListOptions{})
		require.Error(t, err)
		assert.True(t, apierrors.IsResourceExpired(err))
	})

	t.Run("other errors wrapped as InternalError", func(t *testing.T) {
		t.Parallel()
		h := newSystemStoreHarness(t)
		h.secretClient.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			Return(nil, fmt.Errorf("connection refused"))

		_, err := h.store.SystemStore.list(&metav1.ListOptions{})
		require.Error(t, err)
		assert.True(t, apierrors.IsInternalError(err))
	})

	t.Run("combines existing label selector", func(t *testing.T) {
		t.Parallel()
		h := newSystemStoreHarness(t)
		h.secretClient.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			DoAndReturn(func(ns string, opts metav1.ListOptions) (*corev1.SecretList, error) {
				assert.Contains(t, opts.LabelSelector, "custom-label=foo")
				assert.Contains(t, opts.LabelSelector, CloudCredentialLabel+"=true")
				return &corev1.SecretList{}, nil
			})

		_, err := h.store.SystemStore.list(&metav1.ListOptions{LabelSelector: "custom-label=foo"})
		require.NoError(t, err)
	})

	t.Run("non-cloud-credential secrets are filtered out", func(t *testing.T) {
		t.Parallel()
		h := newSystemStoreHarness(t)

		goodSecret := *secretForCredential(newCredential(testCredName))
		badSecret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "not-a-cc", Namespace: CredentialNamespace},
			Type:       "kubernetes.io/tls",
		}

		h.secretClient.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			Return(&corev1.SecretList{Items: []corev1.Secret{goodSecret, badSecret}}, nil)

		result, err := h.store.SystemStore.list(&metav1.ListOptions{})
		require.NoError(t, err)
		assert.Len(t, result.Items, 1)
		assert.Equal(t, testCredName, result.Items[0].Name)
	})
}
