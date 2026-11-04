package management

import (
	"context"
	"encoding/json"

	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	clientv3 "github.com/rancher/rancher/pkg/client/generated/management/v3"
	apisv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const providerRefreshRequestedAnnotation = "auth.cattle.io/provider-refresh-requested"

// RefreshGitHubAppUsersOnce triggers a forced refresh of all user
// group principals to update team memberships for all githubAppConfig providers.
//
// The annotation on each AuthConfig acts as a one-time guard so already-
// processed configs are skipped on subsequent restarts.
func RefreshGitHubAppUsersOnce(ctx context.Context, authConfigs apisv3.AuthConfigClient) {
	configs, err := authConfigs.List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("refreshGitHubAppUsersOnce: listing AuthConfigs: %v", err)
		return
	}

	// Use a JSON Merge Patch to set the annotation; a typed Update (PUT) would
	// strip provider-specific fields not defined in the base AuthConfig struct.
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

	triggered := false
	for _, authConfig := range configs.Items {
		if authConfig.Type != clientv3.GithubAppConfigType {
			continue
		}
		if !authConfig.Enabled {
			logrus.Debugf("refreshGitHubAppUsersOnce: provider %s is not enabled, skipping", authConfig.Name)
			continue
		}
		if authConfig.Annotations[providerRefreshRequestedAnnotation] == "true" {
			logrus.Debugf("refreshGitHubAppUsersOnce: provider %s already refreshed, skipping", authConfig.Name)
			continue
		}

		if !triggered {
			logrus.Infof("refreshGitHubAppUsersOnce: triggering refresh for all users")
			providerrefresh.TriggerAllUserRefresh()
			triggered = true
		}

		if _, err := authConfigs.Patch(authConfig.Name, types.MergePatchType, patch); err != nil {
			logrus.Warnf("refreshGitHubAppUsersOnce: patching annotation on %s: %v", authConfig.Name, err)
		}
	}
}
