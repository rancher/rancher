package dynamiclistener

import (
	"testing"

	"github.com/rancher/rancher/pkg/dynamiclistener"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	caCerts = `-----BEGIN CERTIFICATE-----
MIIDoDCCAoigAwIBAgIUARVYAJKrXL1Eb06sULOq2oLnPKIwDQYJKoZIhvcNAQEL
BQAwaDELMAkGA1UEBhMCVVMxDzANBgNVBAgTBk9yZWdvbjERMA8GA1UEBxMIUG9y
dGxhbmQxEzARBgNVBAoTCkt1YmVybmV0ZXMxCzAJBgNVBAsTAkNBMRMwEQYDVQQD
EwpLdWJlcm5ldGVzMB4XDTE4MDkwNzE3MDcwMFoXDTIzMDkwNjE3MDcwMFowaDEL
MAkGA1UEBhMCVVMxDzANBgNVBAgTBk9yZWdvbjERMA8GA1UEBxMIUG9ydGxhbmQx
EzARBgNVBAoTCkt1YmVybmV0ZXMxCzAJBgNVBAsTAkNBMRMwEQYDVQQDEwpLdWJl
cm5ldGVzMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA1eBwN09XYAlH
zubyFH4wt+1+9cRnvfIicZwQd9Fv6KDmQ2EpyhJx8a5okf7RErPTctm41gUtBSr+
OoBnoA2K8YzcDOlDAdjo8Pb2612VAF9idYaQe3yG6+wfBTU6eCKd3NVpzw3gJMao
h4pFHCUvUWhBC4LWh8plEI0T7B0ncDXQThKdLb+bxZu6IoBvOIYWQ6+vatkwUkOi
PC4JyQoK4M31qgfNiB0XVMdNKi99PWwj3ErnfxPj7nmHbZ9+57uHfeHJfyLH3it7
XfH4fbtfvN+IL/e/DBS0M/y2g5uaBDas3gBz1MisQ60csl3vdv1zast//GSlED8D
N21Lk5DXwQIDAQABo0IwQDAOBgNVHQ8BAf8EBAMCAQYwDwYDVR0TAQH/BAUwAwEB
/zAdBgNVHQ4EFgQUyXeIPPSg65/o2CFshNbkydwUh6YwDQYJKoZIhvcNAQELBQAD
ggEBAIcFQs7DQl4haOICbzlXmScaFD6HGysewarrAPrDDsW3U5fot1911ePyiX66
vw/yEX9wjoGrUQSUkQ9tQN9XKzXo8uROg2y2a+UP2wrkCBP4OdZ6WlQTjMwIlm7J
x33HdJl4jrbb1WBgAUQomJsQEMdDS2ck62/I4iIITmO6Udt41MFUS0dlVIXJcsss
wOknWY3c2qe9HB8Kma7R+85EcvI3p9VVHrHKTA7uSYLXWuidM8DK1Ew/2VgfMc3H
LhN5DBiStvQVA2++hahXDNlftpQXc/hZUq/Zu5U3MCG/yaFS4L1EzD5bRAkxK7Zl
XaIj5O6tpM0zOzPqmUJEsKiNxTI=
-----END CERTIFICATE-----`
	newLine = "\n"
)

type StubListerConfig struct {
	v3.ListenConfigInterface
}

type StubListenConfigLister struct {
	v3.ListenConfigLister
}

func (s *StubListenConfigLister) List(namespace string, selector labels.Selector) (ret []*v3.ListenConfig, err error) {
	return nil, nil
}

type StubSecretsInterface struct {
	v1.SecretInterface
}

type StubServerInterface struct {
	dynamiclistener.ServerInterface
}

func (s *StubServerInterface) Enable(config *v3.ListenConfig) (bool, error) {
	return true, nil
}

func testCACertIsTransformedTo(t *testing.T, original string, final string) {
	controller := &Controller{
		listenConfig:       &StubListerConfig{},
		listenConfigLister: &StubListenConfigLister{},
		secrets:            &StubSecretsInterface{},
		server:             &StubServerInterface{},
	}

	err := controller.sync("", &v3.ListenConfig{Enabled: true, CACerts: original})
	if err != nil {
		t.Error(err)
	}

	if settings.CACerts.Get() != final {
		t.Errorf("expected %v but got %v", final, settings.CACerts.Get())
	}
}

func TestCertWithNoNewLines(t *testing.T) {
	testCACertIsTransformedTo(t, caCerts, caCerts)
}

func TestCertWithOneNewLine(t *testing.T) {
	testCACertIsTransformedTo(t, caCerts+newLine, caCerts)
}

func TestCertWithManyNewLines(t *testing.T) {
	testCACertIsTransformedTo(t, caCerts+newLine+newLine+newLine, caCerts)
}

func TestCertWithPrecedingNewLine(t *testing.T) {
	testCACertIsTransformedTo(t, newLine+caCerts, caCerts)
}

func TestCertWithManyPrecedingNewLines(t *testing.T) {
	testCACertIsTransformedTo(t, newLine+newLine+newLine+caCerts, caCerts)
}

func TestCertWithPrecedingAndFollowingNewLines(t *testing.T) {
	testCACertIsTransformedTo(t, newLine+newLine+newLine+caCerts+newLine+newLine+newLine, caCerts)
}
