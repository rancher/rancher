// cloudcredential implements the store for the CloudCredential resource.
package cloudcredential

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	extcommon "github.com/rancher/rancher/pkg/ext/common"
	rancherfeatures "github.com/rancher/rancher/pkg/features"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	steveext "github.com/rancher/steve/pkg/ext"
	v1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	wranglerName "github.com/rancher/wrangler/v3/pkg/name"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/features"
	"k8s.io/kubernetes/pkg/printers"
	printerstorage "k8s.io/kubernetes/pkg/printers/storage"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
)

const (
	// CredentialNamespace is the namespace where CloudCredential secrets are stored
	CredentialNamespace = "cattle-cloud-credentials"

	// SecretTypePrefix is the prefix for the Secret type field to identify cloud credential secrets
	SecretTypePrefix = "rke.cattle.io/cloud-credential-"

	// Labels for identifying and categorizing cloud credential secrets
	LabelCloudCredential          = "cattle.io/cloud-credential"
	LabelCloudCredentialName      = "cattle.io/cloud-credential-name"
	LabelCloudCredentialNamespace = "cattle.io/cloud-credential-namespace"
	LabelCloudCredentialOwner     = "cattle.io/cloud-credential-owner"

	// Annotations for cloud credential metadata
	AnnotationDescription = "cattle.io/cloud-credential-description"
	AnnotationOwner       = "cattle.io/cloud-credential-owner"
	AnnotationCreatorID   = "field.cattle.io/creatorId"

	// AnnotationLastAppliedConfig is the standard kubectl annotation that leaks credential data.
	// It is always stripped from both the backing secret and the returned CloudCredential.
	AnnotationLastAppliedConfig = "kubectl.kubernetes.io/last-applied-configuration"

	// GeneratePrefix is the prefix used for generated credential secret names
	GeneratePrefix = "cc-"

	// GenericTypePrefix is the prefix required for generic credential types
	GenericTypePrefix = "x-"

	// CredentialConfigSuffix is appended to the type to form the credential config schema name.
	// Must be lowercase to match how DynamicSchemas are registered by node drivers and KEv2 operators.
	CredentialConfigSuffix = "credentialconfig"

	// Data field names used in the backing secret for status information
	FieldConditions    = "conditions"
	FieldVisibleFields = "visibleFields"

	SingularName = "cloudcredential"
	PluralName   = SingularName + "s"

	// ByCloudCredentialNameIndex is the secret cache index keyed by the CloudCredential name label.
	ByCloudCredentialNameIndex = "cloudCredentialName"
)

func isSystemDataField(k string) bool {
	switch k {
	case FieldConditions, FieldVisibleFields:
		return true
	default:
		return false
	}
}

var (
	secretIndexRegistrations sync.Map

	GV = schema.GroupVersion{
		Group:   "ext.cattle.io",
		Version: "v1",
	}

	GVK = schema.GroupVersionKind{
		Group:   GV.Group,
		Version: GV.Version,
		Kind:    "CloudCredential",
	}

	GVR = schema.GroupVersionResource{
		Group:    GV.Group,
		Version:  GV.Version,
		Resource: "cloudcredentials",
	}
)

// +k8s:openapi-gen=false
// +k8s:deepcopy-gen=false

// authHandler is a helper interface hiding the details of retrieving user auth
// information from the request context. This makes these operations mockable
// for store testing.
type authHandler interface {
	UserFrom(ctx context.Context, verb string) (user.Info, bool, bool, error)
}

// Store is the interface to the cloudcredential store.
type Store struct {
	SystemStore
	auth authHandler
}

// +k8s:openapi-gen=false
// +k8s:deepcopy-gen=false

// SystemStore is the interface to the cloudcredential store used internally.
type SystemStore struct {
	namespaceClient    v1.NamespaceClient
	namespaceCache     v1.NamespaceCache
	secretClient       v1.SecretClient
	secretCache        v1.SecretCache
	indexByNameEnabled bool
	dynamicSchemaCache mgmtv3.DynamicSchemaCache
	tableConverter     rest.TableConvertor
}

// NewFromWrangler is a convenience function for creating a cloudcredential store.
func NewFromWrangler(wranglerContext *wrangler.Context, authorizer authorizer.Authorizer) *Store {
	return New(
		authorizer,
		wranglerContext.Core.Namespace(),
		wranglerContext.Core.Namespace().Cache(),
		wranglerContext.Core.Secret(),
		wranglerContext.Mgmt.DynamicSchema().Cache(),
	)
}

// New is the main constructor for cloudcredential stores.
func New(
	auth authorizer.Authorizer,
	namespaceClient v1.NamespaceClient,
	namespaceCache v1.NamespaceCache,
	secretClient v1.SecretController,
	dynamicSchemaCache mgmtv3.DynamicSchemaCache,
) *Store {
	secretCache := secretClient.Cache()
	ensureSecretIndex(secretCache)

	return &Store{
		auth: NewAuthHandler(auth),
		SystemStore: SystemStore{
			namespaceClient:    namespaceClient,
			namespaceCache:     namespaceCache,
			secretClient:       secretClient,
			secretCache:        secretCache,
			indexByNameEnabled: secretCache != nil,
			dynamicSchemaCache: dynamicSchemaCache,
			tableConverter: printerstorage.TableConvertor{
				TableGenerator: printers.NewTableGenerator().With(printHandler),
			},
		},
	}
}

func ensureSecretIndex(secretCache v1.SecretCache) {
	if secretCache == nil {
		return
	}

	cacheKey := fmt.Sprintf("%p", secretCache)
	if _, loaded := secretIndexRegistrations.LoadOrStore(cacheKey, struct{}{}); loaded {
		return
	}

	secretCache.AddIndexer(ByCloudCredentialNameIndex, cloudCredentialNameIndex)
}

func cloudCredentialNameIndex(secret *corev1.Secret) ([]string, error) {
	return []string{
		secret.Labels[LabelCloudCredentialName],
	}, nil
}

// GroupVersionKind implements [rest.GroupVersionKindProvider]
func (s *Store) GroupVersionKind(_ schema.GroupVersion) schema.GroupVersionKind {
	return GVK
}

// NamespaceScoped implements [rest.Scoper]
func (s *Store) NamespaceScoped() bool {
	return true
}

// GetSingularName implements [rest.SingularNameProvider]
func (s *Store) GetSingularName() string {
	return SingularName
}

// New implements [rest.Storage]
func (s *Store) New() runtime.Object {
	obj := &ext.CloudCredential{}
	obj.GetObjectKind().SetGroupVersionKind(GVK)
	return obj
}

// Destroy implements [rest.Storage]
func (s *Store) Destroy() {
}

// ensureNamespace ensures that the namespace for storing credential secrets exists.
func (s *SystemStore) ensureNamespace() error {
	return extcommon.EnsureNamespace(s.namespaceCache, s.namespaceClient, CredentialNamespace)
}

// Create implements [rest.Creator]
func (s *Store) Create(
	ctx context.Context,
	obj runtime.Object,
	createValidation rest.ValidateObjectFunc,
	options *metav1.CreateOptions,
) (runtime.Object, error) {
	if createValidation != nil {
		if err := createValidation(ctx, obj); err != nil {
			return obj, err
		}
	}

	credential, ok := obj.(*ext.CloudCredential)
	if !ok {
		var zeroC *ext.CloudCredential
		return nil, apierrors.NewInternalError(fmt.Errorf("expected %T but got %T", zeroC, obj))
	}

	userInfo, _, hasAccess, err := s.auth.UserFrom(ctx, "create")
	if err != nil {
		return nil, err
	}

	if !hasAccess {
		return nil, apierrors.NewForbidden(GVR.GroupResource(), "", fmt.Errorf("insufficient permissions to create cloud credentials"))
	}

	// Get the user name for ownership tracking
	owner := userInfo.GetName()

	return s.SystemStore.Create(ctx, credential, options, owner)
}

func (s *SystemStore) Create(ctx context.Context, credential *ext.CloudCredential, options *metav1.CreateOptions, owner string) (*ext.CloudCredential, error) {
	dryRun := options != nil && len(options.DryRun) > 0 && options.DryRun[0] == metav1.DryRunAll

	// Type is required
	if credential.Spec.Type == "" {
		return nil, apierrors.NewBadRequest("spec.type is required")
	}

	// Name is required for the CloudCredential
	if credential.Name == "" {
		return nil, apierrors.NewBadRequest("metadata.name is required")
	}

	// Validate the credential type against DynamicSchema
	if err := s.validateCredentialType(credential.Spec.Type); err != nil {
		return nil, err
	}

	rest.FillObjectMetaSystemFields(credential)

	if dryRun {
		// For dry run, return the credential with status populated
		credential.Status.Secret = &corev1.ObjectReference{
			Kind:      "Secret",
			Namespace: CredentialNamespace,
			Name:      GeneratePrefix + credential.Name + "-xxxxx",
		}
		// Clear credentials from response (write-only)
		credential.Spec.Credentials = nil
		return credential, nil
	}

	// Create secret from credential
	secret, err := toSecret(credential, owner)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to convert credential for storage: %w", err))
	}

	if err = s.ensureNamespace(); err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("error ensuring namespace %s: %w", CredentialNamespace, err))
	}

	newSecret, err := s.secretClient.Create(secret)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to store cloud credential: %w", err))
	}

	// Read changes back (credentials will be stripped by fromSecret)
	newCredential, err := fromSecret(newSecret, s.dynamicSchemaCache)
	if err != nil {
		// Clean up broken secret
		_ = s.secretClient.Delete(CredentialNamespace, newSecret.Name, &metav1.DeleteOptions{})
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to regenerate cloud credential %s: %w", newSecret.Name, err))
	}

	return newCredential, nil
}

// Get implements [rest.Getter]
func (s *Store) Get(
	ctx context.Context,
	name string,
	options *metav1.GetOptions,
) (runtime.Object, error) {
	userInfo, isAdmin, hasAccess, err := s.auth.UserFrom(ctx, "get")
	if err != nil {
		return nil, err
	}

	if !hasAccess {
		return nil, apierrors.NewForbidden(GVR.GroupResource(), name, fmt.Errorf("insufficient permissions to access cloud credentials"))
	}

	empty := metav1.GetOptions{}
	useCache := options == nil || *options == empty

	secret, err := s.GetSecret(name, request.NamespaceValue(ctx), options, useCache)
	if err != nil {
		return nil, err
	}

	// Non-admin users can only see their own credentials
	if !isAdmin {
		owner := secret.Annotations[AnnotationOwner]
		creatorID := secret.Annotations[AnnotationCreatorID]
		userName := userInfo.GetName()
		if owner != userName && creatorID != userName {
			return nil, apierrors.NewNotFound(GVR.GroupResource(), name)
		}
	}

	credential, err := fromSecret(secret, s.dynamicSchemaCache)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to extract cloud credential %s: %w", name, err))
	}

	return credential, nil
}

// GetSecret retrieves the backing secret for a cloud credential by name and request namespace.
// The name parameter is the CloudCredential name (stored in the label), not the Secret name.
func (s *SystemStore) GetSecret(name, namespace string, options *metav1.GetOptions, useCache bool) (*corev1.Secret, error) {
	if useCache && s.indexByNameEnabled && s.secretCache != nil {
		secrets, err := s.secretCache.GetByIndex(ByCloudCredentialNameIndex, name)
		if err == nil {
			if secret := firstMatchingCloudCredentialSecret(secrets, namespace); secret != nil {
				return secret, nil
			}
			return nil, apierrors.NewNotFound(GVR.GroupResource(), name)
		}

		logrus.WithError(err).Debugf("cloudcredential: GetSecret: index lookup failed for %q, falling back to list", name)
	}

	// Find the secret by CloudCredential name label
	labelSelector := fmt.Sprintf("%s=%s", LabelCloudCredentialName, name)
	if namespace != "" && namespace != metav1.NamespaceAll {
		labelSelector = fmt.Sprintf("%s,%s=%s", labelSelector, LabelCloudCredentialNamespace, namespace)
	}

	secrets, err := s.secretClient.List(CredentialNamespace, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to list cloud credential secrets: %w", err))
	}

	// Filter for valid cloud credential secrets (type must match our prefix)
	for i := range secrets.Items {
		secret := &secrets.Items[i]
		if namespaceMatches(secret, namespace) && isCloudCredentialSecret(secret) {
			return secret, nil
		}
	}

	return nil, apierrors.NewNotFound(GVR.GroupResource(), name)
}

// isCloudCredentialSecret checks if a secret is a cloud credential secret based on its type
func isCloudCredentialSecret(secret *corev1.Secret) bool {
	return strings.HasPrefix(string(secret.Type), SecretTypePrefix)
}

func firstMatchingCloudCredentialSecret(secrets []*corev1.Secret, namespace string) *corev1.Secret {
	for _, secret := range secrets {
		if secret == nil || secret.Namespace != CredentialNamespace {
			continue
		}
		if namespaceMatches(secret, namespace) && isCloudCredentialSecret(secret) {
			return secret
		}
	}
	return nil
}

func namespaceMatches(secret *corev1.Secret, namespace string) bool {
	if namespace == "" || namespace == metav1.NamespaceAll {
		return true
	}
	return secret.Labels[LabelCloudCredentialNamespace] == namespace
}

// NewList implements [rest.Lister]
func (s *Store) NewList() runtime.Object {
	objList := &ext.CloudCredentialList{}
	objList.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
		Group:   GV.Group,
		Version: GV.Version,
		Kind:    "CloudCredentialList",
	})
	return objList
}

// List implements [rest.Lister]
func (s *Store) List(ctx context.Context, internaloptions *metainternalversion.ListOptions) (runtime.Object, error) {
	options, err := steveext.ConvertListOptions(internaloptions)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	// Extract namespace from request context and filter by it
	namespace := request.NamespaceValue(ctx)
	if namespace != metav1.NamespaceAll {
		labelSelector := fmt.Sprintf("%s=%s", LabelCloudCredentialNamespace, namespace)
		if options.LabelSelector == "" {
			options.LabelSelector = labelSelector
		} else {
			options.LabelSelector = fmt.Sprintf("%s,%s", options.LabelSelector, labelSelector)
		}
	}

	return s.list(ctx, options)
}

func (s *Store) list(ctx context.Context, options *metav1.ListOptions) (*ext.CloudCredentialList, error) {
	userInfo, isAdmin, allowed, err := s.auth.UserFrom(ctx, "list")
	if err != nil {
		return nil, err
	}

	if !allowed {
		// return a 403 Forbidden error for unauthorized users to prevent information disclosure
		return nil, apierrors.NewForbidden(GVR.GroupResource(), "", fmt.Errorf("insufficient permissions to list cloud credentials"))

	}

	// Non-admin users are filtered by owner label at the API server level
	localOptions, err := ListOptionMerge(isAdmin, userInfo.GetName(), options)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to process list options: %w", err))
	}

	return s.SystemStore.list(&localOptions)
}

func (s *SystemStore) list(options *metav1.ListOptions) (*ext.CloudCredentialList, error) {
	// Add label selector to only get cloud credential secrets
	if options.LabelSelector == "" {
		options.LabelSelector = fmt.Sprintf("%s=true", LabelCloudCredential)
	} else {
		options.LabelSelector = fmt.Sprintf("%s,%s=true", options.LabelSelector, LabelCloudCredential)
	}

	secrets, err := s.secretClient.List(CredentialNamespace, *options)
	if err != nil {
		if apierrors.IsResourceExpired(err) || apierrors.IsGone(err) {
			return nil, apierrors.NewResourceExpired(err.Error())
		}
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to list cloud credentials: %w", err))
	}

	credentials := make([]ext.CloudCredential, 0, len(secrets.Items))
	for _, secret := range secrets.Items {
		// Double-check the secret type matches our prefix
		if !isCloudCredentialSecret(&secret) {
			continue
		}
		credential, err := fromSecret(&secret, s.dynamicSchemaCache)
		// ignore broken credentials
		if err != nil {
			continue
		}
		credentials = append(credentials, *credential)
	}

	return &ext.CloudCredentialList{
		ListMeta: metav1.ListMeta{
			ResourceVersion:    secrets.ResourceVersion,
			Continue:           secrets.Continue,
			RemainingItemCount: secrets.RemainingItemCount,
		},
		Items: credentials,
	}, nil
}

// Delete implements [rest.GracefulDeleter]
func (s *Store) Delete(
	ctx context.Context,
	name string,
	deleteValidation rest.ValidateObjectFunc,
	options *metav1.DeleteOptions,
) (runtime.Object, bool, error) {
	_, _, hasAccess, err := s.auth.UserFrom(ctx, "delete")
	if err != nil {
		return nil, false, err
	}

	secret, err := s.GetSecret(name, request.NamespaceValue(ctx), &metav1.GetOptions{}, false)
	if err != nil {
		return nil, false, err
	}

	credential, err := fromSecret(secret, s.dynamicSchemaCache)
	if err != nil {
		return nil, false, apierrors.NewInternalError(fmt.Errorf("error converting secret %s to credential: %w", name, err))
	}

	if deleteValidation != nil {
		if err := deleteValidation(ctx, credential); err != nil {
			return nil, false, err
		}
	}

	if !hasAccess {
		return nil, false, apierrors.NewForbidden(GVR.GroupResource(), name, fmt.Errorf("insufficient permissions to delete cloud credentials"))
	}

	// If an UID precondition exists and matches the credential UID, replace it with the secret's UID
	if options != nil &&
		options.Preconditions != nil &&
		options.Preconditions.UID != nil &&
		*options.Preconditions.UID == credential.UID {

		options.Preconditions.UID = &secret.UID
	}

	// Delete using the actual secret name, not the CloudCredential name
	if err := s.SystemStore.Delete(secret.Name, options); err != nil {
		return nil, false, err
	}

	return credential, true, nil
}

func (s *SystemStore) Delete(name string, options *metav1.DeleteOptions) error {
	err := s.secretClient.Delete(CredentialNamespace, name, options)
	if err == nil {
		return nil
	}
	if apierrors.IsNotFound(err) {
		return nil
	}
	return apierrors.NewInternalError(fmt.Errorf("failed to delete cloud credential %s: %w", name, err))
}

// DeleteCollection implements [rest.CollectionDeleter]
func (s *Store) DeleteCollection(
	ctx context.Context,
	deleteValidation rest.ValidateObjectFunc,
	options *metav1.DeleteOptions,
	listOptions *metainternalversion.ListOptions,
) (runtime.Object, error) {
	userInfo, isAdmin, hasAccess, err := s.auth.UserFrom(ctx, "delete")
	if err != nil {
		return nil, err
	}

	if !hasAccess {
		return nil, apierrors.NewForbidden(GVR.GroupResource(), "", fmt.Errorf("insufficient permissions to delete cloud credentials"))
	}

	convertedListOpts, err := steveext.ConvertListOptions(listOptions)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	// Non-admin users are filtered by owner label at the API server level
	localOptions, err := ListOptionMerge(isAdmin, userInfo.GetName(), convertedListOpts)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to process list options: %w", err))
	}

	credList, err := s.SystemStore.list(&localOptions)
	if err != nil {
		return nil, err
	}

	result := &ext.CloudCredentialList{
		ListMeta: credList.ListMeta,
		Items:    make([]ext.CloudCredential, 0, len(credList.Items)),
	}

	for i := range credList.Items {
		cred := &credList.Items[i]
		if deleteValidation != nil {
			if err := deleteValidation(ctx, cred); err != nil {
				return nil, err
			}
		}

		if cred.Status.Secret != nil {
			if err := s.SystemStore.Delete(cred.Status.Secret.Name, options); err != nil {
				return nil, apierrors.NewInternalError(fmt.Errorf("error deleting cloud credential %s: %w", cred.Name, err))
			}
		}

		result.Items = append(result.Items, *cred)
	}

	return result, nil
}

// Update implements [rest.Updater]
func (s *Store) Update(
	ctx context.Context,
	name string,
	objInfo rest.UpdatedObjectInfo,
	createValidation rest.ValidateObjectFunc,
	updateValidation rest.ValidateObjectUpdateFunc,
	forceAllowCreate bool,
	options *metav1.UpdateOptions,
) (runtime.Object, bool, error) {
	_, _, hasAccess, err := s.auth.UserFrom(ctx, "update")
	if err != nil {
		return nil, false, err
	}

	// Find the backing secret by CloudCredential name
	oldSecret, err := s.GetSecret(name, request.NamespaceValue(ctx), &metav1.GetOptions{}, false)
	if err != nil {
		return nil, false, err
	}

	oldCredential, err := fromSecret(oldSecret, s.dynamicSchemaCache)
	if err != nil {
		return nil, false, apierrors.NewInternalError(fmt.Errorf("error converting secret %s to credential: %w", name, err))
	}

	newObj, err := objInfo.UpdatedObject(ctx, oldCredential)
	if err != nil {
		return nil, false, apierrors.NewInternalError(fmt.Errorf("error getting updated object: %w", err))
	}

	newCredential, ok := newObj.(*ext.CloudCredential)
	if !ok {
		return nil, false, apierrors.NewBadRequest(fmt.Sprintf("invalid object type %T", newObj))
	}

	if updateValidation != nil {
		err = updateValidation(ctx, newCredential, oldCredential)
		if err != nil {
			return nil, false, apierrors.NewBadRequest(fmt.Sprintf("error validating update: %s", err))
		}
	}

	if !hasAccess {
		return nil, false, apierrors.NewForbidden(GVR.GroupResource(), oldCredential.Name, fmt.Errorf("insufficient permissions to update cloud credentials"))
	}

	// Get owner from the existing secret
	owner := oldSecret.Annotations[AnnotationOwner]

	resultCredential, err := s.SystemStore.Update(oldSecret, oldCredential, newCredential, options, owner)
	return resultCredential, false, err
}

func (s *SystemStore) Update(oldSecret *corev1.Secret, oldCredential, credential *ext.CloudCredential, options *metav1.UpdateOptions, owner string) (*ext.CloudCredential, error) {
	dryRun := options != nil && len(options.DryRun) > 0 && options.DryRun[0] == metav1.DryRunAll

	if credential.ObjectMeta.UID != oldCredential.ObjectMeta.UID {
		return nil, apierrors.NewBadRequest("meta.UID is immutable")
	}

	// Type is immutable
	if credential.Spec.Type != oldCredential.Spec.Type {
		return nil, apierrors.NewBadRequest("spec.type is immutable")
	}

	// Name is immutable
	if credential.Name != oldCredential.Name {
		return nil, apierrors.NewBadRequest("metadata.name is immutable")
	}

	// Preserve status (set by controller)
	if credential.Status.Secret == nil && oldCredential.Status.Secret != nil {
		credential.Status.Secret = oldCredential.Status.Secret
	}

	secret, err := toSecret(credential, owner)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to convert credential for storage: %w", err))
	}

	// Preserve existing credential keys that weren't overwritten in the update.
	for k, v := range oldSecret.Data {
		if !isSystemDataField(k) {
			if _, exists := secret.Data[k]; !exists {
				secret.Data[k] = v
			}
		}
	}

	// Preserve the original secret's name and resource version for update
	secret.Name = oldSecret.Name
	secret.GenerateName = ""
	secret.ResourceVersion = oldSecret.ResourceVersion
	secret.UID = oldSecret.UID

	if dryRun {
		// Clear credentials from response (write-only)
		credential.Spec.Credentials = nil
		return credential, nil
	}

	newSecret, err := s.secretClient.Update(secret)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to save updated cloud credential: %w", err))
	}

	newCredential, err := fromSecret(newSecret, s.dynamicSchemaCache)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to regenerate cloud credential: %w", err))
	}

	return newCredential, nil
}

// UpdateStatus updates the validation status fields for a cloud credential.
// The name parameter is the CloudCredential name (stored in the label), not the Secret name.
// Namespace is required so same-named credentials in different namespaces cannot collide.
func (s *SystemStore) UpdateStatus(namespace, name string, status ext.CloudCredentialStatus) error {
	if namespace == "" || namespace == metav1.NamespaceAll {
		return fmt.Errorf("failed to find cloud credential %s: %w", name, apierrors.NewNotFound(GVR.GroupResource(), name))
	}

	// Find the backing secret by CloudCredential name and namespace
	secret, err := s.GetSecret(name, namespace, &metav1.GetOptions{}, false)
	if err != nil {
		return fmt.Errorf("failed to find cloud credential %s: %w", name, err)
	}

	var patchData []map[string]any

	if len(status.Conditions) > 0 {
		conditionsJSON, err := json.Marshal(status.Conditions)
		if err != nil {
			return fmt.Errorf("failed to marshal conditions: %w", err)
		}
		patchData = append(patchData, map[string]any{
			"op":    "replace",
			"path":  "/data/" + FieldConditions,
			"value": string(conditionsJSON),
		})
	}

	if len(patchData) == 0 {
		return nil
	}

	patch, err := json.Marshal(patchData)
	if err != nil {
		return fmt.Errorf("failed to marshal patch: %w", err)
	}

	_, err = s.secretClient.Patch(CredentialNamespace, secret.Name, types.JSONPatchType, patch)
	return err
}

// Watch implements [rest.Watcher]
func (s *Store) Watch(ctx context.Context, internaloptions *metainternalversion.ListOptions) (watch.Interface, error) {
	options, err := steveext.ConvertListOptions(internaloptions)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	return s.watch(ctx, options)
}

func (s *Store) watch(ctx context.Context, options *metav1.ListOptions) (watch.Interface, error) {
	userInfo, isAdmin, hasAccess, err := s.auth.UserFrom(ctx, "watch")
	if err != nil {
		return nil, err
	}

	consumer := &watcher{
		ch: make(chan watch.Event, 100),
	}

	if !hasAccess {
		return nil, apierrors.NewForbidden(GVR.GroupResource(), "", fmt.Errorf("insufficient permissions to watch cloud credentials"))
	}

	// Non-admin users are filtered by owner label at the API server level
	localOptions, err := ListOptionMerge(isAdmin, userInfo.GetName(), options)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to process watch options: %w", err))
	}

	// Add label selector to only watch cloud credential secrets
	if localOptions.LabelSelector == "" {
		localOptions.LabelSelector = fmt.Sprintf("%s=true", LabelCloudCredential)
	} else {
		localOptions.LabelSelector = fmt.Sprintf("%s,%s=true", localOptions.LabelSelector, LabelCloudCredential)
	}

	if !features.FeatureGates().Enabled(features.WatchListClient) {
		localOptions.SendInitialEvents = nil
		localOptions.ResourceVersionMatch = ""
	}

	producer, err := s.secretClient.Watch(CredentialNamespace, localOptions)
	if err != nil {
		logrus.Errorf("cloudcredential: watch: error starting watch: %s", err)
		return nil, apierrors.NewInternalError(fmt.Errorf("cloudcredential: watch: error starting watch: %w", err))
	}

	go func() {
		defer producer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case event, more := <-producer.ResultChan():
				if !more {
					return
				}

				var obj runtime.Object
				switch event.Type {
				case watch.Bookmark:
					secret, ok := event.Object.(*corev1.Secret)
					if !ok {
						logrus.Warnf("cloudcredential: watch: expected secret got %T", event.Object)
						continue
					}
					obj = &ext.CloudCredential{
						TypeMeta: metav1.TypeMeta{
							Kind:       GVK.Kind,
							APIVersion: GV.String(),
						},
						ObjectMeta: metav1.ObjectMeta{
							ResourceVersion: secret.ResourceVersion,
						},
					}
				case watch.Added, watch.Modified, watch.Deleted:
					secret, ok := event.Object.(*corev1.Secret)
					if !ok {
						logrus.Warnf("cloudcredential: watch: expected secret got %T", event.Object)
						continue
					}
					// Double-check the secret type matches our prefix
					if !isCloudCredentialSecret(secret) {
						continue
					}
					credential, err := fromSecret(secret, s.dynamicSchemaCache)
					if err != nil {
						logrus.Errorf("cloudcredential: watch: error converting secret '%s' to credential: %s", secret.Name, err)
						continue
					}
					obj = credential
				default:
					obj = event.Object
				}

				if pushed := consumer.addEvent(watch.Event{
					Type:   event.Type,
					Object: obj,
				}); !pushed {
					return
				}
			}
		}
	}()

	return consumer, nil
}

// watcher implements [watch.Interface]
type watcher struct {
	closedLock sync.RWMutex
	closed     bool
	ch         chan watch.Event
}

func (w *watcher) Stop() {
	w.closedLock.Lock()
	defer w.closedLock.Unlock()

	if w.closed {
		return
	}

	close(w.ch)
	w.closed = true
}

func (w *watcher) ResultChan() <-chan watch.Event {
	return w.ch
}

func (w *watcher) addEvent(event watch.Event) bool {
	w.closedLock.RLock()
	defer w.closedLock.RUnlock()
	if w.closed {
		return false
	}

	w.ch <- event
	return true
}

// credentialAuth is the concrete implementation of authHandler.
type credentialAuth struct {
	authorizer authorizer.Authorizer
}

// NewAuthHandler creates a new authHandler for cloud credential authorization.
func NewAuthHandler(auth authorizer.Authorizer) authHandler {
	return &credentialAuth{authorizer: auth}
}

// UserFrom extracts user info from context and checks cloud credential access.
// Returns the user info, whether they have full access to cloudcredentials, and
// whether they have the specific verb permission.
func (a *credentialAuth) UserFrom(ctx context.Context, verb string) (user.Info, bool, bool, error) {
	userInfo, ok := request.UserFrom(ctx)
	if !ok {
		return nil, false, false, fmt.Errorf("context has no user info")
	}

	namespace := request.NamespaceValue(ctx)

	// Check for full access on cloudcredentials. This matches callers with
	// wildcard access to this resource, including both global admins and users
	// bound to the cloud-credential-administrator role.
	adminDecision, _, err := a.authorizer.Authorize(ctx, &authorizer.AttributesRecord{
		User:            userInfo,
		Verb:            "*",
		APIGroup:        GVR.Group,
		Resource:        GVR.Resource,
		Namespace:       namespace,
		ResourceRequest: true,
	})
	if err != nil {
		return nil, false, false, err
	}

	if adminDecision == authorizer.DecisionAllow {
		return userInfo, true, true, nil
	}

	// No full resource access — check if the user can perform this specific verb.
	decision, _, err := a.authorizer.Authorize(ctx, &authorizer.AttributesRecord{
		User:            userInfo,
		Verb:            verb,
		APIGroup:        GVR.Group,
		Resource:        GVR.Resource,
		Namespace:       namespace,
		ResourceRequest: true,
	})
	if err != nil {
		return nil, false, false, err
	}

	return userInfo, false, decision == authorizer.DecisionAllow, nil
}

// validateCredentialType checks that the credential type is valid.
// If the type has a corresponding DynamicSchema, it's always allowed.
// If it doesn't, it's only allowed if the GenericCloudCredentials feature is enabled
// and the type has the required "x-" prefix.
func (s *SystemStore) validateCredentialType(credType string) error {
	// If no DynamicSchema cache is available, skip schema validation
	if s.dynamicSchemaCache == nil {
		return nil
	}

	schemaName := credType + CredentialConfigSuffix
	_, err := s.dynamicSchemaCache.Get(schemaName)
	if err == nil {
		// Schema exists — type is valid
		return nil
	}

	// No DynamicSchema found — check if generic credentials are allowed
	if !rancherfeatures.GenericCloudCredentials.Enabled() {
		return apierrors.NewBadRequest(fmt.Sprintf(
			"credential type %q has no corresponding DynamicSchema and the %q feature is not enabled",
			credType, "generic-cloud-credentials"))
	}

	if !strings.HasPrefix(credType, GenericTypePrefix) {
		return apierrors.NewBadRequest(fmt.Sprintf(
			"generic credential types must be prefixed with %q, got %q", GenericTypePrefix, credType))
	}

	return nil
}

// toSecret converts a CloudCredential object into the equivalent Secret resource.
// Secret layout:
// - Type: rke.cattle.io/cloud-credential-{credentialType}
// - Labels: cattle.io/cloud-credential=true, cattle.io/cloud-credential-name
// - Annotations: cattle.io/cloud-credential-description, cattle.io/cloud-credential-owner
// - Data: {type}credentialconfig-{fieldName} for each credential field
func toSecret(credential *ext.CloudCredential, owner string) (*corev1.Secret, error) {
	credType := credential.Spec.Type
	if credType == "" {
		credType = "generic"
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    CredentialNamespace,
			GenerateName: GeneratePrefix + wranglerName.SafeConcatName(credential.Namespace, credential.Name) + "-",
		},
		Type: corev1.SecretType(SecretTypePrefix + credType),
		Data: make(map[string][]byte),
	}

	// If the credential already has a backing secret, use its name
	if credential.Status.Secret != nil && credential.Status.Secret.Name != "" {
		secret.Name = credential.Status.Secret.Name
		secret.GenerateName = ""
	}

	// Set labels
	secret.Labels = make(map[string]string)
	for k, v := range credential.Labels {
		secret.Labels[k] = v
	}
	secret.Labels[LabelCloudCredential] = "true"
	secret.Labels[LabelCloudCredentialName] = credential.Name
	if credential.Namespace != "" {
		secret.Labels[LabelCloudCredentialNamespace] = credential.Namespace
	}
	if owner != "" {
		secret.Labels[LabelCloudCredentialOwner] = sanitizeLabelValue(owner)
	}

	// Set annotations
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	for k, v := range credential.Annotations {
		secret.Annotations[k] = v
	}
	if credential.Spec.Description != "" {
		secret.Annotations[AnnotationDescription] = credential.Spec.Description
	}
	if owner != "" {
		secret.Annotations[AnnotationOwner] = owner
		secret.Annotations[AnnotationCreatorID] = owner
	}
	// Drop last-applied-configuration because it can leak credential data.
	delete(secret.Annotations, AnnotationLastAppliedConfig)

	// Copy finalizers and owner references
	secret.Finalizers = append(secret.Finalizers, credential.Finalizers...)
	secret.OwnerReferences = append(secret.OwnerReferences, credential.OwnerReferences...)

	// Store credential fields directly by field name (e.g., "accessKey", "secretKey")
	for fieldName, fieldValue := range credential.Spec.Credentials {
		secret.Data[fieldName] = []byte(fieldValue)
	}

	// Store visible fields list so we know which fields are safe to return on read
	if len(credential.Spec.VisibleFields) > 0 {
		visibleJSON, err := json.Marshal(credential.Spec.VisibleFields)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal visible fields: %w", err)
		}
		secret.Data[FieldVisibleFields] = visibleJSON
	}

	// Store conditions as JSON
	if len(credential.Status.Conditions) > 0 {
		conditionsJSON, err := json.Marshal(credential.Status.Conditions)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal conditions: %w", err)
		}
		secret.Data[FieldConditions] = conditionsJSON
	}

	// Map managed fields
	var err error
	secret.ManagedFields, err = extcommon.MapManagedFields(mapFromCredential, credential.ObjectMeta.ManagedFields)
	if err != nil {
		return nil, fmt.Errorf("failed to map credential managed-fields: %w", err)
	}

	return secret, nil
}

// fromSecret converts a Secret object into the equivalent CloudCredential resource.
// It does not populate spec.credentials: credentials are write-only and are not
// returned on get/list operations.
func fromSecret(secret *corev1.Secret, dynamicSchemaCache mgmtv3.DynamicSchemaCache) (*ext.CloudCredential, error) {
	// Extract credential type from the secret type (e.g., "rke.cattle.io/cloud-credential-amazon" -> "amazon")
	credType := strings.TrimPrefix(string(secret.Type), SecretTypePrefix)
	if credType == "" {
		return nil, fmt.Errorf("credential type missing for secret %s/%s", secret.Namespace, secret.Name)
	}

	// Get the CloudCredential name from the label
	credName := secret.Labels[LabelCloudCredentialName]
	if credName == "" {
		// Fallback to secret name if label is missing
		credName = secret.Name
	}
	if credName == "" {
		return nil, fmt.Errorf("credential name missing for secret %s/%s", secret.Namespace, secret.Name)
	}

	// Get the original namespace from the label
	credNamespace := secret.Labels[LabelCloudCredentialNamespace]

	credential := &ext.CloudCredential{
		TypeMeta: metav1.TypeMeta{
			Kind:       GVK.Kind,
			APIVersion: GV.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              credName,
			Namespace:         credNamespace,
			UID:               secret.UID,
			ResourceVersion:   secret.ResourceVersion,
			CreationTimestamp: secret.CreationTimestamp,
			DeletionTimestamp: secret.DeletionTimestamp,
			Generation:        secret.Generation,
		},
		Spec: ext.CloudCredentialSpec{
			Type: credType,
			// Credentials and VisibleFields are intentionally left empty (write-only).
			Description: secret.Annotations[AnnotationDescription],
		},
		Status: ext.CloudCredentialStatus{
			// Reference to the backing secret
			Secret: &corev1.ObjectReference{
				Kind:            "Secret",
				Namespace:       secret.Namespace,
				Name:            secret.Name,
				UID:             secret.UID,
				ResourceVersion: secret.ResourceVersion,
			},
		},
	}

	// Copy labels (excluding our internal labels)
	credential.Labels = make(map[string]string)
	for k, v := range secret.Labels {
		switch k {
		case LabelCloudCredential, LabelCloudCredentialName, LabelCloudCredentialNamespace:
			// Skip internal labels
		default:
			credential.Labels[k] = v
		}
	}

	// Copy annotations (excluding our internal annotations)
	credential.Annotations = make(map[string]string)
	for k, v := range secret.Annotations {
		switch k {
		case AnnotationDescription, AnnotationOwner, AnnotationLastAppliedConfig:
			// Skip internal annotations and security-sensitive annotations
		default:
			credential.Annotations[k] = v
		}
	}

	// Copy finalizers and owner references
	credential.Finalizers = append(credential.Finalizers, secret.Finalizers...)
	credential.OwnerReferences = append(credential.OwnerReferences, secret.OwnerReferences...)

	// Extract conditions
	if conditionsJSON, ok := secret.Data[FieldConditions]; ok && len(conditionsJSON) > 0 {
		if err := json.Unmarshal(conditionsJSON, &credential.Status.Conditions); err != nil {
			return nil, fmt.Errorf("failed to unmarshal conditions: %w", err)
		}
	}

	// Resolve public fields and populate PublicData.
	// Priority: 1) Per-credential VisibleFields override, 2) DynamicSchema publicFields
	publicData := resolvePublicData(secret, credType, dynamicSchemaCache)
	if len(publicData) > 0 {
		credential.Status.PublicData = publicData
	}

	// Map managed fields
	var err error
	credential.ObjectMeta.ManagedFields, err = extcommon.MapManagedFields(mapFromSecret, secret.ObjectMeta.ManagedFields)
	if err != nil {
		return nil, fmt.Errorf("failed to map secret managed-fields: %w", err)
	}

	return credential, nil
}

// resolvePublicData determines which credential fields are public and returns their values.
// It first checks for a per-credential VisibleFields override stored in the secret data.
// If none is set, it uses the DynamicSchema's PublicFields list if available, falling back
// to a heuristic of treating non-password fields as public for backward compatibility.
func resolvePublicData(secret *corev1.Secret, credType string, dynamicSchemaCache mgmtv3.DynamicSchemaCache) map[string]string {
	// Check for per-credential VisibleFields override first
	if visibleJSON, ok := secret.Data[FieldVisibleFields]; ok && len(visibleJSON) > 0 {
		var visibleFields []string
		if err := json.Unmarshal(visibleJSON, &visibleFields); err == nil && len(visibleFields) > 0 {
			publicData := make(map[string]string, len(visibleFields))
			for _, field := range visibleFields {
				if val, ok := secret.Data[field]; ok {
					publicData[field] = string(val)
				}
			}
			return publicData
		}
	}

	// Fall back to DynamicSchema-based resolution
	if dynamicSchemaCache == nil {
		return nil
	}

	schemaName := credType + CredentialConfigSuffix
	schema, err := dynamicSchemaCache.Get(schemaName)
	if err != nil {
		// No schema found — no public data to resolve
		return nil
	}

	publicData := make(map[string]string)

	// Prefer the explicit PublicFields list if populated on the schema.
	if len(schema.Spec.PublicFields) > 0 {
		for _, fieldName := range schema.Spec.PublicFields {
			if val, ok := secret.Data[fieldName]; ok {
				publicData[fieldName] = string(val)
			}
		}
		return publicData
	}

	// Backward compatibility: if PublicFields is not set, fall back to
	// treating non-password fields as public.
	for fieldName, field := range schema.Spec.ResourceFields {
		if field.Type == "password" {
			continue
		}
		if val, ok := secret.Data[fieldName]; ok {
			publicData[fieldName] = string(val)
		}
	}

	return publicData
}

var (
	// Field path mappings for managed fields transformation
	// Since description is now stored in annotations rather than data, we map it appropriately
	pathSecData           = fieldpath.MakePathOrDie("data")
	pathSecAnnotationDesc = fieldpath.MakePathOrDie("metadata", "annotations", AnnotationDescription)
	pathCredDescription   = fieldpath.MakePathOrDie("spec", "description")

	// secret data reported as status is dropped by the map, as is .data itself
	mapFromSecret = extcommon.MapSpec{
		pathSecData.String():           nil, // Drop secret data
		pathSecAnnotationDesc.String(): pathCredDescription,
	}

	mapFromCredential = extcommon.MapSpec{
		pathCredDescription.String(): pathSecAnnotationDesc,
	}
)

// ConvertToTable implements [rest.TableConvertor]
func (s *Store) ConvertToTable(ctx context.Context, obj runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	return s.tableConverter.ConvertToTable(ctx, obj, tableOptions)
}

// printHandler registers the table printer for CloudCredential objects.
func printHandler(h printers.PrintHandler) {
	columnDefinitions := []metav1.TableColumnDefinition{
		{Name: "Name", Type: "string", Format: "name", Description: metav1.ObjectMeta{}.SwaggerDoc()["name"]},
		{Name: "Type", Type: "string", Description: "Type is the cloud provider type"},
		{Name: "Age", Type: "string", Description: metav1.ObjectMeta{}.SwaggerDoc()["creationTimestamp"]},
		{Name: "Owner", Type: "string", Priority: 1, Description: "Owner is the user who owns this credential"},
		{Name: "Description", Type: "string", Priority: 1, Description: "Description is a human readable description of the credential"},
	}
	_ = h.TableHandler(columnDefinitions, printCloudCredentialList)
	_ = h.TableHandler(columnDefinitions, printCloudCredential)
}

// printCloudCredential prints a single CloudCredential object as a table row.
func printCloudCredential(credential *ext.CloudCredential, options printers.GenerateOptions) ([]metav1.TableRow, error) {
	owner := "<unknown>"
	if credential.Annotations != nil {
		if ownerAnnotation, ok := credential.Annotations[AnnotationOwner]; ok {
			owner = ownerAnnotation
		}
	}

	credType := credential.Spec.Type

	return []metav1.TableRow{{
		Object: runtime.RawExtension{Object: credential},
		Cells: []any{
			credential.Name,
			credType,
			translateTimestampSince(credential.CreationTimestamp),
			owner,
			credential.Spec.Description,
		},
	}}, nil
}

// printCloudCredentialList prints a list of CloudCredential objects as table rows.
func printCloudCredentialList(credentialList *ext.CloudCredentialList, options printers.GenerateOptions) ([]metav1.TableRow, error) {
	rows := make([]metav1.TableRow, 0, len(credentialList.Items))
	for i := range credentialList.Items {
		r, err := printCloudCredential(&credentialList.Items[i], options)
		if err != nil {
			return nil, err
		}
		rows = append(rows, r...)
	}
	return rows, nil
}

// translateTimestampSince returns a human-readable approximation of the elapsed
// time since timestamp
func translateTimestampSince(timestamp metav1.Time) string {
	if timestamp.IsZero() {
		return "<unknown>"
	}

	return duration.HumanDuration(time.Since(timestamp.Time))
}

// ListOptionMerge merges any external filter options with the internal filter
// (for the current user). A non-error empty result indicates that the options
// specified a filter which cannot match anything (e.g. the calling user
// requests a filter for a different user than itself).
// invalidLabelChars matches any character not allowed in Kubernetes label values.
var invalidLabelChars = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// sanitizeLabelValue replaces characters that are invalid in Kubernetes label values
// (e.g. ':' in "system:admin") with '_' to produce a valid label value.
func sanitizeLabelValue(s string) string {
	return invalidLabelChars.ReplaceAllString(s, "_")
}

func ListOptionMerge(isAdmin bool, userName string, options *metav1.ListOptions) (metav1.ListOptions, error) {

	// For admins we do not impose any additional restrictions over the requested
	if isAdmin {
		if options == nil {
			return metav1.ListOptions{}, nil
		}
		return *options, nil
	}

	// For non-admins we additionally filter the result for their own credentials
	sanitizedName := sanitizeLabelValue(userName)
	ownerSelector := labels.Set(map[string]string{
		LabelCloudCredentialOwner: sanitizedName,
	})
	empty := metav1.ListOptions{}
	if options == nil || *options == empty {
		// No external filter to contend with, just set the internal filter.
		return metav1.ListOptions{
			LabelSelector: ownerSelector.AsSelector().String(),
		}, nil
	}

	// We have to contend with an external filter, and merge ours into it.
	localOptions := *options
	callerSelector, err := labels.ConvertSelectorToLabelsMap(localOptions.LabelSelector)
	if err != nil {
		return localOptions, err
	}
	if callerSelector.Has(LabelCloudCredentialOwner) {
		// The external filter already specifies an owner
		if callerSelector[LabelCloudCredentialOwner] != sanitizedName {
			// It asks for a user other than the current.
			// We can bail now, with an empty result, as nothing can match.
			return localOptions, nil
		}
		// It asks for the current user, same as our internal filter.
	} else {
		// The external filter has nothing about the owner. Add it.
		localOptions.LabelSelector = labels.Merge(callerSelector, ownerSelector).AsSelector().String()
	}

	return localOptions, nil
}

// Interface implementations
var (
	_ rest.Creater                  = &Store{}
	_ rest.Getter                   = &Store{}
	_ rest.Lister                   = &Store{}
	_ rest.Watcher                  = &Store{}
	_ rest.GracefulDeleter          = &Store{}
	_ rest.CollectionDeleter        = &Store{}
	_ rest.Updater                  = &Store{}
	_ rest.Storage                  = &Store{}
	_ rest.Scoper                   = &Store{}
	_ rest.SingularNameProvider     = &Store{}
	_ rest.GroupVersionKindProvider = &Store{}
	_ rest.TableConvertor           = &Store{}
)
