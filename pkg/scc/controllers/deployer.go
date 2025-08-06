package controllers

import (
	"context"
	"fmt"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/scc/deployer/params"
	"github.com/rancher/rancher/pkg/settings"
	deploymentControllers "github.com/rancher/wrangler/v3/pkg/generated/controllers/apps/v1"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"

	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/scc/consts"
	"github.com/rancher/rancher/pkg/scc/deployer"
	"github.com/rancher/rancher/pkg/scc/util/generic"
	"github.com/rancher/rancher/pkg/scc/util/log"
)

type deployersHandler struct {
	ctx         context.Context
	log         log.StructuredLogger
	sccDeployer *deployer.SCCDeployer
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
		sccDeployer: sccDeployer,
		deployments: deployments,
	}

	// Set up a handler for the SCCOperatorImage setting
	relatedresource.Watch(ctx, "scc-operator-image-watcher",
		controller.OnRelatedSettings,
		controller.deployments,
		settings,
	)

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

// OnRelatedSettings triggers changes to deployment if related settings change
func (h *deployersHandler) OnRelatedSettings(_, name string, obj runtime.Object) ([]relatedresource.Key, error) {
	if _, ok := obj.(*v3.Setting); ok {
		if name == settings.SCCOperatorImage.Name {
			return []relatedresource.Key{{
				Namespace: consts.DefaultSCCNamespace,
				Name:      consts.DeploymentName,
			}}, nil
		}
	}

	return nil, nil
}

func (h *deployersHandler) OnChanged(name string, incoming *appsv1.Deployment) (*appsv1.Deployment, error) {
	// This ensures that we only care about our deployment - any others will be ignored
	if name != fmt.Sprintf("%s/%s", consts.DefaultSCCNamespace, consts.DeploymentName) {
		return incoming, nil
	}

	h.log.Debugf("Received deployment update for %s", name)
	h.log.Debug(incoming)

	desiredSCCParams, paramsErr := params.ExtractSccOperatorParams()
	if paramsErr != nil {
		h.log.Errorf("Failed to extract SCC operator params: %v", paramsErr)
		return nil, paramsErr
	}

	// TODO: this might need to be more conditional...
	if err := h.ensureDependencies(desiredSCCParams); err != nil {
		return nil, err
	}

	currentDeployment, err := h.sccDeployer.ReconcileDeployment(h.ctx, desiredSCCParams, incoming)
	if err != nil {
		return nil, err
	}

	h.log.Debugf("Finished reconciling deployment for %s", name)
	h.log.Debug(desiredSCCParams)
	h.log.Debug(currentDeployment)

	return incoming, nil
}

func (h *deployersHandler) ensureDependencies(desiredSCCParams *params.SCCOperatorParams) error {
	hasDeps, depsErr := h.sccDeployer.HasAllDependencies()
	if depsErr != nil {
		h.log.Errorf("error checking dependencies: %v", depsErr)
	}
	if !hasDeps {
		if err := h.sccDeployer.EnsureDependenciesConfigured(h.ctx, desiredSCCParams); err != nil {
			return err
		}
	}

	return nil
}

func (h *deployersHandler) OnRemoved(name string, incoming *appsv1.Deployment) (*appsv1.Deployment, error) {
	if name != consts.DeploymentName {
		return incoming, nil
	}

	h.log.Debugf("Checking if %s is ready to be removed", incoming.Name)

	// TODO: we should wait to remove finalizer until after SCC CRs are cleaned up

	return incoming, nil
}
