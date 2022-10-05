package auth

import (
	"context"

	"github.com/rancher/rancher/pkg/auth/cleanup"
	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const authConfigControllerName = "mgmt-auth-config-controller"

// CleanupService performs a cleanup of auxiliary resources belonging to a particular auth provider type.
type CleanupService interface {
	TriggerCleanup(config *v3.AuthConfig) error
}

type authConfigController struct {
	users         v3.UserLister
	authRefresher providerrefresh.UserAuthRefresher
	cleanup       CleanupService
}

func newAuthConfigController(context context.Context, mgmt *config.ManagementContext, scaledContext *config.ScaledContext) *authConfigController {
	authConfigController := &authConfigController{
		users:         mgmt.Management.Users("").Controller().Lister(),
		authRefresher: providerrefresh.NewUserAuthRefresher(context, scaledContext),
		cleanup:       cleanup.NewCleanupService(),
	}
	return authConfigController
}

func (ac *authConfigController) sync(key string, obj *v3.AuthConfig) (runtime.Object, error) {
	err := ac.refreshUsers(obj)
	if err != nil {
		return obj, err
	}
	if !obj.Enabled {
		err := ac.cleanup.TriggerCleanup(obj)
		if err != nil {
			logrus.Errorf("Error on cleanup of auth provider resources: %v", err)
			return obj, err
		}
	}

	return obj, nil
}

func (ac *authConfigController) refreshUsers(obj *v3.AuthConfig) error {
	// if we have changed an auth config, refresh all users belonging to the auth config. This addresses:
	// Disabling an auth provider - now we disable user access
	// Removing a user from auth provider access - now we will immediately revoke access
	users, err := ac.users.List("", labels.Everything())
	if err != nil {
		return err
	}
	for _, user := range users {
		principalID := providerrefresh.GetPrincipalIDForProvider(obj.Name, user)
		if principalID != "" {
			// if we have a principal on this provider, then we need to be refreshed to potentially invalidate
			// access derived from this provider
			ac.authRefresher.TriggerUserRefresh(user.Name, true)
		}
	}
	return nil
}
