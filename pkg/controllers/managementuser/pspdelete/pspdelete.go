package pspdelete

import (
	"context"
	"errors"
	"fmt"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/managementuser/rbac/podsecuritypolicy"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	provisioningcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	v1beta12 "github.com/rancher/rancher/pkg/generated/norman/policy/v1beta1"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	globalUnrestrictedAnnotation = "psp.rke2.io/global-unrestricted"
	globalRestrictedAnnotation   = "psp.rke2.io/global-restricted"
)

type handler struct {
	clusterName         string
	clusterCache        provisioningcontrollers.ClusterCache
	podSecurityPolicies v1beta12.PodSecurityPolicyInterface
}

func Register(ctx context.Context, userContext *config.UserContext) {
	starter := userContext.DeferredStart(ctx, func(ctx context.Context) error {
		clusterName := userContext.ClusterName
		clusterLister := userContext.Management.Management.Clusters("").Controller().Lister()
		err := podsecuritypolicy.CheckClusterVersion(clusterName, clusterLister)
		if err != nil {
			if errors.Is(err, podsecuritypolicy.ErrClusterVersionIncompatible) {
				logrus.Debugf("%v - will not register pspdelete controller for cluster [%s].", err, clusterName)
				return nil
			}
			return fmt.Errorf("unable to parse version of cluster %s: %w", clusterName, err)
		}
		logrus.Debugf("Cluster [%s] is compatible with PSPs, will run pspdelete controller.", clusterName)
		registerDeferred(ctx, userContext)
		return nil
	})

	clusters := userContext.Management.Management.Clusters("")
	clusterCache := userContext.Management.Wrangler.Provisioning.Cluster().Cache()
	clusters.AddHandler(ctx, "pspdelete-deferred", func(key string, obj *v3.Cluster) (runtime.Object, error) {
		if obj == nil ||
			obj.Name != userContext.ClusterName ||
			obj.Spec.DefaultPodSecurityPolicyTemplateName == "" {
			return obj, nil
		}

		clusters, err := clusterCache.GetByIndex(cluster.ByCluster, obj.Name)
		if err != nil || len(clusters) != 1 {
			return obj, err
		}

		clusterSpec := clusters[0].Spec
		if clusterSpec.DefaultPodSecurityPolicyTemplateName != "" && clusterSpec.RKEConfig != nil {
			return obj, starter()
		}

		return obj, nil
	})

}

func registerDeferred(ctx context.Context, context *config.UserContext) {
	h := handler{
		clusterName:         context.ClusterName,
		clusterCache:        context.Management.Wrangler.Provisioning.Cluster().Cache(),
		podSecurityPolicies: context.Policy.PodSecurityPolicies(""),
	}

	context.Policy.PodSecurityPolicies("").AddHandler(ctx, "psp-delete", h.sync)
}

func has(data map[string]string, key string) bool {
	_, ok := data[key]
	return ok
}

func (h *handler) sync(key string, obj *v1beta1.PodSecurityPolicy) (runtime.Object, error) {
	if obj == nil {
		return obj, nil
	}

	if !has(obj.Annotations, globalUnrestrictedAnnotation) && !has(obj.Annotations, globalRestrictedAnnotation) {
		return obj, nil
	}

	clusters, err := h.clusterCache.GetByIndex(cluster.ByCluster, h.clusterName)
	if err != nil || len(clusters) != 1 {
		return obj, err
	}

	clusterSpec := clusters[0].Spec
	if clusterSpec.DefaultPodSecurityPolicyTemplateName != "" && clusters[0].Spec.RKEConfig != nil {
		err := h.podSecurityPolicies.Delete(obj.Name, nil)
		return obj, err
	}

	return obj, nil
}
