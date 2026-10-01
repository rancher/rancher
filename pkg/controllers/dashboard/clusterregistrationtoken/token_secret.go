package clusterregistrationtoken

import (
	"fmt"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecretGetter is an interface for retrieving Secrets by namespace and name.
// It is satisfied by both wrangler's SecretCache and Norman's SecretLister.
type SecretGetter interface {
	Get(namespace, name string) (*corev1.Secret, error)
}

const (
	secretPrefix         = "crt-token-"
	tokenDataKey         = "token"
	previousTokenDataKey = "previousToken"
	// expiresAt and gracePeriodExpiresAt keys store the rotation schedule next to the
	// token it governs. Required to stay idempotent against duplicate rotation triggers.
	expiresAtDataKey            = "expiresAt"
	gracePeriodExpiresAtDataKey = "gracePeriodExpiresAt"
)

func SecretName(crtName string) string {
	return secretPrefix + crtName
}
func IsTokenSecret(secret *corev1.Secret) bool {
	return secret != nil && strings.HasPrefix(secret.Name, secretPrefix)
}

// SecretTokenIndexValues returns the plaintext token values stored in a CRT
// token secret, for use as index keys. Returns the current token and, if
// present, the previous token (valid during a rotation grace period).
// Returns nil for non-CRT secrets or secrets with no current token.
func SecretTokenIndexValues(secret *corev1.Secret) []string {
	if !IsTokenSecret(secret) {
		return nil
	}
	token, ok := secret.Data[tokenDataKey]
	if !ok || len(token) == 0 {
		return nil
	}
	values := []string{string(token)}
	if prev, ok := secret.Data[previousTokenDataKey]; ok && len(prev) > 0 {
		values = append(values, string(prev))
	}
	return values
}

// TokenSecretUsable reports whether the token stored in secret can still be used to register with the
// cluster named by its namespace. A cluster's token secrets live in a namespace named after the cluster,
// and that namespace is deleted with the cluster, but not waited for: a new cluster can be created under
// the same name while the old namespace, and the old cluster's tokens in it, are still being torn down.
// Nothing new can be created in a terminating namespace, so any token found in one belongs to a cluster
// that is gone.
//
// A namespace that can't be found is not treated as stale: a namespace is only removed once everything
// in it is, so the secret can't outlive it, and not finding it only means the namespace cache is behind
// the secret cache. Rejecting then would turn a lagging cache into failed registrations.
func TokenSecretUsable(secret *corev1.Secret, getNamespace func(name string) (*corev1.Namespace, error)) (bool, error) {
	ns, err := getNamespace(secret.Namespace)
	if apierrors.IsNotFound(err) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to get namespace %s of token secret %s: %w", secret.Namespace, secret.Name, err)
	}
	return ns.DeletionTimestamp == nil && ns.Status.Phase != corev1.NamespaceTerminating, nil
}

func GetTokenFromSecret(secrets SecretGetter, crt *v3.ClusterRegistrationToken) (string, error) {
	if crt == nil {
		return "", nil
	}

	if crt.Status.TokenSecretName == "" {
		return "", nil
	}

	secret, err := secrets.Get(crt.Namespace, crt.Status.TokenSecretName)
	if err != nil {
		return "", fmt.Errorf("failed to get token secret %s/%s: %w", crt.Namespace, crt.Status.TokenSecretName, err)
	}

	token, ok := secret.Data[tokenDataKey]
	if !ok {
		return "", fmt.Errorf("token key not found in secret %s/%s", crt.Namespace, crt.Status.TokenSecretName)
	}

	return string(token), nil
}

// NewTokenSecret creates a Secret object for storing a CRT's plaintext token.
func NewTokenSecret(crt *v3.ClusterRegistrationToken, token string, expiresAt string) *corev1.Secret {
	data := map[string][]byte{
		tokenDataKey: []byte(token),
	}

	if expiresAt != "" {
		data[expiresAtDataKey] = []byte(expiresAt)
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SecretName(crt.Name),
			Namespace: crt.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "management.cattle.io/v3",
					Kind:       "ClusterRegistrationToken",
					Name:       crt.Name,
					UID:        crt.UID,
				},
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}
}
