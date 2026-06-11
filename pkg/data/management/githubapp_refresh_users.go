package management

import (
	"context"
	"encoding/json"

	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	"github.com/rancher/rancher/pkg/auth/providers/githubapp"
	apisv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const providerRefreshRequestedAnnotation = "auth.cattle.io/provider-refresh-requested"

// RefreshGitHubAppUsersOnce triggers a forced refresh of all user
// group principals to update team memberships.
//
// The annotation on the "githubapp" AuthConfig acts as a one-time guard
// so the function is a no-op on subsequent restarts.
func RefreshGitHubAppUsersOnce(ctx context.Context, authConfigs apisv3.AuthConfigClient) {
	authConfig, err := authConfigs.Get(githubapp.Name, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		logrus.Debugf("refreshGitHubAppUsersOnce: AuthConfig %s not found, skipping", githubapp.Name)
		return
	}
	if err != nil {
		logrus.Errorf("refreshGitHubAppUsersOnce: getting AuthConfig: %v", err)
		return
	}

	if !authConfig.Enabled {
		logrus.Debugf("refreshGitHubAppUsersOnce: provider %s is not enabled, skipping", githubapp.Name)
		return
	}

	if authConfig.Annotations[providerRefreshRequestedAnnotation] == "true" {
		logrus.Debugf("refreshGitHubAppUsersOnce: provider %s already refreshed, skipping", githubapp.Name)
		return
	}

	logrus.Infof("refreshGitHubAppUsersOnce: triggering refresh for all users of provider %s", githubapp.Name)
	providerrefresh.TriggerAllUserRefresh()

	// Use a JSON Merge Patch instead of a typed Update to set the
	// annotation. AuthConfig objects store provider-specific fields not
	// defined in the base v3.AuthConfig struct; a typed Update (PUT)
	// would strip those fields and corrupt the stored configuration.
	patch, err := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]string{
				providerRefreshRequestedAnnotation: "true",
			},
		},
	})
	if err != nil {
		logrus.Errorf("refreshGitHubAppUsersOnce: marshaling patch: %v", err)
		return
	}
	if _, err := authConfigs.Patch(githubapp.Name, types.MergePatchType, patch); err != nil {
		logrus.Warnf("refreshGitHubAppUsersOnce: patching annotation: %v", err)
	}
}
