package encryptedstore

import (
	"reflect"

	v1 "github.com/rancher/types/apis/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func NewGenericEncrypedStore(prefix, namespace string, namespaceInterface v1.NamespaceInterface, secretsGetter v1.SecretsGetter) (*GenericEncryptedStore, error) {
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
	return g.set(name, data, 0)
}

func (g *GenericEncryptedStore) set(name string, data map[string]string, try int) error {
	logrus.Infof("RAJASHREE_SECRET [set]: set secret called for %v try %v", g.getKey(name), try)
	sec, err := g.secretLister.Get(g.namespace, g.getKey(name))
	if errors.IsNotFound(err) {
		logrus.Infof("RAJASHREE_SECRET [set]: secret not found for %v try %v", g.getKey(name), try)
		sec = &corev1.Secret{}
		sec.Name = g.getKey(name)
		sec.StringData = data
		if _, err := g.secrets.Create(sec); err != nil {
			logrus.Errorf("RAJASHREE_SECRET [set]: Error creating secret for %v try %v: %v", sec.Name, try, err)
			if !errors.IsAlreadyExists(err) {
				logrus.Errorf("RAJASHREE_SECRET [set]: error when creating secret %v try %v: %v", sec.Name, try, err)
				return err
			}
		}
		logrus.Errorf("RAJASHREE_SECRET [set]: created secret for %v try %v", sec.Name, try)
		return nil
	} else if err != nil {
		logrus.Errorf("RAJASHREE_SECRET [set]: error getting secret for %v try %v: %v", sec.Name, try, err)
		return err
	}

	orig := sec.DeepCopy()
	if sec.Data == nil {
		sec.Data = map[string][]byte{}
	}
	for k, v := range data {
		logrus.Infof("RAJASHREE_SECRET [set]: filling secret %v with key %v on try %v", g.getKey(name), k, try)
		sec.Data[k] = []byte(v)
	}

	if !reflect.DeepEqual(orig, sec) {
		logrus.Infof("RAJASHREE_SECRET [set]: updating existing secret %v try %v", g.getKey(name), try)
		_, err = g.secrets.Update(sec)
		if err != nil && try < 5 {
			logrus.Errorf("RAJASHREE_SECRET [set]: error when updating secret %v on try %v: %v", sec.Name, try, err)
			return g.set(name, data, try+1)
		}
	} else {
		logrus.Infof("RAJASHREE_SECRET [set]: secrets for %v are equal on try %v", g.getKey(name), try)
	}
	if err != nil {
		logrus.Errorf("RAJASHREE_SECRET [set]: error updating secret %v on try %v: %v", sec.Name, try, err)
	}
	return err
}

func (g *GenericEncryptedStore) Remove(name string) error {
	err := g.secrets.Delete(g.getKey(name), nil)
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}
