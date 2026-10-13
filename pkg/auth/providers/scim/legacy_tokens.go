package scim

import (
	"github.com/rancher/rancher/pkg/namespace"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// DeleteLegacyTokens deletes SCIM token secrets left in the namespace used before
// [tokenSecretNamespace]. Those secrets are no longer used.
func DeleteLegacyTokens(secrets wcorev1.SecretClient) {
	selector := labels.Set{secretKindLabel: scimAuthToken}.AsSelector().String()

	list, err := secrets.List(namespace.GlobalNamespace, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		logrus.Errorf("scim::DeleteLegacyTokens: failed to list token secrets in %s: %s", namespace.GlobalNamespace, err)
		return
	}

	for _, secret := range list.Items {
		err := secrets.Delete(namespace.GlobalNamespace, secret.Name, &metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{UID: &secret.UID},
		})
		if apierrors.IsNotFound(err) {
			continue // Already deleted, e.g. by another replica.
		}
		if err != nil {
			logrus.Errorf("scim::DeleteLegacyTokens: failed to delete token secret %s/%s: %s", namespace.GlobalNamespace, secret.Name, err)
			continue
		}

		logrus.Warnf("Deleted SCIM token secret %s/%s; SCIM tokens are now read from %s", namespace.GlobalNamespace, secret.Name, tokenSecretNamespace)
	}
}
