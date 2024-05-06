package clusterauthtoken

import (
	"context"
	"fmt"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/managementuser/clusterauthtoken/common"
	managementv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

const tokenByUserAndClusterIndex = "auth.management.cattle.io/token-by-user-and-cluster"

func RegisterIndexers(scaledContext *config.ScaledContext) error {
	tokenInformer := scaledContext.Management.Tokens("").Controller().Informer()
	tokenIndexers := map[string]cache.IndexFunc{
		tokenByUserAndClusterIndex: tokenByUserAndCluster,
	}

	return tokenInformer.AddIndexers(tokenIndexers)
}

func Register(ctx context.Context, cluster *config.UserContext) {
	starter := cluster.DeferredStart(ctx, func(ctx context.Context) error {
		registerDeferred(ctx, cluster)
		return nil
	})

	clusters := cluster.Management.Management.Clusters("")
	clusters.AddHandler(ctx, "clusterauthtoken-deferred", func(key string, obj *v3.Cluster) (runtime.Object, error) {
		if obj != nil &&
			obj.Name == cluster.ClusterName &&
			obj.Spec.LocalClusterAuthEndpoint.Enabled {
			return obj, starter()
		}
		return obj, nil
	})
}

func registerDeferred(ctx context.Context, cluster *config.UserContext) {
	tokenInformer := cluster.Management.Management.Tokens("").Controller().Informer()

	namespace := common.DefaultNamespace
	clusterName := cluster.ClusterName
	clusterAuthToken := cluster.Cluster.ClusterAuthTokens(namespace)
	clusterAuthTokenLister := cluster.Cluster.ClusterAuthTokens(namespace).Controller().Lister()
	clusterUserAttribute := cluster.Cluster.ClusterUserAttributes(namespace)
	clusterUserAttributeLister := cluster.Cluster.ClusterUserAttributes(namespace).Controller().Lister()
	clusterConfigMap := cluster.Core.ConfigMaps(namespace)
	clusterConfigMapLister := cluster.Core.ConfigMaps(namespace).Controller().Lister()
	tokenIndexer := tokenInformer.GetIndexer()
	userLister := cluster.Management.Management.Users("").Controller().Lister()
	userAttribute := cluster.Management.Management.UserAttributes("")
	userAttributeLister := cluster.Management.Management.UserAttributes("").Controller().Lister()
	settingInterface := cluster.Management.Management.Settings("")

	cluster.Management.Management.Settings("").AddHandler(ctx, "cat-setting-controller", (&settingHandler{
		namespace,
		clusterConfigMap,
		clusterConfigMapLister,
		settingInterface,
	}).Sync)

	cluster.Management.Management.Tokens("").AddClusterScopedLifecycle(ctx,
		"cat-token-controller",
		clusterName,
		&tokenHandler{
			namespace,
			clusterAuthToken,
			clusterAuthTokenLister,
			clusterUserAttribute,
			clusterUserAttributeLister,
			tokenIndexer,
			userLister,
			userAttributeLister,
		})

	cluster.Management.Management.Users("").AddHandler(ctx, "cat-user-controller", (&userHandler{
		namespace,
		clusterUserAttribute,
		clusterUserAttributeLister,
	}).Sync)

	cluster.Management.Management.UserAttributes("").AddHandler(ctx, "cat-user-attribute-controller", (&userAttributeHandler{
		namespace,
		clusterUserAttribute,
		clusterUserAttributeLister,
	}).Sync)

	cluster.Cluster.ClusterUserAttributes(namespace).AddHandler(ctx, "cat-cluster-user-attribute-controller", (&clusterUserAttributeHandler{
		userAttribute,
		userAttributeLister,
		clusterUserAttribute,
	}).Sync)
}

func tokenUserClusterKey(token *managementv3.Token) string {
	return fmt.Sprintf("%s/%s", token.UserID, token.ClusterName)
}

func tokenByUserAndCluster(obj interface{}) ([]string, error) {
	t, ok := obj.(*managementv3.Token)
	if !ok {
		return []string{}, nil
	}
	return []string{tokenUserClusterKey(t)}, nil
}
