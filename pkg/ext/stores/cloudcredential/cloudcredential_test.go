package cloudcredential

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	k8suser "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/kubernetes/pkg/printers"
)

const (
	adminUser      = "u-admin"
	regularUser    = "u-standard"
	readOnlyUser   = "u-readonly"
	fullAccessUser = "u-fullaccess"
	noAccessUser   = "u-noaccess"
	genericErr     = "boom"
	testCredName   = "cc-demo"
)

var fixedNow = time.Date(2024, time.January, 2, 3, 4, 5, 6, time.UTC)

type storeTestHarness struct {
	t                  *testing.T
	ctrl               *gomock.Controller
	store              *Store
	namespaceClient    *fake.MockNonNamespacedClientInterface[*corev1.Namespace, *corev1.NamespaceList]
	namespaceCache     *fake.MockNonNamespacedCacheInterface[*corev1.Namespace]
	secretClient       *fake.MockClientInterface[*corev1.Secret, *corev1.SecretList]
	secretCache        *fake.MockCacheInterface[*corev1.Secret]
	dynamicSchemaCache *fake.MockNonNamespacedCacheInterface[*v3.DynamicSchema]
}

func newStoreHarness(t *testing.T, auth authorizer.Authorizer) *storeTestHarness {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	nsClient := fake.NewMockNonNamespacedClientInterface[*corev1.Namespace, *corev1.NamespaceList](ctrl)
	nsCache := fake.NewMockNonNamespacedCacheInterface[*corev1.Namespace](ctrl)
	secretClient := fake.NewMockClientInterface[*corev1.Secret, *corev1.SecretList](ctrl)
	secretCache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
	dsCache := fake.NewMockNonNamespacedCacheInterface[*v3.DynamicSchema](ctrl)

	// By default, DynamicSchema cache returns a basic schema for any type lookup.
	// This allows existing tests that use "amazon" type to pass validation.
	// Tests that need specific schema behavior should override with explicit expectations.
	dsCache.EXPECT().Get(gomock.Any()).Return(&v3.DynamicSchema{
		Spec: v3.DynamicSchemaSpec{
			ResourceFields: map[string]v3.Field{},
		},
	}, nil).AnyTimes()

	if auth == nil {
		auth = authorizer.AuthorizerFunc(func(ctx context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
			return authorizer.DecisionAllow, "", nil
		})
	}

	store := &Store{
		auth: &credentialAuth{authorizer: auth},
		SystemStore: SystemStore{
			namespaceClient:    nsClient,
			namespaceCache:     nsCache,
			secretClient:       secretClient,
			secretCache:        secretCache,
			dynamicSchemaCache: dsCache,
		},
	}

	return &storeTestHarness{
		t:                  t,
		ctrl:               ctrl,
		store:              store,
		namespaceClient:    nsClient,
		namespaceCache:     nsCache,
		secretClient:       secretClient,
		secretCache:        secretCache,
		dynamicSchemaCache: dsCache,
	}
}

func newSystemStoreHarness(t *testing.T) *storeTestHarness {
	t.Helper()
	return newStoreHarness(t, nil)
}

func ctxWithUser(name string) context.Context {
	return request.WithUser(context.Background(), &k8suser.DefaultInfo{Name: name})
}

func adminOnlyAuthorizer() authorizer.Authorizer {
	return authorizer.AuthorizerFunc(func(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
		if attrs.GetUser() != nil && attrs.GetUser().GetName() == adminUser {
			return authorizer.DecisionAllow, "", nil
		}
		return authorizer.DecisionDeny, "", nil
	})
}

// verbAuthorizer builds an authorizer that grants per-user, per-verb access.
// The verbs map is keyed by username → set of allowed verbs.
// A verb of "*" in the allowed set matches the admin/wildcard check.
func verbAuthorizer(verbs map[string]map[string]bool) authorizer.Authorizer {
	return authorizer.AuthorizerFunc(func(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
		user := attrs.GetUser()
		if user == nil {
			return authorizer.DecisionDeny, "", nil
		}
		allowed, ok := verbs[user.GetName()]
		if !ok {
			return authorizer.DecisionDeny, "", nil
		}
		if allowed[attrs.GetVerb()] {
			return authorizer.DecisionAllow, "", nil
		}
		return authorizer.DecisionDeny, "", nil
	})
}

// errorAuthorizer returns an authorizer that always errors.
func errorAuthorizer() authorizer.Authorizer {
	return authorizer.AuthorizerFunc(func(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
		return authorizer.DecisionDeny, "", fmt.Errorf("authorizer error")
	})
}

func newCredential(name string) *ext.CloudCredential {
	return &ext.CloudCredential{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         "ns-default",
			UID:               typesUID("cred-uid"),
			ResourceVersion:   "1",
			CreationTimestamp: metav1.NewTime(fixedNow),
		},
		Spec: ext.CloudCredentialSpec{
			Type: "amazon",
			Credentials: map[string]string{
				"accessKey": "key",
				"secretKey": "secret",
			},
			Description: "sample credential",
		},
		Status: ext.CloudCredentialStatus{},
	}
}

func typesUID(s string) types.UID {
	return types.UID(s)
}

func secretForCredential(credential *ext.CloudCredential) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            credential.Name + "-secret",
			Namespace:       CredentialNamespace,
			ResourceVersion: "1",
			UID:             typesUID("secret-uid"),
			Labels: map[string]string{
				LabelCloudCredential:          "true",
				LabelCloudCredentialName:      credential.Name,
				LabelCloudCredentialNamespace: credential.Namespace,
			},
			Annotations: map[string]string{
				AnnotationDescription: credential.Spec.Description,
				AnnotationOwner:       adminUser,
				AnnotationCreatorID:   adminUser,
			},
			ManagedFields: []metav1.ManagedFieldsEntry{
				{
					FieldsV1: &metav1.FieldsV1{Raw: []byte(`{"f:metadata":{}}`)},
				},
			},
		},
		Type: corev1.SecretType(SecretTypePrefix + credential.Spec.Type),
		Data: map[string][]byte{
			FieldConditions: []byte("[]"),
		},
	}
}

type fakeUpdatedObjectInfo struct {
	obj runtime.Object
	err error
}

func (f *fakeUpdatedObjectInfo) Preconditions() *metav1.Preconditions {
	return nil
}

func (f *fakeUpdatedObjectInfo) UpdatedObject(ctx context.Context, oldObj runtime.Object) (runtime.Object, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.obj, nil
}

func (h *storeTestHarness) expectNamespaceExists() {
	h.namespaceCache.EXPECT().Get(CredentialNamespace).Return(&corev1.Namespace{}, nil)
}

func (h *storeTestHarness) expectSecretListForName(name string, secrets ...corev1.Secret) {
	selector := fmt.Sprintf("%s=%s", LabelCloudCredentialName, name)
	h.secretClient.EXPECT().
		List(CredentialNamespace, metav1.ListOptions{LabelSelector: selector}).
		Return(&corev1.SecretList{Items: secrets}, nil)
}

func (h *storeTestHarness) expectSecretListForNameAndNamespace(name, namespace string, secrets ...corev1.Secret) {
	selector := fmt.Sprintf("%s=%s,%s=%s", LabelCloudCredentialName, name, LabelCloudCredentialNamespace, namespace)
	h.secretClient.EXPECT().
		List(CredentialNamespace, metav1.ListOptions{LabelSelector: selector}).
		Return(&corev1.SecretList{Items: secrets}, nil)
}

func (h *storeTestHarness) expectSecretIndexForName(name string, secrets ...*corev1.Secret) {
	h.secretCache.EXPECT().
		GetByIndex(ByCloudCredentialNameIndex, name).
		Return(secrets, nil)
}

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

	t.Run("requires admin permissions", func(t *testing.T) {
		h := newStoreHarness(t, adminOnlyAuthorizer())
		ctx := ctxWithUser(regularUser)
		credential := newCredential(testCredName)

		_, err := h.store.Create(ctx, credential, nil, nil)
		require.Error(t, err)
		assert.True(t, apierrors.IsForbidden(err))
	})

	t.Run("creates backing secret with owner annotation", func(t *testing.T) {
		h := newStoreHarness(t, adminOnlyAuthorizer())
		ctx := ctxWithUser(adminUser)
		credential := newCredential(testCredName)

		h.expectNamespaceExists()

		h.secretClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
			require.Equal(t, adminUser, secret.Annotations[AnnotationOwner])
			require.Equal(t, CredentialNamespace, secret.Namespace)
			require.Equal(t, SecretTypePrefix+credential.Spec.Type, string(secret.Type))
			secret.Name = "secret-created"
			secret.Labels[LabelCloudCredentialName] = credential.Name
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

	h := newSystemStoreHarness(t)
	ctx := context.Background()

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
			_, err := h.store.SystemStore.Create(ctx, tt.credential, nil, adminUser)
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

	t.Run("fails when namespace ensure errors", func(t *testing.T) {
		h := newSystemStoreHarness(t)
		credential := newCredential(testCredName)
		h.namespaceCache.EXPECT().Get(CredentialNamespace).Return(nil, fmt.Errorf(genericErr))

		_, err := h.store.SystemStore.Create(context.Background(), credential, nil, adminUser)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error ensuring namespace")
	})

	t.Run("propagates secret client error", func(t *testing.T) {
		h := newSystemStoreHarness(t)
		credential := newCredential(testCredName)
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
	h.expectNamespaceExists()

	createdSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret-created",
			Namespace: CredentialNamespace,
			Labels: map[string]string{
				LabelCloudCredentialName: credential.Name,
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

func TestSystemStoreGetSecret(t *testing.T) {
	t.Parallel()

	t.Run("uses cache index when enabled", func(t *testing.T) {
		h := newSystemStoreHarness(t)
		h.store.SystemStore.indexByNameEnabled = true
		secret := secretForCredential(newCredential(testCredName))
		h.expectSecretIndexForName(testCredName, secret)

		result, err := h.store.SystemStore.GetSecret(testCredName, "", nil, true)
		require.NoError(t, err)
		assert.Equal(t, secret.Name, result.Name)
	})

	t.Run("falls back to list on cache index error", func(t *testing.T) {
		h := newSystemStoreHarness(t)
		h.store.SystemStore.indexByNameEnabled = true

		h.secretCache.EXPECT().
			GetByIndex(ByCloudCredentialNameIndex, testCredName).
			Return(nil, fmt.Errorf(genericErr))

		selector := fmt.Sprintf("%s=%s", LabelCloudCredentialName, testCredName)
		secret := *secretForCredential(newCredential(testCredName))
		h.secretClient.EXPECT().
			List(CredentialNamespace, metav1.ListOptions{LabelSelector: selector}).
			Return(&corev1.SecretList{Items: []corev1.Secret{secret}}, nil)

		result, err := h.store.SystemStore.GetSecret(testCredName, "", nil, true)
		require.NoError(t, err)
		assert.Equal(t, secret.Name, result.Name)
	})

	t.Run("returns not found when indexed secrets do not match cloud credential type", func(t *testing.T) {
		h := newSystemStoreHarness(t)
		h.store.SystemStore.indexByNameEnabled = true
		secret := secretForCredential(newCredential(testCredName))
		secret.Type = corev1.SecretTypeOpaque
		h.expectSecretIndexForName(testCredName, secret)

		_, err := h.store.SystemStore.GetSecret(testCredName, "", nil, true)
		require.Error(t, err)
		assert.True(t, apierrors.IsNotFound(err))
	})

	t.Run("returns internal error on list failure", func(t *testing.T) {
		h := newSystemStoreHarness(t)
		selector := fmt.Sprintf("%s=%s", LabelCloudCredentialName, testCredName)
		h.secretClient.EXPECT().
			List(CredentialNamespace, metav1.ListOptions{LabelSelector: selector}).
			Return(nil, fmt.Errorf(genericErr))

		_, err := h.store.SystemStore.GetSecret(testCredName, "", nil, false)
		require.Error(t, err)
		assert.True(t, apierrors.IsInternalError(err))
	})

	t.Run("returns not found when no cloudcredential secrets match", func(t *testing.T) {
		h := newSystemStoreHarness(t)
		selector := fmt.Sprintf("%s=%s", LabelCloudCredentialName, testCredName)
		secret := *secretForCredential(newCredential(testCredName))
		secret.Type = corev1.SecretTypeOpaque
		h.secretClient.EXPECT().
			List(CredentialNamespace, metav1.ListOptions{LabelSelector: selector}).
			Return(&corev1.SecretList{Items: []corev1.Secret{secret}}, nil)

		_, err := h.store.SystemStore.GetSecret(testCredName, "", nil, false)
		require.Error(t, err)
		assert.True(t, apierrors.IsNotFound(err))
	})

	t.Run("returns the first matching secret", func(t *testing.T) {
		h := newSystemStoreHarness(t)
		selector := fmt.Sprintf("%s=%s", LabelCloudCredentialName, testCredName)
		first := *secretForCredential(newCredential(testCredName))
		second := *secretForCredential(newCredential(testCredName))
		second.Name = "second-secret"

		h.secretClient.EXPECT().
			List(CredentialNamespace, metav1.ListOptions{LabelSelector: selector}).
			Return(&corev1.SecretList{Items: []corev1.Secret{first, second}}, nil)

		result, err := h.store.SystemStore.GetSecret(testCredName, "", nil, false)
		require.NoError(t, err)
		assert.Equal(t, first.Name, result.Name)
	})

	t.Run("filters indexed secrets by request namespace", func(t *testing.T) {
		h := newSystemStoreHarness(t)
		h.store.SystemStore.indexByNameEnabled = true
		first := secretForCredential(newCredential(testCredName))
		first.Labels[LabelCloudCredentialNamespace] = "ns-a"
		second := secretForCredential(newCredential(testCredName))
		second.Name = "second-secret"
		second.Labels[LabelCloudCredentialNamespace] = "ns-b"
		h.expectSecretIndexForName(testCredName, first, second)

		result, err := h.store.SystemStore.GetSecret(testCredName, "ns-b", nil, true)
		require.NoError(t, err)
		assert.Equal(t, second.Name, result.Name)
	})

	t.Run("filters listed secrets by request namespace", func(t *testing.T) {
		h := newSystemStoreHarness(t)
		first := *secretForCredential(newCredential(testCredName))
		first.Labels[LabelCloudCredentialNamespace] = "ns-a"
		second := *secretForCredential(newCredential(testCredName))
		second.Name = "second-secret"
		second.Labels[LabelCloudCredentialNamespace] = "ns-b"
		h.expectSecretListForNameAndNamespace(testCredName, "ns-b", first, second)

		result, err := h.store.SystemStore.GetSecret(testCredName, "ns-b", nil, false)
		require.NoError(t, err)
		assert.Equal(t, second.Name, result.Name)
	})
}

func TestStoreGet(t *testing.T) {
	t.Parallel()

	t.Run("requires admin permissions", func(t *testing.T) {
		h := newStoreHarness(t, adminOnlyAuthorizer())
		_, err := h.store.Get(ctxWithUser(regularUser), testCredName, nil)
		require.Error(t, err)
		assert.True(t, apierrors.IsForbidden(err))
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
			assert.Equal(t, adminUser, s.Annotations[AnnotationOwner])
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

func TestToSecret(t *testing.T) {
	t.Parallel()

	credential := newCredential(testCredName)
	credential.Labels = map[string]string{"custom": "label"}
	credential.Annotations = map[string]string{"custom-annotation": "value"}
	credential.Finalizers = []string{"cleanup"}
	credential.OwnerReferences = []metav1.OwnerReference{{Name: "owner"}}
	credential.Status.Conditions = []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue},
	}
	credential.Status.Secret = &corev1.ObjectReference{
		Name: "existing-secret",
	}

	secret, err := toSecret(credential, adminUser)
	require.NoError(t, err)

	assert.Equal(t, "existing-secret", secret.Name)
	assert.Equal(t, CredentialNamespace, secret.Namespace)
	assert.Equal(t, "true", secret.Labels[LabelCloudCredential])
	assert.Equal(t, credential.Name, secret.Labels[LabelCloudCredentialName])
	assert.Equal(t, credential.Namespace, secret.Labels[LabelCloudCredentialNamespace])

	assert.Equal(t, adminUser, secret.Annotations[AnnotationOwner])
	assert.Equal(t, credential.Spec.Description, secret.Annotations[AnnotationDescription])
	assert.Equal(t, "value", secret.Annotations["custom-annotation"])

	require.Contains(t, secret.Data, "accessKey")
	assert.Equal(t, []byte("key"), secret.Data["accessKey"])
	assert.Equal(t, credential.Finalizers, secret.Finalizers)
	assert.Equal(t, credential.OwnerReferences, secret.OwnerReferences)
}

func TestFromSecret(t *testing.T) {
	t.Parallel()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "secret-name",
			Namespace:       CredentialNamespace,
			ResourceVersion: "10",
			UID:             typesUID("secret-uid"),
			Labels: map[string]string{
				LabelCloudCredential:          "true",
				LabelCloudCredentialName:      testCredName,
				LabelCloudCredentialNamespace: "ns-default",
				"custom":                      "value",
			},
			Annotations: map[string]string{
				AnnotationDescription: "desc",
				AnnotationOwner:       adminUser,
				"custom-annotation":   "annotation",
			},
			ManagedFields: []metav1.ManagedFieldsEntry{
				{FieldsV1: &metav1.FieldsV1{Raw: []byte(`{"f:data":{}}`)}},
			},
		},
		Type: corev1.SecretType(SecretTypePrefix + "amazon"),
		Data: map[string][]byte{
			FieldConditions: []byte(`[{"type":"Ready","status":"True"}]`),
		},
	}

	credential, err := fromSecret(secret, nil)
	require.NoError(t, err)

	assert.Equal(t, testCredName, credential.Name)
	assert.Equal(t, "ns-default", credential.Namespace)
	assert.Equal(t, "amazon", credential.Spec.Type)
	assert.Equal(t, "desc", credential.Spec.Description)
	assert.Equal(t, "value", credential.Labels["custom"])
	assert.Equal(t, "annotation", credential.Annotations["custom-annotation"])
	_, hasOwner := credential.Annotations[AnnotationOwner]
	assert.False(t, hasOwner)
	require.NotNil(t, credential.Status.Secret)
	assert.Equal(t, secret.Name, credential.Status.Secret.Name)
	assert.Len(t, credential.Status.Conditions, 1)
}

func TestFromSecretErrors(t *testing.T) {
	t.Parallel()

	t.Run("invalid conditions json", func(t *testing.T) {
		secret := secretForCredential(newCredential(testCredName))
		secret.Data[FieldConditions] = []byte("{")

		_, err := fromSecret(secret, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal conditions")
	})

	t.Run("invalid managed fields mapping", func(t *testing.T) {
		secret := secretForCredential(newCredential(testCredName))
		secret.ManagedFields = []metav1.ManagedFieldsEntry{
			{FieldsV1: &metav1.FieldsV1{Raw: []byte("{")}},
		}

		_, err := fromSecret(secret, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to map secret managed-fields")
	})

	t.Run("invalid visible fields json is silently ignored (write-only field)", func(t *testing.T) {
		secret := secretForCredential(newCredential(testCredName))
		secret.Data[FieldVisibleFields] = []byte("{not-valid-json")

		cred, err := fromSecret(secret, nil)
		require.NoError(t, err)
		// VisibleFields is write-only — not returned in spec
		assert.Empty(t, cred.Spec.VisibleFields)
		// Invalid JSON means no public data can be resolved from VisibleFields
		assert.Empty(t, cred.Status.PublicData)
	})
}

func TestVisibleFieldsAndPublicData(t *testing.T) {
	t.Parallel()

	t.Run("toSecret stores visibleFields in secret data", func(t *testing.T) {
		credential := newCredential(testCredName)
		credential.Spec.VisibleFields = []string{
			"accessKey",
		}

		secret, err := toSecret(credential, adminUser)
		require.NoError(t, err)
		require.Contains(t, secret.Data, FieldVisibleFields)

		var stored []string
		require.NoError(t, json.Unmarshal(secret.Data[FieldVisibleFields], &stored))
		assert.Equal(t, credential.Spec.VisibleFields, stored)
	})

	t.Run("toSecret omits visibleFields when empty", func(t *testing.T) {
		credential := newCredential(testCredName)
		credential.Spec.VisibleFields = nil

		secret, err := toSecret(credential, adminUser)
		require.NoError(t, err)
		_, ok := secret.Data[FieldVisibleFields]
		assert.False(t, ok, "visibleFields should not be present in secret data when empty")
	})

	t.Run("fromSecret does not populate spec.visibleFields (write-only)", func(t *testing.T) {
		secret := secretForCredential(newCredential(testCredName))
		visible := []string{"accessKey"}
		visibleJSON, err := json.Marshal(visible)
		require.NoError(t, err)
		secret.Data[FieldVisibleFields] = visibleJSON
		secret.Data["accessKey"] = []byte("my-key")

		credential, err := fromSecret(secret, nil)
		require.NoError(t, err)
		// VisibleFields is write-only and should NOT be returned on read
		assert.Nil(t, credential.Spec.VisibleFields)
	})

	t.Run("fromSecret populates publicData for visible fields", func(t *testing.T) {
		secret := secretForCredential(newCredential(testCredName))
		visible := []string{"accessKey", "region"}
		visibleJSON, err := json.Marshal(visible)
		require.NoError(t, err)
		secret.Data[FieldVisibleFields] = visibleJSON
		secret.Data["accessKey"] = []byte("my-key")
		secret.Data["region"] = []byte("us-east-1")

		credential, err := fromSecret(secret, nil)
		require.NoError(t, err)
		require.NotNil(t, credential.Status.PublicData)
		assert.Equal(t, "my-key", credential.Status.PublicData["accessKey"])
		assert.Equal(t, "us-east-1", credential.Status.PublicData["region"])
	})

	t.Run("fromSecret excludes non-visible fields from publicData", func(t *testing.T) {
		secret := secretForCredential(newCredential(testCredName))
		visible := []string{"accessKey"}
		visibleJSON, err := json.Marshal(visible)
		require.NoError(t, err)
		secret.Data[FieldVisibleFields] = visibleJSON
		secret.Data["accessKey"] = []byte("my-key")
		secret.Data["secretKey"] = []byte("super-secret")

		credential, err := fromSecret(secret, nil)
		require.NoError(t, err)
		require.NotNil(t, credential.Status.PublicData)
		assert.Contains(t, credential.Status.PublicData, "accessKey")
		assert.NotContains(t, credential.Status.PublicData, "secretKey")
	})

	t.Run("fromSecret skips visible fields not present in secret data", func(t *testing.T) {
		secret := secretForCredential(newCredential(testCredName))
		visible := []string{"accessKey", "missing"}
		visibleJSON, err := json.Marshal(visible)
		require.NoError(t, err)
		secret.Data[FieldVisibleFields] = visibleJSON
		secret.Data["accessKey"] = []byte("my-key")

		credential, err := fromSecret(secret, nil)
		require.NoError(t, err)
		// VisibleFields is write-only
		assert.Nil(t, credential.Spec.VisibleFields)
		require.NotNil(t, credential.Status.PublicData)
		assert.Equal(t, "my-key", credential.Status.PublicData["accessKey"])
		assert.NotContains(t, credential.Status.PublicData, "missing")
	})

	t.Run("fromSecret leaves publicData nil when no visible field values exist", func(t *testing.T) {
		secret := secretForCredential(newCredential(testCredName))
		visible := []string{"missing"}
		visibleJSON, err := json.Marshal(visible)
		require.NoError(t, err)
		secret.Data[FieldVisibleFields] = visibleJSON

		credential, err := fromSecret(secret, nil)
		require.NoError(t, err)
		// VisibleFields is write-only
		assert.Nil(t, credential.Spec.VisibleFields)
		assert.Nil(t, credential.Status.PublicData)
	})

	t.Run("fromSecret with no visibleFields leaves spec and publicData empty", func(t *testing.T) {
		secret := secretForCredential(newCredential(testCredName))
		secret.Data["accessKey"] = []byte("my-key")

		credential, err := fromSecret(secret, nil)
		require.NoError(t, err)
		assert.Nil(t, credential.Spec.VisibleFields)
		assert.Nil(t, credential.Status.PublicData)
	})

	t.Run("round-trip preserves visibleFields through toSecret and fromSecret", func(t *testing.T) {
		credential := newCredential(testCredName)
		credential.Spec.VisibleFields = []string{
			"accessKey",
		}

		secret, err := toSecret(credential, adminUser)
		require.NoError(t, err)

		result, err := fromSecret(secret, nil)
		require.NoError(t, err)
		// VisibleFields is write-only — not populated on read
		assert.Nil(t, result.Spec.VisibleFields)
		require.NotNil(t, result.Status.PublicData)
		assert.Equal(t, "key", result.Status.PublicData["accessKey"])
		assert.NotContains(t, result.Status.PublicData, "secretKey")
	})
}

func TestCreateWithVisibleFields(t *testing.T) {
	t.Parallel()

	t.Run("creates credential with visibleFields and returns publicData", func(t *testing.T) {
		h := newStoreHarness(t, adminOnlyAuthorizer())
		ctx := ctxWithUser(adminUser)
		credential := newCredential(testCredName)
		credential.Spec.VisibleFields = []string{"accessKey"}

		h.expectNamespaceExists()

		h.secretClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
			require.Contains(t, secret.Data, FieldVisibleFields)
			require.Contains(t, secret.Data, "accessKey")
			secret.Name = "secret-created"
			secret.Labels[LabelCloudCredentialName] = credential.Name
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
func rbacAuthorizer() authorizer.Authorizer {
	return verbAuthorizer(map[string]map[string]bool{
		adminUser: {"*": true},
		readOnlyUser: {
			"get":   true,
			"list":  true,
			"watch": true,
		},
		fullAccessUser: {
			"get":    true,
			"list":   true,
			"watch":  true,
			"create": true,
			"update": true,
			"delete": true,
		},
		noAccessUser: {},
	})
}

func TestRBACCreate(t *testing.T) {
	t.Parallel()

	t.Run("admin can create", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		ctx := ctxWithUser(adminUser)
		credential := newCredential(testCredName)

		h.expectNamespaceExists()
		h.secretClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
			secret.Name = "secret-created"
			secret.Labels[LabelCloudCredentialName] = credential.Name
			return secret, nil
		})

		created, err := h.store.Create(ctx, credential, nil, nil)
		require.NoError(t, err)
		require.IsType(t, &ext.CloudCredential{}, created)
	})

	t.Run("read-only user cannot create", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		ctx := ctxWithUser(readOnlyUser)
		credential := newCredential(testCredName)

		_, err := h.store.Create(ctx, credential, nil, nil)
		require.Error(t, err)
		assert.True(t, apierrors.IsForbidden(err))
	})

	t.Run("full-access non-admin can create", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		ctx := ctxWithUser(fullAccessUser)
		credential := newCredential(testCredName)

		h.expectNamespaceExists()
		h.secretClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
			secret.Name = "secret-created"
			secret.Labels[LabelCloudCredentialName] = credential.Name
			return secret, nil
		})

		created, err := h.store.Create(ctx, credential, nil, nil)
		require.NoError(t, err)
		require.IsType(t, &ext.CloudCredential{}, created)
	})

	t.Run("no-access user cannot create", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		ctx := ctxWithUser(noAccessUser)
		credential := newCredential(testCredName)

		_, err := h.store.Create(ctx, credential, nil, nil)
		require.Error(t, err)
		assert.True(t, apierrors.IsForbidden(err))
	})

	t.Run("missing user context errors", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		credential := newCredential(testCredName)

		_, err := h.store.Create(context.Background(), credential, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no user info")
	})

	t.Run("authorizer error is propagated", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, errorAuthorizer())
		ctx := ctxWithUser(adminUser)
		credential := newCredential(testCredName)

		_, err := h.store.Create(ctx, credential, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "authorizer error")
	})
}

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
		secret.Annotations[AnnotationOwner] = readOnlyUser
		secret.Annotations[AnnotationCreatorID] = readOnlyUser
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
		secret.Annotations[AnnotationOwner] = fullAccessUser
		secret.Annotations[AnnotationCreatorID] = fullAccessUser
		h.expectSecretListForName(testCredName, secret)

		obj, err := h.store.Get(ctxWithUser(fullAccessUser), testCredName, nil)
		require.NoError(t, err)
		cred := obj.(*ext.CloudCredential)
		assert.Equal(t, testCredName, cred.Name)
	})

	t.Run("no-access user cannot get", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())

		_, err := h.store.Get(ctxWithUser(noAccessUser), testCredName, nil)
		require.Error(t, err)
		assert.True(t, apierrors.IsForbidden(err))
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
		secret.Annotations[AnnotationOwner] = readOnlyUser
		secret.Annotations[AnnotationCreatorID] = readOnlyUser
		setupListSecrets(h, secret)

		obj, err := h.store.list(ctxWithUser(readOnlyUser), &metav1.ListOptions{})
		require.NoError(t, err)
		assert.Len(t, obj.Items, 1)
	})

	t.Run("full-access non-admin can list own credentials", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		secret := *secretForCredential(newCredential(testCredName))
		secret.Annotations[AnnotationOwner] = fullAccessUser
		secret.Annotations[AnnotationCreatorID] = fullAccessUser
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
				assert.Contains(t, opts.LabelSelector, LabelCloudCredentialOwner+"="+readOnlyUser,
					"list should include owner label selector for non-admin users")
				return &corev1.SecretList{Items: []corev1.Secret{}}, nil
			})

		obj, err := h.store.list(ctxWithUser(readOnlyUser), &metav1.ListOptions{})
		require.NoError(t, err)
		assert.Len(t, obj.Items, 0)
	})

	t.Run("no-access user cannot list", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())

		_, err := h.store.list(ctxWithUser(noAccessUser), &metav1.ListOptions{})
		require.Error(t, err)
		assert.True(t, apierrors.IsForbidden(err))
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

func TestRBACUpdate(t *testing.T) {
	t.Parallel()

	setupForUpdate := func(h *storeTestHarness) *ext.CloudCredential {
		secret := secretForCredential(newCredential(testCredName))
		h.expectSecretListForName(testCredName, *secret)
		updated, err := fromSecret(secret.DeepCopy(), nil)
		require.NoError(t, err)
		updated.Spec.Description = "updated"
		return updated
	}

	t.Run("admin can update", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		updated := setupForUpdate(h)

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
		updated := setupForUpdate(h)
		objInfo := &fakeUpdatedObjectInfo{obj: updated}

		_, _, err := h.store.Update(ctxWithUser(readOnlyUser), testCredName, objInfo, nil, nil, false, nil)
		require.Error(t, err)
		assert.True(t, apierrors.IsForbidden(err))
	})

	t.Run("full-access non-admin can update", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		updated := setupForUpdate(h)

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
		updated := setupForUpdate(h)
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

	t.Run("no-access user gets forbidden", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		ctx := ctxWithUser(noAccessUser)

		_, err := h.store.watch(ctx, &metav1.ListOptions{})
		require.Error(t, err)
		assert.True(t, apierrors.IsForbidden(err))
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

func TestRBACCrossVerb(t *testing.T) {
	t.Parallel()

	t.Run("read-only user can get but cannot create, update, or delete", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		ctx := ctxWithUser(readOnlyUser)
		credential := newCredential(testCredName)
		secret := *secretForCredential(credential)
		// Set owner to readOnlyUser so they can see their own credential
		secret.Annotations[AnnotationOwner] = readOnlyUser
		secret.Annotations[AnnotationCreatorID] = readOnlyUser

		// Can get own credential
		h.expectSecretListForName(testCredName, secret)
		obj, err := h.store.Get(ctx, testCredName, nil)
		require.NoError(t, err)
		assert.Equal(t, testCredName, obj.(*ext.CloudCredential).Name)

		// Cannot create
		_, err = h.store.Create(ctx, newCredential("other"), nil, nil)
		require.Error(t, err)
		assert.True(t, apierrors.IsForbidden(err))

		// Cannot update
		h.expectSecretListForName(testCredName, secret)
		updated, err := fromSecret(secret.DeepCopy(), nil)
		require.NoError(t, err)
		objInfo := &fakeUpdatedObjectInfo{obj: updated}
		_, _, err = h.store.Update(ctx, testCredName, objInfo, nil, nil, false, nil)
		require.Error(t, err)
		assert.True(t, apierrors.IsForbidden(err))

		// Cannot delete
		h.expectSecretListForName(testCredName, secret)
		_, _, err = h.store.Delete(ctx, testCredName, nil, nil)
		require.Error(t, err)
		assert.True(t, apierrors.IsForbidden(err))
	})

	t.Run("no-access user is denied for every operation", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		ctx := ctxWithUser(noAccessUser)
		credential := newCredential(testCredName)
		secret := *secretForCredential(credential)

		// Cannot get
		_, err := h.store.Get(ctx, testCredName, nil)
		require.Error(t, err)
		assert.True(t, apierrors.IsForbidden(err))

		// Cannot create
		_, err = h.store.Create(ctx, newCredential("other"), nil, nil)
		require.Error(t, err)
		assert.True(t, apierrors.IsForbidden(err))

		// Cannot list
		_, err = h.store.list(ctx, &metav1.ListOptions{})
		require.Error(t, err)
		assert.True(t, apierrors.IsForbidden(err))

		// Cannot update
		h.expectSecretListForName(testCredName, secret)
		updated, convErr := fromSecret(secret.DeepCopy(), nil)
		require.NoError(t, convErr)
		objInfo := &fakeUpdatedObjectInfo{obj: updated}
		_, _, err = h.store.Update(ctx, testCredName, objInfo, nil, nil, false, nil)
		require.Error(t, err)
		assert.True(t, apierrors.IsForbidden(err))

		// Cannot delete
		h.expectSecretListForName(testCredName, secret)
		_, _, err = h.store.Delete(ctx, testCredName, nil, nil)
		require.Error(t, err)
		assert.True(t, apierrors.IsForbidden(err))
	})

	t.Run("full-access non-admin can do everything", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		ctx := ctxWithUser(fullAccessUser)
		credential := newCredential(testCredName)

		// Can create
		h.expectNamespaceExists()
		h.secretClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
			secret.Name = "secret-created"
			secret.Labels[LabelCloudCredentialName] = credential.Name
			return secret, nil
		})
		_, err := h.store.Create(ctx, credential.DeepCopy(), nil, nil)
		require.NoError(t, err)

		// Can get own credential
		secret := *secretForCredential(credential)
		secret.Annotations[AnnotationOwner] = fullAccessUser
		secret.Annotations[AnnotationCreatorID] = fullAccessUser
		h.expectSecretListForName(testCredName, secret)
		obj, err := h.store.Get(ctx, testCredName, nil)
		require.NoError(t, err)
		assert.Equal(t, testCredName, obj.(*ext.CloudCredential).Name)

		// Can list own credentials
		h.secretClient.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			Return(&corev1.SecretList{Items: []corev1.Secret{secret}}, nil)
		listObj, err := h.store.list(ctx, &metav1.ListOptions{})
		require.NoError(t, err)
		assert.Len(t, listObj.Items, 1)

		// Can update own credential
		h.expectSecretListForName(testCredName, secret)
		updated, err := fromSecret(secret.DeepCopy(), nil)
		require.NoError(t, err)
		updated.Spec.Description = "updated"
		h.secretClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
			s.ResourceVersion = "2"
			return s, nil
		})
		objInfo := &fakeUpdatedObjectInfo{obj: updated}
		_, _, err = h.store.Update(ctx, testCredName, objInfo, nil, nil, false, &metav1.UpdateOptions{})
		require.NoError(t, err)

		// Can delete own credential
		h.expectSecretListForName(testCredName, secret)
		h.secretClient.EXPECT().Delete(CredentialNamespace, secret.Name, gomock.Any()).Return(nil)
		_, _, err = h.store.Delete(ctx, testCredName, nil, nil)
		require.NoError(t, err)
	})
}

// ============================================================================
// validateCredentialType tests
// ============================================================================

func TestValidateCredentialType(t *testing.T) {
	t.Parallel()

	t.Run("nil cache skips validation", func(t *testing.T) {
		t.Parallel()
		store := SystemStore{dynamicSchemaCache: nil}
		err := store.validateCredentialType("anytype")
		require.NoError(t, err)
	})

	t.Run("known schema type is allowed", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		dsCache := fake.NewMockNonNamespacedCacheInterface[*v3.DynamicSchema](ctrl)
		dsCache.EXPECT().Get("amazonec2"+CredentialConfigSuffix).Return(&v3.DynamicSchema{}, nil)

		store := SystemStore{dynamicSchemaCache: dsCache}
		err := store.validateCredentialType("amazonec2")
		require.NoError(t, err)
	})

	t.Run("unknown type with feature disabled returns error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		dsCache := fake.NewMockNonNamespacedCacheInterface[*v3.DynamicSchema](ctrl)
		dsCache.EXPECT().Get("unknowntype"+CredentialConfigSuffix).Return(nil, apierrors.NewNotFound(
			schema.GroupResource{Resource: "dynamicschemas"}, "unknowntypecredentialconfig"))

		store := SystemStore{dynamicSchemaCache: dsCache}
		err := store.validateCredentialType("unknowntype")
		require.Error(t, err)
		assert.True(t, apierrors.IsBadRequest(err))
		assert.Contains(t, err.Error(), "generic-cloud-credentials")
	})
}

// ============================================================================
// DeleteCollection tests
// ============================================================================

func TestDeleteCollection(t *testing.T) {
	t.Parallel()

	t.Run("non-admin forbidden", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		ctx := ctxWithUser(readOnlyUser)

		_, err := h.store.DeleteCollection(ctx, nil, nil, &metainternalversion.ListOptions{})
		require.Error(t, err)
		assert.True(t, apierrors.IsForbidden(err))
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

func TestUpdateStatus(t *testing.T) {
	t.Parallel()

	t.Run("updates with conditions", func(t *testing.T) {
		t.Parallel()
		h := newSystemStoreHarness(t)

		secret := secretForCredential(newCredential(testCredName))
		h.secretClient.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			Return(&corev1.SecretList{Items: []corev1.Secret{*secret}}, nil)

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
		h.secretClient.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			Return(&corev1.SecretList{Items: []corev1.Secret{*secret}}, nil)

		// No Patch call expected since there's nothing to update
		err := h.store.SystemStore.UpdateStatus("ns-default", testCredName, ext.CloudCredentialStatus{})
		require.NoError(t, err)
	})

	t.Run("secret not found returns error", func(t *testing.T) {
		t.Parallel()
		h := newSystemStoreHarness(t)

		h.secretClient.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			Return(&corev1.SecretList{Items: []corev1.Secret{}}, nil)

		err := h.store.SystemStore.UpdateStatus("ns-default", testCredName, ext.CloudCredentialStatus{})
		require.Error(t, err)
	})

	t.Run("patch error propagated", func(t *testing.T) {
		t.Parallel()
		h := newSystemStoreHarness(t)

		secret := secretForCredential(newCredential(testCredName))
		h.secretClient.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			Return(&corev1.SecretList{Items: []corev1.Secret{*secret}}, nil)

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
		first.Labels[LabelCloudCredentialNamespace] = "ns-a"
		second := *secretForCredential(newCredential(testCredName))
		second.Name = "second-secret"
		second.Labels[LabelCloudCredentialNamespace] = "ns-b"
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

func TestWatch(t *testing.T) {
	t.Parallel()

	t.Run("unauthorized user gets forbidden", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		ctx := ctxWithUser(noAccessUser)

		_, err := h.store.watch(ctx, &metav1.ListOptions{})
		require.Error(t, err)
		assert.True(t, apierrors.IsForbidden(err))
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
		ownSecret.Annotations[AnnotationOwner] = readOnlyUser
		ownSecret.Annotations[AnnotationCreatorID] = readOnlyUser
		ownSecret.Labels[LabelCloudCredentialOwner] = readOnlyUser

		fakeWatcher := watch.NewFake()

		// Verify that the Watch call includes the owner label selector
		h.secretClient.EXPECT().
			Watch(CredentialNamespace, gomock.Any()).
			DoAndReturn(func(ns string, opts metav1.ListOptions) (watch.Interface, error) {
				assert.Contains(t, opts.LabelSelector, LabelCloudCredentialOwner+"="+readOnlyUser,
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
				assert.Contains(t, opts.LabelSelector, LabelCloudCredentialNamespace+"=user-ns")
				assert.Contains(t, opts.LabelSelector, LabelCloudCredential+"=true")
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
				assert.NotContains(t, opts.LabelSelector, LabelCloudCredentialNamespace)
				assert.Contains(t, opts.LabelSelector, LabelCloudCredential+"=true")
				return &corev1.SecretList{}, nil
			})

		_, err := h.store.List(ctx, &metainternalversion.ListOptions{})
		require.NoError(t, err)
	})
}

// ============================================================================
// ConvertToTable / print tests
// ============================================================================

func TestPrintCloudCredential(t *testing.T) {
	t.Parallel()

	t.Run("credential with all fields", func(t *testing.T) {
		t.Parallel()
		cred := &ext.CloudCredential{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "my-cred",
				CreationTimestamp: metav1.NewTime(fixedNow),
				Annotations:       map[string]string{AnnotationOwner: "u-admin"},
			},
			Spec:   ext.CloudCredentialSpec{Type: "amazonec2", Description: "test cred"},
			Status: ext.CloudCredentialStatus{},
		}
		rows, err := printCloudCredential(cred, printers.GenerateOptions{})
		require.NoError(t, err)
		require.Len(t, rows, 1)
		cells := rows[0].Cells
		assert.Equal(t, "my-cred", cells[0])
		assert.Equal(t, "amazonec2", cells[1])
		assert.Equal(t, "u-admin", cells[3])
		assert.Equal(t, "test cred", cells[4])
	})

	t.Run("missing owner shows unknown", func(t *testing.T) {
		t.Parallel()
		cred := &ext.CloudCredential{
			ObjectMeta: metav1.ObjectMeta{Name: "cred"},
			Spec:       ext.CloudCredentialSpec{Type: "test"},
		}
		rows, err := printCloudCredential(cred, printers.GenerateOptions{})
		require.NoError(t, err)
		assert.Equal(t, "<unknown>", rows[0].Cells[3])
	})
}

func TestSanitizeLabelValue(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "system_admin", sanitizeLabelValue("system:admin"))
	assert.Equal(t, "u-12345", sanitizeLabelValue("u-12345"))
	assert.Equal(t, "normal.user_name", sanitizeLabelValue("normal.user_name"))
	assert.Equal(t, "user_with_spaces", sanitizeLabelValue("user with spaces"))
	assert.Equal(t, "", sanitizeLabelValue(""))
}

func TestListOptionMerge(t *testing.T) {
	t.Parallel()

	t.Run("admin gets options unchanged", func(t *testing.T) {
		t.Parallel()
		opts := &metav1.ListOptions{LabelSelector: "custom=true"}
		result, err := ListOptionMerge(true, "admin", opts)
		require.NoError(t, err)
		assert.Equal(t, "custom=true", result.LabelSelector)
	})

	t.Run("admin with nil options gets empty options", func(t *testing.T) {
		t.Parallel()
		result, err := ListOptionMerge(true, "admin", nil)
		require.NoError(t, err)
		assert.Equal(t, metav1.ListOptions{}, result)
	})

	t.Run("non-admin with no options gets owner selector", func(t *testing.T) {
		t.Parallel()
		opts := &metav1.ListOptions{}
		result, err := ListOptionMerge(false, "user-1", opts)
		require.NoError(t, err)
		assert.Contains(t, result.LabelSelector, LabelCloudCredentialOwner+"=user-1")
	})

	t.Run("non-admin with nil options gets owner selector", func(t *testing.T) {
		t.Parallel()
		result, err := ListOptionMerge(false, "user-1", nil)
		require.NoError(t, err)
		assert.Contains(t, result.LabelSelector, LabelCloudCredentialOwner+"=user-1")
	})

	t.Run("non-admin sanitizes owner label value", func(t *testing.T) {
		t.Parallel()
		result, err := ListOptionMerge(false, "system:admin", nil)
		require.NoError(t, err)
		assert.Contains(t, result.LabelSelector, LabelCloudCredentialOwner+"=system_admin")
	})

	t.Run("non-admin with existing selector merges owner", func(t *testing.T) {
		t.Parallel()
		opts := &metav1.ListOptions{LabelSelector: "custom=true"}
		result, err := ListOptionMerge(false, "user-1", opts)
		require.NoError(t, err)
		assert.Contains(t, result.LabelSelector, LabelCloudCredentialOwner+"=user-1")
		assert.Contains(t, result.LabelSelector, "custom=true")
	})

	t.Run("non-admin with same owner in selector passes through", func(t *testing.T) {
		t.Parallel()
		opts := &metav1.ListOptions{LabelSelector: LabelCloudCredentialOwner + "=user-1"}
		result, err := ListOptionMerge(false, "user-1", opts)
		require.NoError(t, err)
		assert.Contains(t, result.LabelSelector, LabelCloudCredentialOwner+"=user-1")
	})

	t.Run("non-admin with different owner in selector returns empty", func(t *testing.T) {
		t.Parallel()
		opts := &metav1.ListOptions{LabelSelector: LabelCloudCredentialOwner + "=other_user"}
		result, err := ListOptionMerge(false, "other:user", opts)
		require.NoError(t, err)
		assert.Contains(t, result.LabelSelector, LabelCloudCredentialOwner+"=other_user")
	})

	t.Run("non-admin with different owner after sanitization returns original selector", func(t *testing.T) {
		t.Parallel()
		opts := &metav1.ListOptions{LabelSelector: LabelCloudCredentialOwner + "=other-user"}
		result, err := ListOptionMerge(false, "user:1", opts)
		require.NoError(t, err)
		assert.Equal(t, opts.LabelSelector, result.LabelSelector)
	})
}

func TestPrintCloudCredentialList(t *testing.T) {
	t.Parallel()

	list := &ext.CloudCredentialList{
		Items: []ext.CloudCredential{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "cred1"},
				Spec:       ext.CloudCredentialSpec{Type: "amazon"},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "cred2"},
				Spec:       ext.CloudCredentialSpec{Type: "azure"},
			},
		},
	}

	rows, err := printCloudCredentialList(list, printers.GenerateOptions{})
	require.NoError(t, err)
	assert.Len(t, rows, 2)
	assert.Equal(t, "cred1", rows[0].Cells[0])
	assert.Equal(t, "cred2", rows[1].Cells[0])
}

// ============================================================================
// Store.Delete tests
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
				assert.Contains(t, opts.LabelSelector, LabelCloudCredential+"=true")
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
