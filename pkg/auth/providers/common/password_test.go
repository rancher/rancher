package common

import (
	"testing"

	"github.com/pkg/errors"
	fake1 "github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		GetNamespacedFunc: func(namespace string, name string, opts metav1.GetOptions) (*v1.Secret, error) {
			s := v1.Secret{
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
