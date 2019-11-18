package setting

import (
	"fmt"
	"testing"

	v1fakes "github.com/rancher/types/apis/core/v1/fakes"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	ingressIpDomain      = "ingress-ip-domain"
	ingressIpDomainValue = "foo.bar"
	rdnsBaseURL          = "rdns-base-url"
	rdnsBaseURLValue     = "https://foo.bar.rancher.cloud/v1"
)

func TestSecretChangeOnSettingList(t *testing.T) {
	settings := []*v3.Setting{
		{ObjectMeta: metav1.ObjectMeta{
			Name: ingressIpDomain,
		}, Value: ingressIpDomainValue},
		{ObjectMeta: metav1.ObjectMeta{
			Name: rdnsBaseURL,
		}, Value: rdnsBaseURLValue},
	}

	c := Controller{
		managementSettingLister: &fakes.SettingListerMock{
			ListFunc: func(namespace string, selector labels.Selector) ([]*v3.Setting, error) {
				return settings, nil
			},
		},
		secretsLister: &v1fakes.SecretListerMock{
			GetFunc: func(namespace string, name string) (*v1.Secret, error) {
				if namespace == CattleNamespace {
					return &v1.Secret{}, nil
				}
				return nil, fmt.Errorf("invalid namespace %s", namespace)
			},
		},
		secrets: &v1fakes.SecretInterfaceMock{
			UpdateFunc: func(secret *v1.Secret) (*v1.Secret, error) {
				if value, exist := secret.StringData[ingressIpDomain]; exist {
					if value == ingressIpDomainValue {
						return secret, nil
					}
					return nil, fmt.Errorf("invalid secret data %v", secret.StringData)
				} else if value, exist = secret.StringData[rdnsBaseURL]; exist {
					if value == rdnsBaseURLValue {
						return secret, nil
					}
					return nil, fmt.Errorf("invalid secret data %v", secret.StringData)
				}
				return nil, fmt.Errorf("invalid secret %v", secret)
			},
		},
	}

	for _, s := range settings {
		err := c.createOrUpdate(s, CattleNamespace)
		assert.NoError(t, err)
	}
}
