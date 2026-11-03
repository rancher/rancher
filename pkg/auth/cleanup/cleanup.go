package cleanup

import (
	"encoding/json"
	"errors"
	"slices"

	"github.com/rancher/rancher/pkg/auth/api/secrets"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
)

var cleanupProviders = []string{
	client.GenericOIDCConfigType,
	client.CognitoConfigType,
}

const cleanedUpSecretsAnnotation = "auth.cattle.io/unused-secrets-cleaned"

// CleanupUnusedSecretTokens removes tokens from the cattle-system namespace that have
// been removed from the PerUserCacheProviders.
//
// The AuthConfig is annotated to indicate that the secrets have been cleaned.
func CleanupUnusedSecretTokens(secretsInterface wcorev1.SecretController, authConfigs v3.AuthConfigController) (cleanupErr error) {
	configs, err := authConfigs.Cache().List(labels.Everything())
	if err != nil {
		return err
	}

	for _, authConfig := range configs {
		if !slices.Contains(cleanupProviders, authConfig.Type) {
			continue
		}

		if val := authConfig.Annotations[cleanedUpSecretsAnnotation]; val == "true" {
			continue
		}

		logrus.Infof("Cleaning unused tokens from provider %s", authConfig.Name)
		if err := secrets.CleanupOAuthTokens(secretsInterface, authConfig.Name); err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
			continue
		}

		patch, err := json.Marshal(map[string]any{
			"metadata": map[string]any{
				"annotations": map[string]string{
					cleanedUpSecretsAnnotation: "true",
				},
			},
		})
		if err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
			continue
		}

		if _, err := authConfigs.Patch(authConfig.Name, types.MergePatchType, patch); err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}

	return
}
