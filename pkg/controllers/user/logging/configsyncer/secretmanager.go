package configsyncer

import (
	"reflect"

	"github.com/pkg/errors"
	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/types/config"

	k8scorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// This manager is responsible for init/update config in secret

type SecretManager struct {
	secrets v1.SecretInterface
}

func NewSecretManager(cluster *config.UserContext) *SecretManager {
	return &SecretManager{
		secrets: cluster.Core.Secrets(loggingconfig.LoggingNamespace),
	}
}

func (s *SecretManager) newSecret(name, namespace string, data map[string][]byte) (err error) {

	secret, err := s.secrets.Controller().Lister().Get(namespace, name)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if secret != nil {
		return nil
	}

	secret = &k8scorev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}
	if _, err = s.secrets.Create(secret); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func (s *SecretManager) updateSecret(name, namespace string, data map[string][]byte) error {
	existSecret, err := s.secrets.GetNamespaced(namespace, name, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "get secret %s:%s failed", namespace, name)
	}

	newSecret := existSecret.DeepCopy()
	if newSecret.Data == nil {
		newSecret.Data = make(map[string][]byte)
	}

	for k, v := range data {
		newSecret.Data[k] = v
	}

	if reflect.DeepEqual(existSecret.Data, newSecret.Data) {
		return nil
	}

	if _, err = s.secrets.Update(newSecret); err != nil {
		return errors.Wrapf(err, "update secret %s:%s failed", namespace, name)
	}
	return nil
}
