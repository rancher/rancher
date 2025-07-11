package offline

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rancher/rancher/pkg/scc/controllers/common"
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
)

type SecretManager struct {
	secretNamespace       string
	requestSecretName     string
	certificateSecretName string
	ownerRef              *metav1.OwnerReference
	secrets               v1core.SecretController
	secretCache           v1core.SecretCache
	offlineRequest        []byte
	defaultLabels         map[string]string
}

func New(
	namespace, requestName, certificateName string,
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
		ownerRef:              ownerRef,
		defaultLabels:         labels,
	}
}

func (o *SecretManager) Remove() error {
	var err error
	certErr := o.RemoveOfflineCertificate()
	requestErr := o.RemoveOfflineRequest()
	if requestErr != nil && certErr != nil {
		err = fmt.Errorf("failed to remove both offline request & certificate: %v; %v", requestErr, certErr)
	}
	if certErr != nil {
		err = fmt.Errorf("failed to remove offline certificate: %v", certErr)
	}
	if requestErr != nil {
		err = fmt.Errorf("failed to remove offline request: %v", requestErr)
	}
	return err
}

func (o *SecretManager) removeOfflineFinalizer(incomingSecret *corev1.Secret) error {
	if common.SecretHasOfflineFinalizer(incomingSecret) {
		updatedSecret := incomingSecret.DeepCopy()
		updatedSecret, _ = common.SecretRemoveOfflineFinalizer(updatedSecret)
		if _, updateErr := o.secrets.Update(updatedSecret); updateErr != nil {
			if apierrors.IsNotFound(updateErr) {
				return nil
			}

			return updateErr
		}
	}

	return nil
}
