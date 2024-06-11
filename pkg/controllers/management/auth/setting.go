package auth

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	"github.com/rancher/rancher/pkg/auth/providers/azure"
	"github.com/rancher/rancher/pkg/auth/userretention"
	"github.com/rancher/rancher/pkg/crondaemon"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
)

const authSettingController = "mgmt-auth-settings-controller"

type SettingController struct {
	ensureUserRetentionLabels func() error
	scheduleUserRetention     func(string) error
}

func newAuthSettingController(ctx context.Context, mgmt *config.ManagementContext) *SettingController {
	userRetention := userretention.New(mgmt.Wrangler)
	userRetentionDaemon := crondaemon.New(ctx, "userretention", userRetention.Run)
	userRetentionLabeler := userretention.NewUserLabeler(ctx, mgmt.Wrangler)

	return &SettingController{
		ensureUserRetentionLabels: userRetentionLabeler.EnsureForAll,
		scheduleUserRetention:     userRetentionDaemon.Schedule,
	}
}

// sync is called periodically and on real updates
func (c *SettingController) sync(key string, obj *v3.Setting) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}

	switch obj.Name {
	case settings.AuthUserInfoResyncCron.Name:
		providerrefresh.UpdateRefreshCronTime(obj.Value)
	case settings.AuthUserInfoMaxAgeSeconds.Name:
		providerrefresh.UpdateRefreshMaxAge(obj.Value)
	case settings.AzureGroupCacheSize.Name:
		azure.UpdateGroupCacheSize(obj.Value)
	case settings.UserRetentionCron.Name:
		if err := c.scheduleUserRetention(obj.Value); err != nil {
			logrus.Errorf("Failed to schedule user retention daemon: %v", err)
		}
	case settings.DisableInactiveUserAfter.Name,
		settings.DeleteInactiveUserAfter.Name,
		settings.UserLastLoginDefault.Name:
		if err := c.ensureUserRetentionLabels(); err != nil {
			return nil, fmt.Errorf("error updating retention labels for users: %w", err)
		}
	}

	return nil, nil
}
