package saml

import (
	"crypto/x509"
	"testing"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/authconfigtest"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSaveSamlConfigRetriesOnConflict(t *testing.T) {
	srv := authconfigtest.NewUpdateServer(t, map[string]any{
		"metadata": map[string]any{"name": GenericSAMLName},
		"type":     client.GenericSAMLConfigType,
	})
	srv.WriteAfterGets(1)
	secrets, created := authconfigtest.NewSecretController(t)
	s := &Provider{name: GenericSAMLName, authConfigs: srv.Client(), secrets: secrets}
	s.getSamlConfig = s.getSamlConfigFromUnstructured

	config := &apiv3.SamlConfig{
		AuthConfig: apiv3.AuthConfig{ObjectMeta: metav1.ObjectMeta{
			Name:        GenericSAMLName,
			Annotations: map[string]string{"test-annotation": "test-value"},
		}},
		SpKey: "test-key",
	}
	require.NoError(t, s.saveSamlConfig(config))

	authconfigtest.RequireUpdatedAfterConflict(t, srv)
	metadata := srv.Stored()["metadata"].(map[string]any)
	assert.Equal(t, map[string]any{
		authconfigtest.ConcurrentWriteAnnotation: "true",
		"test-annotation":                        "test-value",
	}, metadata["annotations"])
	secretName := common.SecretNamePrefix(GenericSAMLName, client.GenericSAMLConfigType) + "-spkey"
	assert.Equal(t, map[string]map[string]string{secretName: {"spkey": "test-key"}}, created)
	assert.Equal(t, common.SecretsNamespace+":"+secretName, srv.Stored()["spKey"])
}

func TestSaveSamlConfigWithLDAPRetriesOnConflict(t *testing.T) {
	originalGetLDAPConfig := getLDAPConfig
	t.Cleanup(func() {
		getLDAPConfig = originalGetLDAPConfig
	})
	getLDAPConfig = func(common.AuthProvider, string) (*apiv3.LdapConfig, *x509.CertPool, error) {
		return &apiv3.LdapConfig{
			LdapFields: apiv3.LdapFields{ServiceAccountPassword: "test-password"},
		}, nil, nil
	}

	srv := authconfigtest.NewUpdateServer(t, map[string]any{
		"metadata": map[string]any{"name": OKTAName},
		"type":     client.OKTAConfigType,
	})
	srv.WriteAfterGets(1)
	secrets, created := authconfigtest.NewSecretController(t)
	s := &Provider{
		name:         OKTAName,
		authConfigs:  srv.Client(),
		secrets:      secrets,
		ldapProvider: &mockLdapProvider{providerName: OKTAName},
	}
	s.getSamlConfig = s.getSamlConfigFromUnstructured

	config := &apiv3.SamlConfig{
		AuthConfig: apiv3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: OKTAName}},
		SpKey:      "test-key",
	}
	require.NoError(t, s.saveSamlConfig(config))

	authconfigtest.RequireUpdatedAfterConflict(t, srv)
	prefix := common.SecretNamePrefix(OKTAName, client.OKTAConfigType)
	assert.Equal(t, map[string]map[string]string{
		prefix + "-spkey":                  {"spkey": "test-key"},
		prefix + "-serviceaccountpassword": {"serviceaccountpassword": "test-password"},
	}, created)
	assert.Equal(t, common.SecretsNamespace+":"+prefix+"-spkey", srv.Stored()["spKey"])
	ldapConfig := srv.Stored()["openLdapConfig"].(map[string]any)
	assert.Equal(t, common.SecretsNamespace+":"+prefix+"-serviceaccountpassword", ldapConfig["serviceAccountPassword"])
}
