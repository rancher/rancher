package pspdelete

import (
	"context"

	"github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	provisioningcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	v1beta12 "github.com/rancher/rancher/pkg/generated/norman/policy/v1beta1"
	"github.com/rancher/rancher/pkg/types/config"
	"k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	globalUnrestrictedAnnotation = "psp.rke2.io/global-unrestricted"
)

type handler struct {
	clusterName         string
	clusterCache        provisioningcontrollers.ClusterCache
	podSecurityPolicies v1beta12.PodSecurityPolicyInterface
}

func Register(ctx context.Context, context *config.UserContext) {
	h := handler{
		clusterName:         context.ClusterName,
		clusterCache:        context.Management.Wrangler.Provisioning.Cluster().Cache(),
		podSecurityPolicies: context.Policy.PodSecurityPolicies(""),
	}

	context.Policy.PodSecurityPolicies("").AddHandler(ctx, "psp-delete", h.sync)
}

func (h *handler) sync(key string, obj *v1beta1.PodSecurityPolicy) (runtime.Object, error) {
	if obj == nil {
		return obj, nil
	}

	if _, ok := obj.Annotations[globalUnrestrictedAnnotation]; !ok {
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
