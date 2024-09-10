package secrets

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/catalog/utils"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CleanupClientSecrets tries to delete common client secrets for each auth provider.
func CleanupClientSecrets(secretInterface corev1.SecretInterface, config *v3.AuthConfig) error {
	if config == nil {
		return fmt.Errorf("cannot delete auth provider secrets if its config is nil")
	}

	fields, ok := TypeToFields[config.Type]
	if !ok {
		return fmt.Errorf("cannot delete auth provider %s because it's unknown to Rancher", config.Type)
	}

	var result error
	for _, field := range fields {
		err := common.DeleteSecret(secretInterface, config.Type, field)
		if err != nil && !apierrors.IsNotFound(err) {
			result = multierror.Append(result, err)
		}
	}

	if utils.Contains(tokens.PerUserCacheProviders, config.Name) {
		err := CleanupOAuthTokens(secretInterface, config.Name)
		if err != nil {
			result = multierror.Append(result, err)
		}
	}

	if fieldsMap, ok := SubTypeToFields[config.Type]; ok {
		for _, slice := range fieldsMap {
			for _, field := range slice {
				err := common.DeleteSecret(secretInterface, config.Type, field)
				if err != nil && !apierrors.IsNotFound(err) {
					result = multierror.Append(result, err)
				}
			}
		}
	}

	for _, secretName := range NameToFields[config.Name] {
		err := common.DeleteSecret(secretInterface, config.Name, secretName)
		if err != nil && !apierrors.IsNotFound(err) {
			result = multierror.Append(result, err)
		}
	}
	return result
}

// CleanupOAuthTokens attempts to delete all secrets that contain users' OAuth access tokens.
// It is not possible to filter secrets easily by presence of specific key(s) in the data object. The method fetches all
// Opaque secrets in the relevant namespace and looks at the target key in the data to find a secret that stores a user's
// access token to delete.
func CleanupOAuthTokens(secretInterface corev1.SecretInterface, key string) error {
	opaqueSecrets, err := secretInterface.ListNamespaced(tokens.SecretNamespace, metav1.ListOptions{FieldSelector: "type=Opaque"})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to fetch secrets to clean up: %w", err)
	}

	var result error
	for i := range opaqueSecrets.Items {
		secret := &opaqueSecrets.Items[i]
		if _, keyPresent := secret.Data[key]; keyPresent {
			err := secretInterface.DeleteNamespaced(tokens.SecretNamespace, secret.Name, &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				result = multierror.Append(result, err)
			}
		}
	}

	return result
}
