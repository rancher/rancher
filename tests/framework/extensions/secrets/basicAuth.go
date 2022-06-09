package secrets

import (
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewBasicAuthSecret is a constructor for a Basic Auth secret type
func NewBasicAuthSecret(name, namespace, username, password string) *coreV1.Secret {
	return &coreV1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"username": []byte(username),
			"password": []byte(password),
		},
		Type: "kubernetes.io/basic-auth",
	}
}
