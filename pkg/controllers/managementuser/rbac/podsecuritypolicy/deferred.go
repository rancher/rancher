package podsecuritypolicy

import (
	"context"
	"errors"
	"strings"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
)

func Register(ctx context.Context, userContext *config.UserContext) {
	starter := userContext.DeferredStart(ctx, func(ctx context.Context) error {
		clusterName := userContext.ClusterName
		logrus.Infof("Checking cluster [%s] compatibility before registering podsecuritypolicy controllers.", clusterName)
		clusterLister := userContext.Management.Management.Clusters("").Controller().Lister()
		err := checkClusterVersion(clusterName, clusterLister)
		if err != nil {
			if errors.Is(err, errVersionIncompatible) {
				logrus.Errorf("%v - will not register podsecuritypolicy controllers for cluster [%s].", err, clusterName)
				return nil
			}
			return err
		}
		logrus.Infof("cluster [%s] compatibility for podsecuritypolicy controllers check succeeded.", clusterName)
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
