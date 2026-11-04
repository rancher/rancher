package github

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

func TestSaveGithubConfigRetriesOnConflict(t *testing.T) {
	srv := authconfigtest.NewUpdateServer(t, map[string]any{
		"metadata": map[string]any{"name": "github"},
		"type":     client.GithubConfigType,
	})
	srv.WriteAfterGets(1)
	secrets, created := authconfigtest.NewSecretController(t)
	p := &Provider{authConfigs: srv.Client(), secrets: secrets}

	config := &apiv3.GithubConfig{
		AuthConfig:   apiv3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "github"}},
		ClientSecret: "test-secret",
	}
	require.NoError(t, p.saveGithubConfig(config))

	authconfigtest.RequireUpdatedAfterConflict(t, srv)
	secretName := common.SecretNamePrefix("github", client.GithubConfigType) + "-clientsecret"
	assert.Equal(t, map[string]map[string]string{secretName: {"clientsecret": "test-secret"}}, created)
	assert.Equal(t, common.SecretsNamespace+":"+secretName, srv.Stored()["clientSecret"])
}
