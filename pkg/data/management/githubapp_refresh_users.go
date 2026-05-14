package management

import (
	"context"

	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	"github.com/rancher/rancher/pkg/auth/providers/githubapp"
	apisv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		return // Provider config is missing; nothing to do.
	}
	if err != nil {
		logrus.Errorf("refreshGitHubAppUsersOnce: getting AuthConfig: %v", err)
		return
	}

	if !authConfig.Enabled {
		return // Provider is not enabled; nothing to do.
	}

	if authConfig.Annotations[providerRefreshRequestedAnnotation] == "true" {
		return // Already ran.
	}

	providerrefresh.TriggerAllUserRefresh()

	// Set the annotation so subsequent restarts skip it.
	authConfig = authConfig.DeepCopy()
	if authConfig.Annotations == nil {
		authConfig.Annotations = map[string]string{}
	}
	authConfig.Annotations[providerRefreshRequestedAnnotation] = "true"
	if _, err := authConfigs.Update(authConfig); err != nil {
		logrus.Warnf("refreshGitHubAppUsersOnce: setting annotation: %v", err)
	}
}
