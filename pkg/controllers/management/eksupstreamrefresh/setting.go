package eksupstreamrefresh

import (
	"context"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/robfig/cron"
)

const (
	eksUpstreamRefreshSettingController = "eks-upstream-refresh-cron-settings-controller"
)

func Register(ctx context.Context, wContext *wrangler.Context) {
	wContext.Mgmt.Setting().OnChange(ctx, eksUpstreamRefreshSettingController, sync)
}

func sync(key string, obj *v3.Setting) (*v3.Setting, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}

	if obj.Name != settings.EKSUpstreamRefreshCron.Name {
		return obj, nil
	}

	if obj.Value == "" {
		return obj, nil
	}
	schedule, err := cron.ParseStandard(obj.Value)
	if err != nil {
		return obj, err
	}

	eksUpstreamRefresher.refreshCronJob.Stop()
	eksUpstreamRefresher.refreshCronJob = cron.New()
	eksUpstreamRefresher.refreshCronJob.Schedule(schedule, cron.FuncJob(eksUpstreamRefresher.refreshAllUpstreamStates))
	eksUpstreamRefresher.refreshCronJob.Start()
	return nil, nil
}
