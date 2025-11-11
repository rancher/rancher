package capr

import (
	"context"

	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/capr/planner"
	"github.com/rancher/rancher/pkg/controllers/capr/autoscaler"
	"github.com/rancher/rancher/pkg/controllers/capr/bootstrap"
	"github.com/rancher/rancher/pkg/controllers/capr/dynamicschema"
	"github.com/rancher/rancher/pkg/controllers/capr/machinedrain"
	"github.com/rancher/rancher/pkg/controllers/capr/machinenodelookup"
	"github.com/rancher/rancher/pkg/controllers/capr/machineprovision"
	"github.com/rancher/rancher/pkg/controllers/capr/managesystemagent"
	plannercontroller "github.com/rancher/rancher/pkg/controllers/capr/planner"
	"github.com/rancher/rancher/pkg/controllers/capr/plansecret"
	"github.com/rancher/rancher/pkg/controllers/capr/rkecluster"
	"github.com/rancher/rancher/pkg/controllers/capr/rkecontrolplane"
	"github.com/rancher/rancher/pkg/controllers/capr/unmanaged"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/provisioningv2/image"
	"github.com/rancher/rancher/pkg/provisioningv2/kubeconfig"
	"github.com/rancher/rancher/pkg/provisioningv2/prebootstrap"
	"github.com/rancher/rancher/pkg/provisioningv2/systeminfo"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
)

func EarlyRegister(ctx context.Context, clients *wrangler.Context) error {
	if features.MCM.Enabled() {
		if err := dynamicschema.Register(ctx, clients); err != nil {
			return err
		}
	}
	return nil
}

func Register(ctx context.Context, clients *wrangler.CAPIContext, kubeconfigManager *kubeconfig.Manager) error {
	rkePlanner := planner.New(ctx, clients, planner.InfoFunctions{
		ImageResolver:           image.ResolveWithControlPlane,
		ReleaseData:             capr.GetKDMReleaseData,
		SystemAgentImage:        settings.SystemAgentInstallerImage.Get,
		SystemPodLabelSelectors: systeminfo.NewRetriever(clients).GetSystemPodLabelSelectors,
		GetBootstrapManifests:   prebootstrap.NewRetriever(clients).GeneratePreBootstrapClusterAgentManifest,
	})
	if features.MCM.Enabled() {
		machineprovision.Register(ctx, clients, kubeconfigManager)
		autoscaler.Register(ctx, clients)
	}
	rkecluster.Register(ctx, clients)
	bootstrap.Register(ctx, clients)
	machinenodelookup.Register(ctx, clients, kubeconfigManager)
	plannercontroller.Register(ctx, clients, rkePlanner)
	plansecret.Register(ctx, clients)
	unmanaged.Register(ctx, clients, kubeconfigManager)
	rkecontrolplane.Register(ctx, clients)
	managesystemagent.Register(ctx, clients)
	machinedrain.Register(ctx, clients)

	return nil
}
