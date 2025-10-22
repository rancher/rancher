package cleanup

import (
	"errors"

	"github.com/rancher/rancher/pkg/auth/api/secrets"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
)

var cleanupProviders = []string{"genericoidc", "cognito"}

const cleanedUpSecretsAnnotation = "auth.cattle.io/unused-secrets-cleaned"

// CleanupUnusedSecretTokens removes tokens from the cattle-system namespace that have
// been removed from the PerUserCacheProviders.
//
// The AuthConfig is annotated to indicate that the secrets have been cleaned.
func CleanupUnusedSecretTokens(secretsInterface wcorev1.SecretController, authConfigs v3.AuthConfigController) (cleanupErr error) {
	for _, name := range cleanupProviders {
		authConfig, err := authConfigs.Cache().Get(name)
		if err != nil {
			logrus.Errorf("getting AuthConfig %s: %s", name, err)
			cleanupErr = errors.Join(cleanupErr, err)
			continue
		}

		if val := authConfig.Annotations[cleanedUpSecretsAnnotation]; val == "true" {
			continue
		}

		logrus.Infof("Cleaning unused tokens from provider %s", name)
		if err := secrets.CleanupOAuthTokens(secretsInterface, name); err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
			continue
		}

		authConfig = authConfig.DeepCopy()
		if authConfig.Annotations == nil {
			authConfig.Annotations = map[string]string{}
		}

		authConfig.Annotations[cleanedUpSecretsAnnotation] = "true"
		if _, err := authConfigs.Update(authConfig); err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}

	return
}
