package activedirectory

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

func TestSaveActiveDirectoryConfigRetriesOnConflict(t *testing.T) {
	srv := authconfigtest.NewUpdateServer(t, map[string]any{
		"metadata": map[string]any{"name": "activedirectory"},
		"type":     client.ActiveDirectoryConfigType,
	})
	srv.WriteAfterGets(1)
	secrets, created := authconfigtest.NewSecretController(t)
	p := &adProvider{authConfigs: srv.Client(), secrets: secrets}

	config := &apiv3.ActiveDirectoryConfig{
		AuthConfig:             apiv3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "activedirectory"}},
		ServiceAccountPassword: "test-password",
	}
	require.NoError(t, p.saveActiveDirectoryConfig(config))

	authconfigtest.RequireUpdatedAfterConflict(t, srv)
	secretName := common.SecretNamePrefix("activedirectory", client.ActiveDirectoryConfigType) + "-serviceaccountpassword"
	assert.Equal(t, map[string]map[string]string{secretName: {"serviceaccountpassword": "test-password"}}, created)
	assert.Equal(t, common.SecretsNamespace+":"+secretName, srv.Stored()["serviceAccountPassword"])
}
