package cleanup

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/rancher/rancher/pkg/auth/api/secrets"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/types"
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

		var patch []byte
		if authConfig.Annotations != nil {
			patch, err = json.Marshal([]jsonPatch{{
				Op:    "add",
				Path:  "/metadata/annotations/" + strings.ReplaceAll(cleanedUpSecretsAnnotation, "/", "~1"),
				Value: "true",
			}})
		} else {
			patch, err = json.Marshal([]jsonPatch{{
				Op:   "add",
				Path: "/metadata/annotations",
				Value: map[string]string{
					cleanedUpSecretsAnnotation: "true",
				},
			}})
		}
		if err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
			continue
		}

		if _, err := authConfigs.Patch(name, types.JSONPatchType, patch); err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}

	return
}

type jsonPatch struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}
