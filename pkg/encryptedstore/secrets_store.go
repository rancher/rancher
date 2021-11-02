package encryptedstore

import (
	"reflect"
	"time"

	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	defaultNamespace = "cattle-system"
)

type GenericEncryptedStore struct {
	prefix       string
	namespace    string
	secrets      v1.SecretInterface
	secretLister v1.SecretLister
}

func NewGenericEncryptedStore(prefix, namespace string, namespaceInterface v1.NamespaceInterface, secretsGetter v1.SecretsGetter) (*GenericEncryptedStore, error) {
	if namespace == "" {
		namespace = defaultNamespace
	}

	_, err := namespaceInterface.Get(namespace, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		ns := &corev1.Namespace{}
		ns.Name = namespace
		if _, err := namespaceInterface.Create(ns); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	return &GenericEncryptedStore{
		prefix:       prefix,
		namespace:    namespace,
		secrets:      secretsGetter.Secrets(namespace),
		secretLister: secretsGetter.Secrets(namespace).Controller().Lister(),
	}, nil
}

func (g *GenericEncryptedStore) Get(name string) (map[string]string, error) {
	sec, err := g.secretLister.Get(g.namespace, g.getKey(name))
	if err != nil {
		return nil, err
	}

	result := map[string]string{}
	for k, v := range sec.Data {
		result[k] = string(v)
	}

	return result, nil
}

func (g *GenericEncryptedStore) getKey(name string) string {
	return g.prefix + name
}

func (g *GenericEncryptedStore) Set(name string, data map[string]string) error {
	return g.set(name, data)
}

func (g *GenericEncryptedStore) set(name string, data map[string]string) error {
	logrus.Debugf("[GenericEncryptedStore]: set secret called for %v", g.getKey(name))
	sec, err := g.secretLister.Get(g.namespace, g.getKey(name))
	if errors.IsNotFound(err) {
		logrus.Debugf("[GenericEncryptedStore]: Creating secret for %v", g.getKey(name))
		sec = &corev1.Secret{}
		sec.Name = g.getKey(name)
		sec.StringData = data
		if _, err := g.secrets.Create(sec); err != nil {
			if !errors.IsAlreadyExists(err) {
				return err
			}
			logrus.Debugf("[GenericEncryptedStore]: secret %v already exists, updating secret", sec.Name)
			// if secret already exists, update it with the current cluster status
			return g.updateSecretWithBackoff(name, data)
		}
		return nil
	} else if err != nil {
		return err
	}

	secToUpdate := prepareSecretForUpdate(sec, data)
	if !reflect.DeepEqual(secToUpdate.Data, sec.Data) {
		logrus.Debugf("[GenericEncryptedStore]: updating secret %v", g.getKey(name))
		_, err = g.secrets.Update(secToUpdate)
		if err != nil {
			if !errors.IsConflict(err) {
				return err
			}
			return g.updateSecretWithBackoff(name, data)
		}
	}
	return err
}

func (g *GenericEncryptedStore) updateSecretWithBackoff(name string, data map[string]string) error {
	backoff := wait.Backoff{
		Duration: 100 * time.Millisecond,
		Factor:   1,
		Jitter:   0,
		Steps:    5,
	}
	return wait.ExponentialBackoff(backoff, func() (bool, error) {
		// fetch secret from the db when retrying due to IsConflict/IsAlreadyExists error
		secret, err := g.secrets.GetNamespaced(g.namespace, g.getKey(name), metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("[GenericEncryptedStore]: error getting secret %v from db: %v", g.getKey(name), err)
			return false, err
		}
		secToUpdate := prepareSecretForUpdate(secret, data)
		if !reflect.DeepEqual(secToUpdate.Data, secret.Data) {
			_, err = g.secrets.Update(secToUpdate)
			if err != nil {
				if errors.IsConflict(err) {
					logrus.Errorf("[GenericEncryptedStore]: conflict error updating secret %v: %v, retrying update", g.getKey(name), err)
					return false, nil
				}
				logrus.Errorf("[GenericEncryptedStore]: error when updating secret %v: %v", g.getKey(name), err)
				return false, err
			}
			logrus.Debugf("[GenericEncryptedStore]: successfully updated secret %v ", g.getKey(name))
		}
		return true, nil
	})
}

func prepareSecretForUpdate(secret *corev1.Secret, data map[string]string) *corev1.Secret {
	secToUpdate := secret.DeepCopy()
	if secToUpdate.Data == nil {
		secToUpdate.Data = map[string][]byte{}
	}
	for k, v := range data {
		secToUpdate.Data[k] = []byte(v)
	}
	return secToUpdate
}

func (g *GenericEncryptedStore) Remove(name string) error {
	err := g.secrets.Delete(g.getKey(name), nil)
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}
