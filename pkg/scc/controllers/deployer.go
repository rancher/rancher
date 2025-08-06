package controllers

import (
	"context"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/scc/consts"
	"github.com/rancher/rancher/pkg/scc/deployer"
	"github.com/rancher/rancher/pkg/scc/util/generic"
	"github.com/rancher/rancher/pkg/scc/util/log"
	deploymentControllers "github.com/rancher/wrangler/v3/pkg/generated/controllers/apps/v1"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	appsv1 "k8s.io/api/apps/v1"
)

type deployersHandler struct {
	ctx         context.Context
	log         log.StructuredLogger
	deployments deploymentControllers.DeploymentController
}

func RegisterDeployer(
	ctx context.Context,
	logger log.StructuredLogger,
	sccDeployer *deployer.SCCDeployer,
	deployments deploymentControllers.DeploymentController,
	settings mgmtv3.SettingController,
) {
	controller := &deployersHandler{
		ctx:         ctx,
		log:         logger.WithField("controller", "scc-deployers"),
		deployments: deployments,
	}

	// Set up a handler for the SCCOperatorImage setting
	relatedresource.Watch(ctx, "scc-operator-image-watcher",
		sccDeployer.OnRelatedSettings,
		controller.deployments,
		settings)

	generic.NamespaceScopedOnChange(
		ctx,
		"scc-deployment",
		consts.DefaultSCCNamespace,
		controller.deployments,
		controller.OnChanged,
	)

	generic.NamespaceScopedOnRemove(
		ctx,
		"scc-deployment-delete",
		consts.DefaultSCCNamespace,
		controller.deployments,
		controller.OnRemoved,
	)
}

func (h *deployersHandler) OnChanged(name string, incoming *appsv1.Deployment) (*appsv1.Deployment, error) {
	if name != consts.DeploymentName {
		return incoming, nil
	}

	h.log.Info(incoming)

	return incoming, nil
}

func (h *deployersHandler) OnRemoved(name string, incoming *appsv1.Deployment) (*appsv1.Deployment, error) {
	if name != consts.DeploymentName {
		return incoming, nil
	}

	h.log.Debugf("Checking if %s is ready to be removed", incoming.Name)

	// TODO: we should wait to remove finalizer until after SCC CRs are cleaned up

	return incoming, nil
}
