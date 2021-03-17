package multiclusterapp

import (
	"context"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	pv3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/types/config"
	"k8s.io/apimachinery/pkg/runtime"
)

func StartMCAppEnqueueController(ctx context.Context, management *config.ManagementContext) {
	m := MCAppEnqueueController{
		mcApps: management.Management.MultiClusterApps(""),
	}
	management.Project.Apps("").AddHandler(ctx, "management-mcapp-enqueue-controller", m.sync)
}

type MCAppEnqueueController struct {
	mcApps v3.MultiClusterAppInterface
}

func (m *MCAppEnqueueController) sync(key string, app *pv3.App) (runtime.Object, error) {
	if app == nil {
		return app, nil
	}
	if mcappName, ok := app.Labels[MultiClusterAppIDSelector]; ok {
		m.mcApps.Controller().Enqueue(namespace.GlobalNamespace, mcappName)
	}
	return app, nil
}
