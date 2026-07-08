package clusterregistrationtoken

import (
	"fmt"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	corev1 "k8s.io/api/core/v1"
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
		data["expiresAt"] = []byte(expiresAt)
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
