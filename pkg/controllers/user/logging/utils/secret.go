package utils

import (
	"io/ioutil"
	"os"
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

func UpdateSSLAuthentication(prefix string, esConfig *v3.ElasticsearchConfig, spConfig *v3.SplunkConfig, kfConfig *v3.KafkaConfig, syslogConfig *v3.SyslogConfig, fluentForwarder *v3.FluentForwarderConfig, secrets rv1.SecretInterface) error {
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
	} else if fluentForwarder != nil {
		certificate = fluentForwarder.Certificate
	}

	data := buildCertSecret(prefix, certificate, clientCert, clientKey)
	return updateSecret(loggingconfig.SSLSecretName, data, secrets)
}

func UpdateConfig(configPath, name, level string, secrets rv1.SecretInterface) error {
	data, err := buildConfigSecret(configPath, loggingconfig.LoggingNamespace, name, level)
	if err != nil {
		return err
	}
	return updateSecret(name, data, secrets)
}

func InitSecret(secrets rv1.SecretInterface) (err error) {
	if _, err = secrets.Controller().Lister().Get(loggingconfig.LoggingNamespace, loggingconfig.SSLSecretName); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if _, err := secrets.Create(newSecret(loggingconfig.LoggingNamespace, loggingconfig.SSLSecretName, make(map[string][]byte))); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	if _, err := secrets.Controller().Lister().Get(loggingconfig.LoggingNamespace, loggingconfig.ClusterLoggingName); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if _, err = secrets.Create(newSecret(loggingconfig.LoggingNamespace, loggingconfig.ClusterLoggingName, map[string][]byte{
		"cluster.conf": []byte{},
	})); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	if _, err = secrets.Controller().Lister().Get(loggingconfig.LoggingNamespace, loggingconfig.ProjectLoggingName); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if _, err = secrets.Create(newSecret(loggingconfig.LoggingNamespace, loggingconfig.ProjectLoggingName, map[string][]byte{
		"project.conf": []byte{},
	})); err != nil && !apierrors.IsAlreadyExists(err) {
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
			break
		}
		newData[k] = []byte{}
	}
	us.Data = newData
	if _, err := secrets.Update(us); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func updateSecret(name string, data map[string][]byte, secrets rv1.SecretInterface) error {
	existSecret, err := secrets.Get(name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "get secret %s:%s failed", loggingconfig.LoggingNamespace, name)
		}
		if existSecret, err = secrets.Create(newSecret(loggingconfig.LoggingNamespace, name, data)); err != nil && !apierrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "create secret %s:%s failed", loggingconfig.LoggingNamespace, name)
		}
	}

	for k, v := range existSecret.Data {
		if _, ok := data[k]; !ok {
			data[k] = v
		}
	}

	newSecret := existSecret.DeepCopy()
	newSecret.Data = data
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

func buildCertSecret(prefix string, certificate, clientCert, clientKey string) map[string][]byte {
	return map[string][]byte{
		prefix + "_" + loggingconfig.CaFileName:     []byte(certificate),
		prefix + "_" + loggingconfig.ClientCertName: []byte(clientCert),
		prefix + "_" + loggingconfig.ClientKeyName:  []byte(clientKey),
	}
}

func buildConfigSecret(configPath, namespace, name, level string) (map[string][]byte, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return nil, errors.Wrapf(err, "find %s logging configuration file %s failed", level, configPath)
	}
	defer file.Close()
	buf, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, errors.Wrapf(err, "read %s logging configuration file %s failed", level, configPath)
	}

	return map[string][]byte{
		level + ".conf": buf,
	}, nil
}
