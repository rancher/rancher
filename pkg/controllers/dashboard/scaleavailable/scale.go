package scaleavailable

import (
	"context"
	"strconv"

	"github.com/rancher/rancher/pkg/wrangler"
	appscontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/apps/v1"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
)

const (
	availableAnnotation = "management.cattle.io/scale-available"
)

type handler struct {
	node        corecontrollers.NodeCache
	deployments appscontrollers.DeploymentClient
}

func Register(ctx context.Context, wrangler *wrangler.Context) {
	h := &handler{
		node:        wrangler.Core.Node().Cache(),
		deployments: wrangler.Apps.Deployment(),
	}
	deploymentCache := wrangler.Apps.Deployment().Cache()
	wrangler.Apps.Deployment().OnChange(ctx, "scale-available", h.OnChange)
	deploymentCache.AddIndexer(availableAnnotation, func(obj *appsv1.Deployment) ([]string, error) {
		if val := obj.Annotations[availableAnnotation]; val != "" {
			return []string{availableAnnotation}, nil
		}
		return nil, nil
	})
	relatedresource.Watch(ctx, "scale-available-trigger", func(namespace, name string, obj runtime.Object) (result []relatedresource.Key, _ error) {
		deps, err := deploymentCache.GetByIndex(availableAnnotation, availableAnnotation)
		if err != nil {
			return nil, err
		}
		for _, obj := range deps {
			result = append(result, relatedresource.Key{
				Namespace: obj.Namespace,
				Name:      obj.Name,
			})
		}
		return result, nil
	}, wrangler.Apps.Deployment(), wrangler.Core.Node())
}

func (h *handler) OnChange(key string, deployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	if deployment == nil {
		return nil, nil
	}
	numStr := deployment.Annotations[availableAnnotation]
	if numStr == "" {
		return deployment, nil
	}

	num, err := strconv.Atoi(numStr)
	if err != nil {
		// ignore errors
		return deployment, nil
	}

	selector := labels.SelectorFromSet(deployment.Spec.Template.Spec.NodeSelector)
	if deployment.Spec.Template.Spec.Affinity != nil && deployment.Spec.Template.Spec.Affinity.NodeAffinity != nil &&
		deployment.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		for _, term := range deployment.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
			for _, term := range term.MatchExpressions {
				req, err := labels.NewRequirement(term.Key, selection.Operator(term.Operator), term.Values)
				if err == nil {
					selector.Add(*req)
				}
			}
		}
	}

	nodes, err := h.node.List(labels.SelectorFromSet(deployment.Spec.Template.Spec.NodeSelector))
	if err != nil {
		return deployment, nil
	}

	if len(nodes) < num {
		num = len(nodes)
	}

	if num == 0 {
		num = 1
	}

	if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != int32(num) {
		replicas := int32(num)
		deployment := deployment.DeepCopy()
		deployment.Spec.Replicas = &replicas
		return h.deployments.Update(deployment)
	}

	return deployment, nil
}
