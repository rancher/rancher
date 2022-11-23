package auth

import (
	"context"

	"github.com/rancher/rancher/pkg/auth/cleanup"
	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	controllers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	authConfigControllerName = "mgmt-auth-config-controller"

	// CleanupAnnotation exists to prevent admins from running the cleanup routine in two scenarios:
	// 1. When the provider has not been enabled or deliberately disabled, and thus does not need cleanup.
	// 2. When the value of the annotation is 'user-locked', set manually by admins in advance.
	// Rancher will run cleanup only if the provider becomes disabled,
	// and the annotation's value is 'unlocked'.
	CleanupAnnotation = "management.cattle.io/auth-provider-cleanup"

	CleanupUnlocked      = "unlocked"
	CleanupUserLocked    = "user-locked"
	CleanupRancherLocked = "rancher-locked"
)

// CleanupService performs a cleanup of auxiliary resources belonging to a particular auth provider type.
type CleanupService interface {
	Run(config *v3.AuthConfig) error
}

type authConfigController struct {
	users            v3.UserLister
	authRefresher    providerrefresh.UserAuthRefresher
	cleanup          CleanupService
	authConfigClient controllers.AuthConfigClient
}

func newAuthConfigController(context context.Context, mgmt *config.ManagementContext, scaledContext *config.ScaledContext) *authConfigController {
	controller := &authConfigController{
		users:            mgmt.Management.Users("").Controller().Lister(),
		authRefresher:    providerrefresh.NewUserAuthRefresher(context, scaledContext),
		cleanup:          cleanup.NewCleanupService(mgmt.Core.Secrets(""), mgmt.Wrangler.Mgmt),
		authConfigClient: mgmt.Wrangler.Mgmt.AuthConfig(),
	}
	return controller
}

func (ac *authConfigController) setCleanupAnnotation(obj *v3.AuthConfig, value string) (runtime.Object, error) {
	if obj.Annotations == nil {
		obj.Annotations = make(map[string]string)
	}
	objCopy := obj.DeepCopy()
	objCopy.Annotations[CleanupAnnotation] = value
	return ac.authConfigClient.Update(objCopy)
}

func (ac *authConfigController) sync(key string, obj *v3.AuthConfig) (runtime.Object, error) {
	// If obj is nil, the auth config has been deleted. Rancher currently does not handle deletions gracefully,
	// meaning it does not perform resource cleanup. Admins should disable an auth provider instead of deleting its auth config.
	if obj == nil {
		return nil, nil
	}
	err := ac.refreshUsers(obj)
	if err != nil {
		return obj, err
	}
	value := obj.Annotations[CleanupAnnotation]
	if value == "" {
		if obj.Enabled {
			value = CleanupUnlocked
		} else {
			value = CleanupRancherLocked
		}
		return ac.setCleanupAnnotation(obj, value)
	}

	if obj.Enabled && value == CleanupRancherLocked {
		return ac.setCleanupAnnotation(obj, CleanupUnlocked)
	}

	if !obj.Enabled {
		refusalFmt := "Refusing to clean up auth provider %s because its auth config annotation %s is set to %s."

		switch value {
		case CleanupUnlocked:
			err := ac.cleanup.Run(obj)
			if err != nil {
				return obj, err
			}
			logrus.Infof("Auth provider %s has been cleaned up successfully. Locking down its cleanup operation...", obj.Name)
			// Lock the config after cleanup.
			return ac.setCleanupAnnotation(obj, CleanupRancherLocked)
		case CleanupRancherLocked:
			logrus.Infof(refusalFmt, obj.Name, CleanupAnnotation, CleanupRancherLocked)
			return obj, nil
		case CleanupUserLocked:
			logrus.Infof(refusalFmt, obj.Name, CleanupAnnotation, CleanupUserLocked)
			return obj, nil
		default:
			logrus.Infof("Refusing to clean up auth provider %s because its auth config annotation %s is invalid", obj.Name, CleanupAnnotation)
			return obj, nil
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
