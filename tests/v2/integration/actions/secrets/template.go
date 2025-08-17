package secrets

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewSecretTemplate is a constructor that creates the secret template for secrets
func NewSecretTemplate(secretName, namespace string, data map[string][]byte, secretType corev1.SecretType, labels, annotations map[string]string) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        secretName,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Type: secretType,
		Data: data,
	}
}
