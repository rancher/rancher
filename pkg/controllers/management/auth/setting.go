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
	"k8s.io/apimachinery/pkg/runtime"
)

const authSettingController = "mgmt-auth-settings-controller"

type SettingController struct {
	providerRefreshCronTime   func(string) error
	providerRefreshMaxAge     func(string) error
	azureUpdateGroupCacheSize func(string) error
	ensureUserRetentionLabels func() error
	scheduleUserRetention     func(string) error
}

func newAuthSettingController(ctx context.Context, mgmt *config.ManagementContext) *SettingController {
	userRetention := userretention.New(mgmt.Wrangler)
	userRetentionDaemon := crondaemon.New(ctx, "userretention", userRetention.Run)
	userRetentionLabeler := userretention.NewUserLabeler(ctx, mgmt.Wrangler)

	return &SettingController{
		providerRefreshCronTime:   providerrefresh.UpdateRefreshCronTime,
		providerRefreshMaxAge:     providerrefresh.UpdateRefreshMaxAge,
		azureUpdateGroupCacheSize: azure.UpdateGroupCacheSize,
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
		if err := c.providerRefreshCronTime(obj.Value); err != nil {
			return nil, fmt.Errorf("error refreshing cron time: %v", err)
		}
	case settings.AuthUserInfoMaxAgeSeconds.Name:
		if err := c.providerRefreshMaxAge(obj.Value); err != nil {
			return nil, fmt.Errorf("error refreshing max age: %v", err)
		}
	case settings.AzureGroupCacheSize.Name:
		if err := c.azureUpdateGroupCacheSize(obj.Value); err != nil {
			return nil, fmt.Errorf("error updating group cache size with azure: %v", err)
		}
	case settings.UserRetentionCron.Name:
		if err := c.scheduleUserRetention(obj.Value); err != nil {
			return nil, fmt.Errorf("error scheduling user retention daemon: %v", err)
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
