package secrets

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewSecretTemplate is a constructor that creates the secret template for secrets
func NewSecretTemplate(secretName, namespaceName string, data map[string][]byte, secretType corev1.SecretType) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespaceName,
		},
		Data: data,
		Type: secretType,
	}
}
