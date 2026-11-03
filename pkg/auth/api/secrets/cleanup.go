package secrets

import (
	"errors"
	"fmt"
	"slices"

	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/name"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CleanupClientSecrets tries to delete common client secrets for each auth provider.
func CleanupClientSecrets(secretInterface wcorev1.SecretController, config *v3.AuthConfig) error {
	if config == nil {
		return fmt.Errorf("cannot delete auth provider secrets if its config is nil")
	}

	fields, ok := TypeToFields[config.Type]
	if !ok {
		return fmt.Errorf("cannot delete auth provider %s because it's unknown to Rancher", config.Type)
	}

	prefix := common.SecretNamePrefix(config.Name, config.Type)
	var result error
	for _, field := range fields {
		err := common.DeleteSecret(secretInterface, prefix, field)
		if err != nil && !apierrors.IsNotFound(err) {
			result = errors.Join(result, err)
		}
	}

	providerName := common.ProviderNameFromType(config.Type)
	if slices.Contains(tokens.PerUserCacheProviders, providerName) {
		err := CleanupOAuthTokens(secretInterface, config.Name)
		result = errors.Join(result, err)
	}

	if fieldsMap, ok := SubTypeToFields[config.Type]; ok {
		for _, slice := range fieldsMap {
			for _, field := range slice {
				err := common.DeleteSecret(secretInterface, prefix, field)
				if err != nil && !apierrors.IsNotFound(err) {
					result = errors.Join(result, err)
				}
			}
		}
	}

	for _, secretName := range NameToFields[providerName] {
		// Secrets named after the AuthConfig are shortened like the
		// Azure access token secret, see clients.AccessTokenSecretName.
		err := common.DeleteSecret(secretInterface, name.SafeConcatName(config.Name), secretName)
		if err != nil && !apierrors.IsNotFound(err) {
			result = errors.Join(result, err)
		}
	}
	return result
}

// CleanupOAuthTokens attempts to delete all secrets that contain users' OAuth access tokens.
// It is not possible to filter secrets easily by presence of specific key(s) in the data object. The method fetches all
// Opaque secrets in the relevant namespace and looks at the target key in the data to find a secret that stores a user's
// access token to delete.
func CleanupOAuthTokens(secretInterface wcorev1.SecretController, key string) error {
	opaqueSecrets, err := secretInterface.List(tokens.SecretNamespace, metav1.ListOptions{FieldSelector: "type=Opaque"})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to fetch secrets to clean up: %w", err)
	}

	var result error
	for i := range opaqueSecrets.Items {
		secret := &opaqueSecrets.Items[i]
		if _, keyPresent := secret.Data[key]; keyPresent {
			err := secretInterface.Delete(tokens.SecretNamespace, secret.Name, &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				result = errors.Join(result, err)
			}
		}
	}

	return result
}
