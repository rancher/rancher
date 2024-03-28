package common

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	clientv3 "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	fake1 "github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
)

const (
	appSecretKey   = "applicationSecret"
	appSecretValue = "superSecret"
)

type testPair struct {
	in  string
	out string
}

var tests = []testPair{
	{in: "potato", out: "potato"},
	{in: SecretsNamespace + "-foo", out: SecretsNamespace + "-foo"},
	{in: SecretsNamespace + ":bar", out: appSecretValue},
	{in: "bad:thing", out: "bad:thing"},
}

func TestReadFromSecret(t *testing.T) {
	secretInterface := fake1.SecretInterfaceMock{
		GetNamespacedFunc: func(namespace string, name string, opts metav1.GetOptions) (*corev1.Secret, error) {
			s := corev1.Secret{
				Data: make(map[string][]byte),
			}
			if name == "bar" {
				s.Data[appSecretKey] = []byte(appSecretValue)
				return &s, nil
			}
			return nil, errors.New("secret not found")
		},
	}

	for _, pair := range tests {
		info, err := ReadFromSecret(&secretInterface, pair.in, appSecretKey)
		assert.Nil(t, err)
		assert.Equal(t, pair.out, info)
	}
}

func TestSavePasswordSecret(t *testing.T) {
	secrets := &secretFake{}
	secretInterface := newSecretInterfaceMock(secrets)

	name, err := SavePasswordSecret(secretInterface, "test-password",
		clientv3.LdapConfigFieldServiceAccountPassword,
		"shibbolethConfig")
	assert.NoError(t, err)

	wantSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shibbolethconfig-serviceaccountpassword",
			Namespace: "cattle-global-data",
		},
		StringData: map[string]string{
			"serviceaccountpassword": "test-password",
		},
		Type: corev1.SecretTypeOpaque,
	}
	assert.Equal(t, []*corev1.Secret{wantSecret}, secrets.Created)
	assert.Equal(t, wantSecret.Namespace+":"+wantSecret.Name, name)
}

type secretFake struct {
	Created []*corev1.Secret
}

func newSecretInterfaceMock(secrets *secretFake) v1.SecretInterface {
	controller := &fakes.SecretControllerMock{
		ListerFunc: func() v1.SecretLister {
			return &fakes.SecretListerMock{
				GetFunc: func(ns string, name string) (*corev1.Secret, error) {
					return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
				},
			}
		},
	}
	return &fakes.SecretInterfaceMock{
		ControllerFunc: func() v1.SecretController {
			return controller
		},
		CreateFunc: func(in1 *corev1.Secret) (*v1.Secret, error) {
			secrets.Created = append(secrets.Created, in1)
			return in1, nil
		},
	}
}
