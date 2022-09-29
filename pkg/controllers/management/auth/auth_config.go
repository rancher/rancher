package auth

import (
	"context"

	"github.com/rancher/rancher/pkg/auth/cleanup"
	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
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
	if !obj.Enabled {
		err := ac.cleanup.TriggerCleanup(obj)
		if err != nil {
			logrus.Errorf("Error on cleanup of auth provider resources: %v", err)
			return obj, err
		}
	}

	return obj, nil
}
