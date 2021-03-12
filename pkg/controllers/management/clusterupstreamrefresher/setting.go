package clusterupstreamrefresher

import (
	"context"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/robfig/cron"
)

const (
	clusterUpstreamRefreshSettingController = "cluster-upstream-refresh-cron-settings-controller"
)

func Register(ctx context.Context, wContext *wrangler.Context) {
	wContext.Mgmt.Setting().OnChange(ctx, clusterUpstreamRefreshSettingController, sync)
}

func sync(key string, obj *v3.Setting) (*v3.Setting, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}

	if obj.Name != settings.ClusterUpstreamRefreshCron.Name {
		return obj, nil
	}

	if obj.Value == "" {
		return obj, nil
	}
	schedule, err := cron.ParseStandard(obj.Value)
	if err != nil {
		return obj, err
	}

	clusterUpstreamRefresher.refreshCronJob.Stop()
	clusterUpstreamRefresher.refreshCronJob = cron.New()
	clusterUpstreamRefresher.refreshCronJob.Schedule(schedule, cron.FuncJob(clusterUpstreamRefresher.refreshAllUpstreamStates))
	clusterUpstreamRefresher.refreshCronJob.Start()
	return nil, nil
}
