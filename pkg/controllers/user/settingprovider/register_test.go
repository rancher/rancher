package settingprovider

import (
	"fmt"
	"testing"

	csetting "github.com/rancher/rancher/pkg/controllers/user/setting"
	"github.com/rancher/rancher/pkg/settings"
	v1fakes "github.com/rancher/types/apis/core/v1/fakes"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	v1b "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ingressIpDomain      = "ingress-ip-domain"
	ingressIpDomainValue = "foo.bar"
	rdnsBaseURL          = "rdns-base-url"
	rdnsBaseURLValue     = "https://foo.bar.rancher.cloud/v1"
)

func TestClusterSettingProvider(t *testing.T) {
	testSettings := make(map[string]settings.Setting, 2)
	testSettings[ingressIpDomain] = settings.Setting{
		Default: ingressIpDomainValue,
	}
	testSettings[rdnsBaseURL] = settings.Setting{
		Default: rdnsBaseURLValue,
	}

	sp := settingsProvider{
		secretsLister: &v1fakes.SecretListerMock{
			GetFunc: func(namespace string, name string) (*v1.Secret, error) {
				if namespace == csetting.CattleNamespace {
					return &v1.Secret{}, nil
				}
				return nil, fmt.Errorf("invalid namespace %s", namespace)
			},
		},
		secrets: &v1fakes.SecretInterfaceMock{
			GetNamespacedFunc: func(namespace string, name string, opts v1b.GetOptions) (*v1.Secret, error) {
				return &v1.Secret{}, nil
			},
			UpdateFunc: func(secret *v1.Secret) (*v1.Secret, error) {
				return secret, nil
			},
		},
	}

	err := sp.SetAll(testSettings)
	assert.NoError(t, err)

	domain := sp.Get(ingressIpDomain)
	assert.Equal(t, ingressIpDomainValue, domain)

	rdns := sp.Get(rdnsBaseURL)
	assert.Equal(t, rdnsBaseURLValue, rdns)
}
