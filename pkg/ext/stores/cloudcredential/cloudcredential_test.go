package cloudcredential

import (
	"context"
	"encoding/json"
	"fmt"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
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
		auth: auth,
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
		if attrs.GetResource() == "*" {
			if allowed["*"] {
				return authorizer.DecisionAllow, "", nil
			}
			return authorizer.DecisionDeny, "", nil
		}
		if allowed["*"] || allowed[attrs.GetVerb()] {
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
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            credential.Name + "-secret",
			Namespace:       CredentialNamespace,
			ResourceVersion: "1",
			UID:             typesUID("secret-uid"),
			Labels: map[string]string{
				CloudCredentialLabel:          "true",
				CloudCredentialNameLabel:      credential.Name,
				CloudCredentialNamespaceLabel: credential.Namespace,
			},
			Annotations: map[string]string{
				CloudCredentialDescriptionAnnotation: credential.Spec.Description,
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

	setSecretOwner(secret, adminUser)

	return secret
}

func setSecretOwner(secret *corev1.Secret, owner string) {
	secret.Labels[CloudCredentialOwnerLabel] = sanitizeLabelValue(owner)
	secret.Annotations[CreatorIDAnnotation] = owner
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

func secretPointers(secrets ...corev1.Secret) []*corev1.Secret {
	ptrs := make([]*corev1.Secret, 0, len(secrets))
	for i := range secrets {
		ptrs = append(ptrs, secrets[i].DeepCopy())
	}
	return ptrs
}

func (h *storeTestHarness) expectSecretListForName(name string, secrets ...corev1.Secret) {
	h.secretCache.EXPECT().
		List(CredentialNamespace, gomock.Any()).
		Return(secretPointers(secrets...), nil)
}

func (h *storeTestHarness) expectSecretListForNameAndNamespace(name, namespace string, secrets ...corev1.Secret) {
	h.secretCache.EXPECT().
		List(CredentialNamespace, gomock.Any()).
		Return(secretPointers(secrets...), nil)
}

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
	assert.Equal(t, "true", secret.Labels[CloudCredentialLabel])
	assert.Equal(t, credential.Name, secret.Labels[CloudCredentialNameLabel])
	assert.Equal(t, credential.Namespace, secret.Labels[CloudCredentialNamespaceLabel])
	assert.Equal(t, adminUser, secret.Labels[CloudCredentialOwnerLabel])

	assert.Equal(t, adminUser, secret.Annotations[CreatorIDAnnotation])
	assert.Equal(t, credential.Spec.Description, secret.Annotations[CloudCredentialDescriptionAnnotation])
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
				CloudCredentialLabel:          "true",
				CloudCredentialNameLabel:      testCredName,
				CloudCredentialNamespaceLabel: "ns-default",
				CloudCredentialOwnerLabel:     sanitizeLabelValue(adminUser),
				"custom":                      "value",
			},
			Annotations: map[string]string{
				CloudCredentialDescriptionAnnotation: "desc",
				CreatorIDAnnotation:                  adminUser,
				"custom-annotation":                  "annotation",
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
	assert.Equal(t, sanitizeLabelValue(adminUser), credential.Labels[CloudCredentialOwnerLabel])
	assert.Equal(t, "value", credential.Labels["custom"])
	assert.Equal(t, adminUser, credential.Annotations[CreatorIDAnnotation])
	assert.Equal(t, "annotation", credential.Annotations["custom-annotation"])
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

func TestRBACCrossVerb(t *testing.T) {
	t.Parallel()

	t.Run("non-admin owner can create, get, update, and delete", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		ctx := ctxWithUser(readOnlyUser)
		credential := newCredential(testCredName)
		secret := *secretForCredential(credential)
		setSecretOwner(&secret, readOnlyUser)

		// Can get own credential
		h.expectSecretListForName(testCredName, secret)
		obj, err := h.store.Get(ctx, testCredName, nil)
		require.NoError(t, err)
		assert.Equal(t, testCredName, obj.(*ext.CloudCredential).Name)

		// Can create
		h.secretCache.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			Return(([]*corev1.Secret)(nil), nil)
		h.expectNamespaceExists()
		h.secretClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
			secret.Name = "secret-created"
			secret.Labels[CloudCredentialNameLabel] = "other"
			return secret, nil
		})
		_, err = h.store.Create(ctx, newCredential("other"), nil, nil)
		require.NoError(t, err)

		// Can update own credential
		h.expectSecretListForName(testCredName, secret)
		updated, err := fromSecret(secret.DeepCopy(), nil)
		require.NoError(t, err)
		h.secretClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
			s.ResourceVersion = "2"
			return s, nil
		})
		objInfo := &fakeUpdatedObjectInfo{obj: updated}
		_, _, err = h.store.Update(ctx, testCredName, objInfo, nil, nil, false, nil)
		require.NoError(t, err)

		// Can delete own credential
		h.expectSecretListForName(testCredName, secret)
		h.secretClient.EXPECT().Delete(CredentialNamespace, secret.Name, gomock.Any()).Return(nil)
		_, _, err = h.store.Delete(ctx, testCredName, nil, nil)
		require.NoError(t, err)
	})

	t.Run("non-owner is filtered or denied for another user's credential", func(t *testing.T) {
		t.Parallel()
		h := newStoreHarness(t, rbacAuthorizer())
		ctx := ctxWithUser(noAccessUser)
		secret := *secretForCredential(newCredential(testCredName))

		// Cannot get another user's credential
		h.expectSecretListForName(testCredName, secret)
		_, err := h.store.Get(ctx, testCredName, nil)
		require.Error(t, err)
		assert.True(t, apierrors.IsNotFound(err))

		// Can create
		h.secretCache.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			Return(([]*corev1.Secret)(nil), nil)
		h.expectNamespaceExists()
		h.secretClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
			secret.Name = "secret-created"
			secret.Labels[CloudCredentialNameLabel] = "other"
			return secret, nil
		})
		_, err = h.store.Create(ctx, newCredential("other"), nil, nil)
		require.NoError(t, err)

		// List is filtered to own credentials
		h.secretClient.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			DoAndReturn(func(ns string, opts metav1.ListOptions) (*corev1.SecretList, error) {
				assert.Contains(t, opts.LabelSelector, CloudCredentialOwnerLabel+"="+noAccessUser)
				return &corev1.SecretList{Items: []corev1.Secret{}}, nil
			})
		listObj, err := h.store.list(ctx, &metav1.ListOptions{})
		require.NoError(t, err)
		assert.Len(t, listObj.Items, 0)

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
		h.secretCache.EXPECT().
			List(CredentialNamespace, gomock.Any()).
			Return(([]*corev1.Secret)(nil), nil)
		h.expectNamespaceExists()
		h.secretClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
			secret.Name = "secret-created"
			secret.Labels[CloudCredentialNameLabel] = credential.Name
			return secret, nil
		})
		_, err := h.store.Create(ctx, credential.DeepCopy(), nil, nil)
		require.NoError(t, err)

		// Can get own credential
		secret := *secretForCredential(credential)
		setSecretOwner(&secret, fullAccessUser)
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

func TestPrintCloudCredential(t *testing.T) {
	t.Parallel()

	t.Run("credential with all fields", func(t *testing.T) {
		t.Parallel()
		cred := &ext.CloudCredential{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "my-cred",
				CreationTimestamp: metav1.NewTime(fixedNow),
				Annotations:       map[string]string{CreatorIDAnnotation: "u-admin"},
			},
			Spec:   ext.CloudCredentialSpec{Type: "amazonec2", Description: "test cred"},
			Status: ext.CloudCredentialStatus{},
		}
		rows, err := cloudCredentialToTableRows(cred, printers.GenerateOptions{})
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
		rows, err := cloudCredentialToTableRows(cred, printers.GenerateOptions{})
		require.NoError(t, err)
		assert.Equal(t, unknownOwnerValue, rows[0].Cells[3])
	})
}

func TestSanitizeLabelValue(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "u-12345", sanitizeLabelValue("u-12345"))
	assert.Equal(t, "system_admin", sanitizeLabelValue("system:admin"))
	assert.Equal(t, "normal.user_name", sanitizeLabelValue("normal.user_name"))
	assert.Equal(t, "user_with_spaces", sanitizeLabelValue("user with spaces"))
	assert.Equal(t, "", sanitizeLabelValue(""))
}

func TestToListOptions(t *testing.T) {
	t.Parallel()

	t.Run("admin gets options unchanged", func(t *testing.T) {
		t.Parallel()
		u := k8suser.DefaultInfo{Name: "admin"}
		opts := &metav1.ListOptions{LabelSelector: "custom=true"}
		result, err := toListOptions(opts, &u, true)
		require.NoError(t, err)
		assert.Contains(t, result.LabelSelector, "custom=true")
		assert.Contains(t, result.LabelSelector, CloudCredentialLabel+"=true")
	})

	t.Run("admin with nil options gets empty options", func(t *testing.T) {
		t.Parallel()
		u := k8suser.DefaultInfo{Name: "admin"}
		result, err := toListOptions(&metav1.ListOptions{}, &u, true)
		require.NoError(t, err)
		assert.Contains(t, result.LabelSelector, CloudCredentialLabel+"=true")
	})

	t.Run("non-admin with no options gets owner selector", func(t *testing.T) {
		t.Parallel()
		u := k8suser.DefaultInfo{Name: "user-1"}
		opts := &metav1.ListOptions{}
		result, err := toListOptions(opts, &u, false)
		require.NoError(t, err)
		assert.Contains(t, result.LabelSelector, CloudCredentialOwnerLabel+"=user-1")
	})

	t.Run("non-admin with nil options gets owner selector", func(t *testing.T) {
		t.Parallel()
		u := k8suser.DefaultInfo{Name: "user-1"}
		result, err := toListOptions(&metav1.ListOptions{}, &u, false)
		require.NoError(t, err)
		assert.Contains(t, result.LabelSelector, CloudCredentialOwnerLabel+"=user-1")
	})

	t.Run("system users get sanitized owner selector", func(t *testing.T) {
		t.Parallel()
		u := k8suser.DefaultInfo{Name: "system:admin"}
		result, err := toListOptions(&metav1.ListOptions{}, &u, false)
		require.NoError(t, err)
		assert.Contains(t, result.LabelSelector, CloudCredentialOwnerLabel+"="+sanitizeLabelValue(u.Name))
	})

	t.Run("non-admin with existing selector merges owner", func(t *testing.T) {
		t.Parallel()
		u := k8suser.DefaultInfo{Name: "user-1"}
		opts := &metav1.ListOptions{LabelSelector: "custom=true"}
		result, err := toListOptions(opts, &u, false)
		require.NoError(t, err)
		assert.Contains(t, result.LabelSelector, CloudCredentialOwnerLabel+"=user-1")
		assert.Contains(t, result.LabelSelector, "custom=true")
	})

	t.Run("non-admin with same owner in selector passes through", func(t *testing.T) {
		t.Parallel()
		u := k8suser.DefaultInfo{Name: "user-1"}
		opts := &metav1.ListOptions{LabelSelector: CloudCredentialOwnerLabel + "=user-1"}
		result, err := toListOptions(opts, &u, false)
		require.NoError(t, err)
		assert.Contains(t, result.LabelSelector, CloudCredentialOwnerLabel+"=user-1")
	})

	t.Run("system users keep matching explicit owner selector", func(t *testing.T) {
		t.Parallel()
		u := k8suser.DefaultInfo{Name: "other:user"}
		opts := &metav1.ListOptions{LabelSelector: CloudCredentialOwnerLabel + "=other_user"}
		result, err := toListOptions(opts, &u, false)
		require.NoError(t, err)
		assert.Contains(t, result.LabelSelector, CloudCredentialOwnerLabel+"=other_user")
		assert.NotContains(t, result.LabelSelector, CloudCredentialOwnerLabel+"=other:user")
	})

	t.Run("system users override conflicting owner selector", func(t *testing.T) {
		t.Parallel()
		u := k8suser.DefaultInfo{Name: "user:1"}
		opts := &metav1.ListOptions{LabelSelector: CloudCredentialOwnerLabel + "=other-user"}
		result, err := toListOptions(opts, &u, false)
		require.NoError(t, err)
		assert.Contains(t, result.LabelSelector, CloudCredentialOwnerLabel+"="+sanitizeLabelValue(u.Name))
		assert.NotContains(t, result.LabelSelector, CloudCredentialOwnerLabel+"=user:1")
		assert.NotContains(t, result.LabelSelector, CloudCredentialOwnerLabel+"=other-user")
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

	rows, err := cloudCredentialListToTableRows(list, printers.GenerateOptions{})
	require.NoError(t, err)
	assert.Len(t, rows, 2)
	assert.Equal(t, "cred1", rows[0].Cells[0])
	assert.Equal(t, "cred2", rows[1].Cells[0])
}
