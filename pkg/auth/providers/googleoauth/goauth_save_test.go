package googleoauth

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

func TestSaveGoogleOAuthConfigRetriesOnConflict(t *testing.T) {
	srv := authconfigtest.NewUpdateServer(t, map[string]any{
		"metadata": map[string]any{"name": "googleoauth"},
		"type":     client.GoogleOauthConfigType,
	})
	srv.WriteAfterGets(1)
	secrets, created := authconfigtest.NewSecretController(t)
	g := &googleOauthProvider{authConfigs: srv.Client(), secrets: secrets}

	config := &apiv3.GoogleOauthConfig{
		AuthConfig:               apiv3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "googleoauth"}},
		OauthCredential:          "test-oauth-credential",
		ServiceAccountCredential: "test-service-account-credential",
	}
	require.NoError(t, g.saveGoogleOAuthConfigCR(config))

	authconfigtest.RequireUpdatedAfterConflict(t, srv)
	prefix := common.SecretNamePrefix("googleoauth", client.GoogleOauthConfigType)
	assert.Equal(t, map[string]map[string]string{
		prefix + "-oauthcredential":          {"oauthcredential": "test-oauth-credential"},
		prefix + "-serviceaccountcredential": {"serviceaccountcredential": "test-service-account-credential"},
	}, created)
	assert.Equal(t, common.SecretsNamespace+":"+prefix+"-oauthcredential", srv.Stored()["oauthCredential"])
	assert.Equal(t, common.SecretsNamespace+":"+prefix+"-serviceaccountcredential", srv.Stored()["serviceAccountCredential"])
}
