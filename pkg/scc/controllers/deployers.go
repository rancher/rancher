package controllers

import (
	"context"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/scc/consts"
	"github.com/rancher/rancher/pkg/scc/deployer"
	"github.com/rancher/rancher/pkg/scc/util/log"
	deploymentControllers "github.com/rancher/wrangler/v3/pkg/generated/controllers/apps/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

type deployersHandler struct {
	ctx             context.Context
	log             log.StructuredLogger
	deployments     deploymentControllers.DeploymentController
	deploymentCache deploymentControllers.DeploymentCache
}

func RegisterDeployers(
	ctx context.Context,
	logger log.StructuredLogger,
	sccDeployer deployer.SCCDeployer,
	deployments deploymentControllers.DeploymentController,
	deploymentCache deploymentControllers.DeploymentCache,
	settings mgmtv3.SettingController,
) {
	controller := &deployersHandler{
		ctx:             ctx,
		log:             logger.WithField("controller", "scc-deployers"),
		deployments:     deployments,
		deploymentCache: deploymentCache,
	}

	// Set up a handler for the SCCOperatorImage setting
	relatedresource.Watch(ctx, "scc-operator-image-watcher",
		sccDeployer.OnRelatedSettings,
		deployments,
		settings)

	namespaceScopedOnChange(
		ctx,
		"scc-deployment",
		consts.DefaultSCCNamespace,
		controller.deployments,
		controller.OnDeploymentChange,
	)
}

func (h *deployersHandler) OnDeploymentChange(name string, incomingDeploy *appsv1.Deployment) (*appsv1.Deployment, error) {
	return incomingDeploy, nil
}

func namespaceScopedOnChange[T generic.RuntimeMetaObject](ctx context.Context, name, namespace string, c generic.ControllerMeta, sync generic.ObjectHandler[T]) {
	condition := namespaceScopedCondition(namespace)
	onChangeHandler := generic.FromObjectHandlerToHandler(sync)
	c.AddGenericHandler(ctx, name, func(key string, obj runtime.Object) (runtime.Object, error) {
		if condition(obj) {
			return onChangeHandler(key, obj)
		}
		return obj, nil
	})
}

// TODO(wrangler/v4): revert to use OnRemove when it supports options (https://github.com/rancher/wrangler/pull/472).
func namespaceScopedOnRemove[T generic.RuntimeMetaObject](ctx context.Context, name, namespace string, c generic.ControllerMeta, sync generic.ObjectHandler[T]) {
	condition := namespaceScopedCondition(namespace)
	onRemoveHandler := generic.NewRemoveHandler(name, c.Updater(), generic.FromObjectHandlerToHandler(sync))
	c.AddGenericHandler(ctx, name, func(key string, obj runtime.Object) (runtime.Object, error) {
		if condition(obj) {
			return onRemoveHandler(key, obj)
		}
		return obj, nil
	})
}

func namespaceScopedCondition(namespace string) func(obj runtime.Object) bool {
	return func(obj runtime.Object) bool { return inExpectedNamespace(obj, namespace) }
}

func inExpectedNamespace(obj runtime.Object, namespace string) bool {
	metadata, err := meta.Accessor(obj)
	if err != nil {
		return false
	}

	return metadata.GetNamespace() == namespace
}
