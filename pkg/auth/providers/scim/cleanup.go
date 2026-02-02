package scim

import (
	"fmt"

	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func CleanupSecrets(secrets wcorev1.SecretController, provider string) error {
	labelSet := labels.Set(map[string]string{
		secretKindLabel:   scimAuthToken,
		authProviderLabel: provider,
	})

	list, err := secrets.List(tokenSecretNamespace, metav1.ListOptions{LabelSelector: labelSet.AsSelector().String()})
	if err != nil {
		return fmt.Errorf("scim::Cleanup: failed to list token secrets for provider %s: %w", provider, err)
	}

	for _, secret := range list.Items {
		err = secrets.Delete(secret.Namespace, secret.Name, &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("scim::Cleanup: failed to delete token secret %s for provider %s: %w", secret.Name, provider, err)
		}
	}

	return nil
}
