package podsecuritypolicy

import (
	"context"
	"strings"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"k8s.io/apimachinery/pkg/runtime"
)

func Register(ctx context.Context, userContext *config.UserContext) {
	starter := userContext.DeferredStart(ctx, func(ctx context.Context) error {
		registerDeferred(ctx, userContext)
		return nil
	})

	clusterPrefix := userContext.ClusterName + ":"
	psptpb := userContext.Management.Management.PodSecurityPolicyTemplateProjectBindings("")
	psptpb.AddHandler(ctx, "psptpb-deferred",
		func(key string, obj *v3.PodSecurityPolicyTemplateProjectBinding) (runtime.Object, error) {
			if obj != nil && strings.HasPrefix(obj.TargetProjectName, clusterPrefix) {
				return obj, starter()
			}
			return obj, nil
		})

	clusters := userContext.Management.Management.Clusters("")
	clusters.AddHandler(ctx, "psptpb-deferred", func(key string, obj *apimgmtv3.Cluster) (runtime.Object, error) {
		if obj != nil && obj.Name == userContext.ClusterName && obj.Spec.DefaultPodSecurityPolicyTemplateName != "" {
			return obj, starter()
		}
		return obj, nil
	})
}

func registerDeferred(ctx context.Context, context *config.UserContext) {
	RegisterCluster(ctx, context)
	RegisterClusterRole(ctx, context)
	RegisterBindings(ctx, context)
	RegisterNamespace(ctx, context)
	RegisterPodSecurityPolicy(ctx, context)
	RegisterServiceAccount(ctx, context)
	RegisterTemplate(ctx, context)
}
