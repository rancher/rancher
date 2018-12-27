package clusterauthtoken

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/pkg/controllers/user/clusterauthtoken/common"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/cache"

	managementv3 "github.com/rancher/types/apis/management.cattle.io/v3"
)

const tokenByUserAndClusterIndex = "auth.management.cattle.io/token-by-user-and-cluster"

func Register(ctx context.Context, cluster *config.UserContext) {
	tokenInformer := cluster.Management.Management.Tokens("").Controller().Informer()
	tokenIndexers := map[string]cache.IndexFunc{
		tokenByUserAndClusterIndex: tokenByUserAndCluster,
	}
	if err := tokenInformer.AddIndexers(tokenIndexers); err != nil {
		logrus.Error(err)
		return
	}

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

	cluster.Management.Management.Settings("").AddLifecycle(ctx, "cat-setting-controller", &SettingHandler{
		namespace,
		clusterConfigMap,
		clusterConfigMapLister,
	})
	cluster.Management.Management.Tokens("").AddLifecycle(ctx, "cat-token-controller", &TokenHandler{
		namespace,
		clusterName,
		clusterAuthToken,
		clusterAuthTokenLister,
		clusterUserAttribute,
		clusterUserAttributeLister,
		tokenIndexer,
		userLister,
		userAttributeLister,
	})
	cluster.Management.Management.Users("").AddLifecycle(ctx, "cat-user-controller", &UserHandler{
		namespace,
		clusterUserAttribute,
		clusterUserAttributeLister,
	})
	cluster.Management.Management.UserAttributes("").AddHandler(ctx, "cat-user-attribute-controller", (&UserAttributeHandler{
		namespace,
		clusterUserAttribute,
		clusterUserAttributeLister,
	}).Sync)
	cluster.Cluster.ClusterUserAttributes(namespace).AddHandler(ctx, "cat-cluster-user-attribute-controller", (&ClusterUserAttributeHandler{
		userAttribute,
		userAttributeLister,
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
