package ldap

import (
	"testing"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/authconfigtest"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSaveLDAPConfigRetriesOnConflict(t *testing.T) {
	srv := authconfigtest.NewUpdateServer(t, map[string]any{
		"metadata": map[string]any{"name": "openldap"},
		"type":     client.OpenLdapConfigType,
	})
	srv.WriteAfterGets(1)
	secrets, created := authconfigtest.NewSecretController(t)
	p := &ldapProvider{authConfigs: srv.Client(), secrets: secrets, providerName: "openldap"}

	config := &apiv3.LdapConfig{
		AuthConfig: apiv3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "openldap"}},
		LdapFields: apiv3.LdapFields{ServiceAccountPassword: "test-password"},
	}
	require.NoError(t, p.saveLDAPConfig(config))

	authconfigtest.RequireUpdatedAfterConflict(t, srv)
	secretName := common.SecretNamePrefix("openldap", client.OpenLdapConfigType) + "-serviceaccountpassword"
	assert.Equal(t, map[string]map[string]string{secretName: {"serviceaccountpassword": "test-password"}}, created)
	assert.Equal(t, common.SecretsNamespace+":"+secretName, srv.Stored()["serviceAccountPassword"])
}
