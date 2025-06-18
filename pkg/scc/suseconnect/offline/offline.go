package offline

import (
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SecretManager struct {
	secretNamespace       string
	requestSecretName     string
	certificateSecretName string
	finalizer             string
	ownerRef              *metav1.OwnerReference
	secrets               v1core.SecretController
	secretCache           v1core.SecretCache
	offlineRequest        []byte
	defaultLabels         map[string]string
}

func New(
	namespace, requestName, certificateName string,
	finalizer string,
	ownerRef *metav1.OwnerReference,
	secrets v1core.SecretController,
	secretCache v1core.SecretCache,
	labels map[string]string,
) *SecretManager {
	return &SecretManager{
		secretNamespace:       namespace,
		requestSecretName:     requestName,
		certificateSecretName: certificateName,
		secrets:               secrets,
		secretCache:           secretCache,
		finalizer:             finalizer,
		ownerRef:              ownerRef,
		defaultLabels:         labels,
	}
}
