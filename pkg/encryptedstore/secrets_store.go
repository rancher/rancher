package encryptedstore

import (
	"reflect"

	"github.com/rancher/types/apis/core/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	defaultNamespace = "cattle-system"
)

type GenericEncryptedStore struct {
	prefix    string
	namespace string
	secrets   typedv1.SecretsGetter
}

func NewGenericEncrypedStore(prefix, namespace string, namespaceInterface v1.NamespaceInterface, secrets typedv1.SecretsGetter) (*GenericEncryptedStore, error) {
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
		prefix:    prefix,
		namespace: namespace,
		secrets:   secrets,
	}, nil
}

func (g *GenericEncryptedStore) Get(name string) (map[string]string, error) {
	sec, err := g.secrets.Secrets(g.namespace).Get(g.getKey(name), metav1.GetOptions{})
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
	sec, err := g.secrets.Secrets(g.namespace).Get(g.getKey(name), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		sec = &corev1.Secret{}
		sec.Name = g.getKey(name)
		sec.StringData = data
		_, err := g.secrets.Secrets(g.namespace).Create(sec)
		return err
	} else if err != nil {
		return err
	}

	orig := sec.DeepCopy()
	if sec.Data == nil {
		sec.Data = map[string][]byte{}
	}
	for k, v := range data {
		sec.Data[k] = []byte(v)
	}

	if !reflect.DeepEqual(orig, sec) {
		_, err = g.secrets.Secrets(g.namespace).Update(sec)
	}
	return err
}

func (g *GenericEncryptedStore) Remove(name string) error {
	err := g.secrets.Secrets(g.namespace).Delete(g.getKey(name), nil)
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}
