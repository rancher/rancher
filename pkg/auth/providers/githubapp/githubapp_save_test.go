package githubapp

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

func TestSaveGithubAppConfigRetriesOnConflict(t *testing.T) {
	srv := authconfigtest.NewUpdateServer(t, map[string]any{
		"metadata": map[string]any{"name": "githubapp"},
		"type":     client.GithubAppConfigType,
	})
	srv.WriteAfterGets(1)
	secrets, created := authconfigtest.NewSecretController(t)
	p := &Provider{authConfigs: srv.Client(), secrets: secrets}

	config := &apiv3.GithubAppConfig{
		AuthConfig:   apiv3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "githubapp"}},
		ClientSecret: "test-secret",
		PrivateKey:   "test-key",
	}
	require.NoError(t, p.saveGithubAppConfig(config))

	authconfigtest.RequireUpdatedAfterConflict(t, srv)
	prefix := common.SecretNamePrefix("githubapp", client.GithubAppConfigType)
	assert.Equal(t, map[string]map[string]string{
		prefix + "-clientsecret": {"clientsecret": "test-secret"},
		prefix + "-privatekey":   {"privatekey": "test-key"},
	}, created)
	assert.Equal(t, common.SecretsNamespace+":"+prefix+"-clientsecret", srv.Stored()["clientSecret"])
	assert.Equal(t, common.SecretsNamespace+":"+prefix+"-privatekey", srv.Stored()["privateKey"])
}
