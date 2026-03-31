package cloudcredential

import (
	"context"
	"encoding/json"
	"fmt"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

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
	userInfo, isAdmin, err := s.userFrom(ctx, "update")
	if err != nil {
		return nil, false, err
	}

	oldSecret, err := s.GetSecret(name, request.NamespaceValue(ctx))
	if err != nil {
		return nil, false, err
	}

	if !isAdmin && oldSecret.Labels[LabelCloudCredentialOwner] != sanitizeLabelValue(userInfo.GetName()) {
		return nil, false, apierrors.NewForbidden(GVR.GroupResource(), name, fmt.Errorf("insufficient permissions to update cloud credential"))
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

	// Get owner from the existing credential annotation
	owner := oldCredential.Annotations[AnnotationCreatorID]
	if owner == "" {
		owner = oldSecret.Labels[LabelCloudCredentialOwner]
	}

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
	if namespace == metav1.NamespaceAll {
		return fmt.Errorf("failed to find cloud credential %s: %w", name, apierrors.NewNotFound(GVR.GroupResource(), name))
	}

	// Find the backing secret by CloudCredential name and namespace
	secret, err := s.GetSecret(name, namespace)
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
