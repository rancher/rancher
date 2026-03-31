// cloudcredential implements the store for the CloudCredential resource.
package cloudcredential

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	extcommon "github.com/rancher/rancher/pkg/ext/common"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/apiserver/pkg/authentication/user"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
)

func isSystemDataField(k string) bool {
	switch k {
	case FieldConditions, FieldVisibleFields:
		return true
	default:
		return false
	}
}

// isCloudCredentialSecret checks if a secret is a cloud credential secret based on its type
func isCloudCredentialSecret(secret *corev1.Secret) bool {
	return strings.HasPrefix(string(secret.Type), SecretTypePrefix)
}

func namespaceMatches(secret *corev1.Secret, namespace string) bool {
	if namespace == metav1.NamespaceAll {
		return true
	}
	return secret.Labels[LabelCloudCredentialNamespace] == namespace
}

// toSecret converts a CloudCredential object into the equivalent Secret resource.
// Secret layout:
//   - Type: rke.cattle.io/cloud-credential-{credentialType}
//   - Labels: cattle.io/cloud-credential=true, cattle.io/cloud-credential-name,
//     cattle.io/cloud-credential-namespace, cattle.io/cloud-credential-owner
//   - Annotations: cattle.io/cloud-credential-description, field.cattle.io/creatorId
//   - Data: field values plus visible-fields and conditions payloads
func toSecret(credential *ext.CloudCredential, owner string) (*corev1.Secret, error) {
	credType := credential.Spec.Type
	if credType == "" {
		credType = "generic"
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    CredentialNamespace,
			GenerateName: GeneratePrefix,
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
		case AnnotationDescription, AnnotationLastAppliedConfig:
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

// translateTimestampSince returns a human-readable approximation of the elapsed
// time since timestamp
func translateTimestampSince(timestamp metav1.Time) string {
	if timestamp.IsZero() {
		return "<unknown>"
	}

	return duration.HumanDuration(time.Since(timestamp.Time))
}

func toListOptions(listOptions *metav1.ListOptions, userInfo user.Info, isAdmin bool) (*metav1.ListOptions, error) {
	labelSet, err := labels.ConvertSelectorToLabelsMap(listOptions.LabelSelector)
	if err != nil {
		return nil, fmt.Errorf("error converting label selector: %w", err)
	}

	secretLabels := labels.Set{
		LabelCloudCredential: "true",
	}

	if !isAdmin {
		secretLabels[LabelCloudCredentialOwner] = sanitizeLabelValue(userInfo.GetName())
	}

	labelSet = labels.Merge(labelSet, secretLabels)
	listOptions.LabelSelector = labelSet.AsSelector().String()

	return listOptions, nil
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
