package utils

import (
	"reflect"
	"strings"

	"github.com/pkg/errors"
	rv1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
)

func UpdateSSLAuthentication(prefix string, esConfig *v3.ElasticsearchConfig, spConfig *v3.SplunkConfig, kfConfig *v3.KafkaConfig, syslogConfig *v3.SyslogConfig, secrets rv1.SecretInterface) error {
	var certificate, clientCert, clientKey string
	if esConfig != nil {
		certificate = esConfig.Certificate
		clientCert = esConfig.ClientCert
		clientKey = esConfig.ClientKey
	} else if spConfig != nil {
		certificate = spConfig.Certificate
		clientCert = spConfig.ClientCert
		clientKey = spConfig.ClientKey
	} else if kfConfig != nil {
		certificate = kfConfig.Certificate
		clientCert = kfConfig.ClientCert
		clientKey = kfConfig.ClientKey
	} else if syslogConfig != nil {
		certificate = syslogConfig.Certificate
		clientCert = syslogConfig.ClientCert
		clientKey = syslogConfig.ClientKey
	}
	return updateSecret(loggingconfig.SSLSecretName, prefix, certificate, clientCert, clientKey, secrets)
}

func InitSecret(secrets rv1.SecretInterface) error {
	_, err := secrets.Controller().Lister().Get(loggingconfig.LoggingNamespace, loggingconfig.SSLSecretName)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if _, err := secrets.Create(newSecret(loggingconfig.LoggingNamespace, loggingconfig.SSLSecretName, make(map[string][]byte))); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

func UnsetSecret(secrets rv1.SecretInterface, name, prefix string) error {
	ncm, err := secrets.Get(name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}

	us := ncm.DeepCopy()
	newData := make(map[string][]byte)
	for k, v := range us.Data {
		if !strings.HasPrefix(k, prefix) {
			newData[k] = v
		}
	}
	us.Data = newData
	if _, err := secrets.Update(us); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func updateSecret(name, prefix, certificate, clientCert, clientKey string, secrets rv1.SecretInterface) error {
	existSecret, err := secrets.Get(name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "get secret %s:%s failed", loggingconfig.LoggingNamespace, name)
		}
		data := setSecretData(prefix, nil, certificate, clientCert, clientKey)
		if existSecret, err = secrets.Create(newSecret(loggingconfig.LoggingNamespace, name, data)); err != nil && !apierrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "create secret %s:%s failed", loggingconfig.LoggingNamespace, name)
		}
	}

	newSecret := existSecret.DeepCopy()
	newSecret.Data = setSecretData(prefix, newSecret.Data, certificate, clientCert, clientKey)
	if reflect.DeepEqual(existSecret.Data, newSecret.Data) {
		return nil
	}

	if _, err = secrets.Update(newSecret); err != nil {
		return errors.Wrapf(err, "update secret %s:%s failed", loggingconfig.LoggingNamespace, name)
	}
	return nil
}

func newSecret(namespace, name string, data map[string][]byte) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}
}

func setSecretData(prefix string, data map[string][]byte, certificate, clientCert, clientKey string) map[string][]byte {
	if data == nil {
		data = make(map[string][]byte)
	}
	data[prefix+"_"+loggingconfig.CaFileName] = []byte(certificate)
	data[prefix+"_"+loggingconfig.ClientCertName] = []byte(clientCert)
	data[prefix+"_"+loggingconfig.ClientKeyName] = []byte(clientKey)
	return data
}
