package cleanup

import (
	"errors"

	"github.com/rancher/rancher/pkg/auth/api/secrets"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
)

var cleanupProviders = []string{"genericoidc", "cognito"}

// CleanupUnusedSecretTokens removes tokens from the cattle-system namespace that have
// been removed from the PerUserCacheProviders.
func CleanupUnusedSecretTokens(secretsInterface wcorev1.SecretController) (cleanupErr error) {
	for _, name := range cleanupProviders {
		logrus.Infof("Cleaning unused tokens from provider %s", name)
		if err := secrets.CleanupOAuthTokens(secretsInterface, name); err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}

	return
}
