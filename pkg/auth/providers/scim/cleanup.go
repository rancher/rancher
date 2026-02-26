package scim

import (
	"fmt"

	apisv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func Cleanup(secrets wcorev1.SecretController, groups apisv3.GroupClient, provider string) error {
	labelSet := labels.Set{
		secretKindLabel:   scimAuthToken,
		authProviderLabel: provider,
	}

	secretsList, err := secrets.List(tokenSecretNamespace, metav1.ListOptions{LabelSelector: labelSet.AsSelector().String()})
	if err != nil {
		return fmt.Errorf("scim::Cleanup: failed to list token secrets for provider %s: %w", provider, err)
	}

	for _, secret := range secretsList.Items {
		err = secrets.Delete(secret.Namespace, secret.Name, &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("scim::Cleanup: failed to delete token secret %s for provider %s: %w", secret.Name, provider, err)
		}
	}

	grouplist, err := groups.List(metav1.ListOptions{LabelSelector: labels.Set{authProviderLabel: provider}.AsSelector().String()})
	if err != nil {
		return fmt.Errorf("scim::Cleanup: failed to list groups for provider %s: %w", provider, err)
	}

	for _, group := range grouplist.Items {
		err = groups.Delete(group.Name, &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("scim::Cleanup: failed to delete group %s for provider %s: %w", group.Name, provider, err)
		}
	}

	return nil
}
