package cloudcredential

import (
	"context"
	"fmt"
	"strings"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	extcommon "github.com/rancher/rancher/pkg/ext/common"
	rancherfeatures "github.com/rancher/rancher/pkg/features"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
)

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

	userInfo, _, err := s.userFrom(ctx, "create")
	if err != nil {
		return nil, err
	}

	return s.SystemStore.Create(ctx, credential, options, userInfo.GetName())
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

// ensureNamespace ensures that the namespace for storing credential secrets exists.
func (s *SystemStore) ensureNamespace() error {
	return extcommon.EnsureNamespace(s.namespaceCache, s.namespaceClient, CredentialNamespace)
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
