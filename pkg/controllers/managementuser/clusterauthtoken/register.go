package clusterauthtoken

import (
	"context"
	"fmt"

	extv1 "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/managementuser/clusterauthtoken/common"
	extstore "github.com/rancher/rancher/pkg/ext/stores/tokens"
	ext "github.com/rancher/rancher/pkg/generated/controllers/ext.cattle.io/v1"
	managementv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

const (
	tokenByUserAndClusterIndex = "auth.management.cattle.io/token-by-user-and-cluster"

	clusterController              = "cat-cluster-controller-deferred"
	tokenController                = "cat-token-controller"
	extTokenController             = "cat-ext-token-controller"
	settingController              = "cat-setting-controller"
	userController                 = "cat-user-controller"
	userAttributeController        = "cat-user-attribute-controller"
	clusterUserAttributeController = "cat-cluster-user-attribute-controller"
	clusterAuthTokenController     = "cat-cluster-auth-token-controller"
)

// RegisterExtIndexers adds indexing of ext tokens by user and cluster to their
// controller.
func RegisterExtIndexers(extAPI ext.Interface) error {
	return extAPI.Token().Informer().
		AddIndexers(map[string]cache.IndexFunc{
			tokenByUserAndClusterIndex: extTokenByUserAndCluster,
		})
}

// RegisterIndexers adds indexing of v3 tokens by user and cluster to their
// controller.
func RegisterIndexers(scaledContext *config.ScaledContext) error {
	return scaledContext.Management.Tokens("").Controller().Informer().
		AddIndexers(map[string]cache.IndexFunc{
			tokenByUserAndClusterIndex: tokenByUserAndCluster,
		})
}

// Register sets up cluster initializations to run when the cluster has started.
func Register(ctx context.Context, cluster *config.UserContext) {
	starter := cluster.DeferredStart(ctx, func(ctx context.Context) error {
		registerDeferred(ctx, cluster)
		return nil
	})

	clusters := cluster.Management.Management.Clusters("")
	clusters.AddHandler(ctx, clusterController, func(key string, obj *v3.Cluster) (runtime.Object, error) {
		if obj != nil &&
			obj.Name == cluster.ClusterName &&
			obj.Spec.LocalClusterAuthEndpoint.Enabled {
			return obj, starter()
		}
		return obj, nil
	})
}

// registerDeferred sets up the handlers for the incoming cluster which sync
// tokens (v3 and ext) to the cluster auth tokens in the remote.
func registerDeferred(ctx context.Context, cluster *config.UserContext) {
	ext := wrangler.GetExtAPI(cluster.Management.Wrangler)

	tokenInformer := cluster.Management.Management.Tokens("").Controller().Informer()
	tokenCache := cluster.Management.Wrangler.Mgmt.Token().Cache()
	tokenClient := cluster.Management.Wrangler.Mgmt.Token()

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

	extTokenIndexer := ext.Token().Informer().GetIndexer()
	eTokenCache := ext.Token().Cache()
	eTokenStore := extstore.NewSystemFromWrangler(cluster.Management.Wrangler)

	cluster.Management.Management.Settings("").AddHandler(ctx, settingController, (&settingHandler{
		namespace,
		clusterConfigMap,
		clusterConfigMapLister,
		settingInterface,
	}).Sync)

	handler := &tokenHandler{
		namespace,
		clusterAuthToken,
		clusterAuthTokenLister,
		clusterUserAttribute,
		clusterUserAttributeLister,
		tokenIndexer,
		extTokenIndexer,
		userLister,
		userAttributeLister,
	}

	cluster.Management.Management.Tokens("").AddClusterScopedLifecycle(ctx,
		tokenController,
		clusterName,
		handler)

	eTokenLifecycle(ctx, ext.Token(), extTokenController, clusterName, handler)

	cluster.Management.Management.Users("").AddHandler(ctx, userController, (&userHandler{
		namespace,
		clusterUserAttribute,
		clusterUserAttributeLister,
	}).Sync)

	cluster.Management.Management.UserAttributes("").AddHandler(ctx, userAttributeController, (&userAttributeHandler{
		namespace,
		clusterUserAttribute,
		clusterUserAttributeLister,
	}).Sync)

	cluster.Cluster.ClusterUserAttributes(namespace).AddHandler(ctx, clusterUserAttributeController, (&clusterUserAttributeHandler{
		userAttribute,
		userAttributeLister,
		clusterUserAttribute,
	}).Sync)

	cluster.Cluster.ClusterAuthTokens(namespace).AddHandler(ctx, clusterAuthTokenController, (&clusterAuthTokenHandler{
		tokenCache:  tokenCache,
		tokenClient: tokenClient,
		eTokenCache: eTokenCache,
		eTokenStore: eTokenStore,
	}).sync)
}

// tokenUserClusterKey computes the v3 token's key for indexing by user and
// cluster
func tokenUserClusterKey(token *managementv3.Token) string {
	return fmt.Sprintf("%s/%s", token.UserID, token.ClusterName)
}

// tokenByUserAndCluster indexes v3 tokens by the user and cluster they belong to
func tokenByUserAndCluster(obj interface{}) ([]string, error) {
	t, ok := obj.(*managementv3.Token)
	if !ok {
		return []string{}, nil
	}
	return []string{tokenUserClusterKey(t)}, nil
}

// extTokenByUserAndCluster indexes ext tokens by the user and cluster they belong to
func extTokenByUserAndCluster(obj interface{}) ([]string, error) {
	t, ok := obj.(*extv1.Token)
	if !ok {
		return []string{}, nil
	}
	return []string{extTokenUserClusterKey(t)}, nil
}

// extTokenUserClusterKey computes the ext token's key for indexing by user and
// cluster
func extTokenUserClusterKey(token *extv1.Token) string {
	return fmt.Sprintf("%s/%s", token.Spec.UserID, token.Spec.ClusterName)
}

// eTokenLifecycle registers handlers watching for tokens scoped to the given
// cluster. The handlers sync changes in these tokens to the remote cluster, as
// cluster auth tokens.
func eTokenLifecycle(ctx context.Context, tok ext.TokenController, controller, clusterName string, h *tokenHandler) {
	tok.OnChange(ctx,
		controller+"-change-"+clusterName,
		func(key string, obj *extv1.Token) (*extv1.Token, error) {
			// ignore removals
			if obj == nil {
				return obj, nil
			}
			// handle only tokens referencing the watched cluster
			if clusterName != obj.Spec.ClusterName {
				return obj, nil
			}
			return h.ExtUpdated(obj)
		})

	tok.OnRemove(ctx,
		controller+"-remove-"+clusterName,
		func(key string, obj *extv1.Token) (*extv1.Token, error) {
			// handle only tokens referencing the watched cluster
			if clusterName != obj.Spec.ClusterName {
				return obj, nil
			}
			return h.ExtRemove(obj)
		})
}
