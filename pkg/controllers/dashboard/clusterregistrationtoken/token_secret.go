package clusterregistrationtoken

import (
	"fmt"

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

func GetTokensFromSecret(secrets SecretGetter, crt *v3.ClusterRegistrationToken) (current string, previous string, err error) {
	if crt == nil || crt.Status.TokenSecretName == "" {
		return "", "", nil
	}
	
	secret, err := secrets.Get(crt.Namespace, crt.Status.TokenSecretName)
	if err != nil {
		return "", "", fmt.Errorf("failed to get token secret %s/%s: %w", crt.Namespace, crt.Status.TokenSecretName, err)
	}

	if token, ok := secret.Data[tokenDataKey]; ok {
		current = string(token)
	}
	if prev, ok := secret.Data[previousTokenDataKey]; ok {
		previous = string(prev)
	}

	return current, previous, nil
}
