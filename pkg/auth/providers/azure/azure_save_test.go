package azure

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

func TestSaveAzureConfigRetriesOnConflict(t *testing.T) {
	srv := authconfigtest.NewUpdateServer(t, map[string]any{
		"metadata": map[string]any{"name": "azuread"},
		"type":     client.AzureADConfigType,
	})
	srv.WriteAfterGets(1)
	secrets, created := authconfigtest.NewSecretController(t)
	ap := &Provider{
		authConfigs: srv.Client(),
		Retriever:   srv.Client().ObjectClient().UnstructuredClient(),
		secrets:     secrets,
	}

	config := &apiv3.AzureADConfig{
		AuthConfig: apiv3.AuthConfig{ObjectMeta: metav1.ObjectMeta{
			Name:        "azuread",
			Annotations: map[string]string{"test-annotation": "test-value"},
		}},
		ApplicationSecret: "test-secret",
	}
	require.NoError(t, ap.saveAzureConfigK8s(config))

	authconfigtest.RequireUpdatedAfterConflict(t, srv)
	annotations := srv.Stored()["metadata"].(map[string]any)["annotations"].(map[string]any)
	assert.Equal(t, "test-value", annotations["test-annotation"], "the retried update should keep the annotations from the request")
	secretName := common.SecretNamePrefix("azuread", client.AzureADConfigType) + "-applicationsecret"
	assert.Equal(t, map[string]map[string]string{secretName: {"applicationsecret": "test-secret"}}, created)
	assert.Equal(t, common.SecretsNamespace+":"+secretName, srv.Stored()["applicationSecret"])
}
