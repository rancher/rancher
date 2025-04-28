package clusterauthtoken

import (
	"context"
	"fmt"

	extv1 "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/managementuser/clusterauthtoken/common"
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

func RegisterExtIndexers(extAPI ext.Interface) error {
	fmt.Printf("ZZZZZ ext token indexers\n")

	return extAPI.Token().Informer().
		AddIndexers(map[string]cache.IndexFunc{
			tokenByUserAndClusterIndex: extTokenByUserAndCluster,
		})
}

func RegisterIndexers(scaledContext *config.ScaledContext) error {
	return scaledContext.Management.Tokens("").Controller().Informer().
		AddIndexers(map[string]cache.IndexFunc{
			tokenByUserAndClusterIndex: tokenByUserAndCluster,
		})
}

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

func registerDeferred(ctx context.Context, cluster *config.UserContext) {
	fmt.Printf("ZZZZZ c(%p) register deferred ....\n", ctx)

	ext := wrangler.GetExtAPI(cluster.Management.Wrangler)

	fmt.Printf("ZZZZZ c(%p) usrC ((%T))\n", cluster, cluster)
	fmt.Printf("ZZZZZ c(%p) Mgm1 ((%T))\n", cluster, cluster.Management)
	fmt.Printf("ZZZZZ c(%p) Mgm2 ((%T))\n", cluster, cluster.Management.Management)
	fmt.Printf("ZZZZZ c(%p) nTok ((%T))\n", cluster, cluster.Management.Management.Tokens(""))
	fmt.Printf("ZZZZZ c(%p) TokC ((%T))\n", cluster, cluster.Management.Management.Tokens("").Controller())
	fmt.Printf("ZZZZZ c(%p) TokI ((%T))\n", cluster, cluster.Management.Management.Tokens("").Controller().Informer())

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

	fmt.Printf("ZZZZZ w(%p) wran ((%T))\n", cluster.Management.Wrangler, cluster.Management.Wrangler)
	fmt.Printf("ZZZZZ w(%p) wExt ((%T) %p)\n", cluster.Management.Wrangler, cluster.Management.Wrangler.Ext, cluster.Management.Wrangler.Ext)

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
	}).sync)
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

func extTokenByUserAndCluster(obj interface{}) ([]string, error) {
	fmt.Printf("ZZZZZ XX eTBUAC (%v)\n", obj)

	t, ok := obj.(*extv1.Token)
	if !ok {
		return []string{}, nil
	}
	return []string{extTokenUserClusterKey(t)}, nil
}

func extTokenUserClusterKey(token *extv1.Token) string {
	fmt.Printf("ZZZZZ XX eTUCK u(%s) // c(%s)\n",
		token.Spec.UserID, token.Spec.ClusterName)

	return fmt.Sprintf("%s/%s", token.Spec.UserID, token.Spec.ClusterName)
}

func eTokenLifecycle(ctx context.Context, tok ext.TokenController, controller, clusterName string, h *tokenHandler) {

	fmt.Printf("ZZZZZ eTokLC (%T -- (%v))\n", tok, tok)
	fmt.Printf("ZZZZZ CLUSTER ((%s))\n", clusterName)
	fmt.Printf("ZZZZZ CONTROL ((%s))\n", controller)

	tok.OnChange(ctx,
		controller+"-change-"+clusterName,
		func(key string, obj *extv1.Token) (*extv1.Token, error) {
			// ignore removals
			if obj == nil {
				fmt.Printf("ZZZZZ A ETOKEN CHANGE /%s/ (--|--) @(%s)\n", key, clusterName)
				return obj, nil
			}
			fmt.Printf("ZZZZZ A ETOKEN CHANGE /%s/ (%s|%s) @(%s)\n",
				key, obj.Name, obj.Spec.ClusterName, clusterName)
			// ignore tokens of no or other clusters
			if clusterName != obj.Spec.ClusterName {
				fmt.Printf("ZZZZZ A ETOKEN CHANGE /%s/ (%s|%s) @(%s) IGNORED, MISMATCH\n",
					key, obj.Name, obj.Spec.ClusterName, clusterName)
				return obj, nil
			}
			return h.ExtUpdated(obj)
		})

	tok.OnRemove(ctx,
		controller+"-remove-"+clusterName,
		func(key string, obj *extv1.Token) (*extv1.Token, error) {
			fmt.Printf("ZZZZZ A ETOKEN REMOVE /%s/ (%s|%s) @(%s)\n", key, obj.Name, obj.Spec.ClusterName, clusterName)
			// ignore tokens of no or other clusters
			if clusterName != obj.Spec.ClusterName {
				fmt.Printf("ZZZZZ A ETOKEN REMOVE /%s/ (%s|%s) @(%s) IGNORED, MISMATCH\n",
					key, obj.Name, obj.Spec.ClusterName, clusterName)
				return obj, nil
			}
			return h.ExtRemove(obj)
		})
}
