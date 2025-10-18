package clusterauthtoken

import (
	"context"
	"fmt"

	lassocache "github.com/rancher/lasso/pkg/cache"
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	extv1 "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers"
	"github.com/rancher/rancher/pkg/controllers/managementuser/clusterauthtoken/common"
	extstore "github.com/rancher/rancher/pkg/ext/stores/tokens"
	ext "github.com/rancher/rancher/pkg/generated/controllers/ext.cattle.io/v1"
	managementv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	logrus.Debugf("[%s] register ext indexer", clusterAuthTokenController)
	return extAPI.Token().Informer().
		AddIndexers(map[string]cache.IndexFunc{
			tokenByUserAndClusterIndex: extTokenByUserAndCluster,
		})
}

// RegisterIndexers adds indexing of v3 tokens by user and cluster to their
// controller.
func RegisterIndexers(scaledContext *config.ScaledContext) error {
	logrus.Debugf("[%s] register v3 indexer", clusterAuthTokenController)
	return scaledContext.Management.Tokens("").Controller().Informer().
		AddIndexers(map[string]cache.IndexFunc{
			tokenByUserAndClusterIndex: tokenByUserAndCluster,
		})
}

// Register sets up cluster initializations to run when the cluster has started.
func Register(ctx context.Context, cluster *config.UserContext) {
	// Defer until the EXT API is ready
	clusters := cluster.Management.Management.Clusters("")
	cluster.Management.Wrangler.DeferredEXTAPIRegistration.DeferFunc(func(w *wrangler.EXTAPIContext) {
		starter := cluster.DeferredStart(ctx, func(ctx context.Context) error {
			if err := registerDeferred(ctx, cluster, w); err != nil {
				logrus.Errorf("[%s] Failed to register controller: %v", clusterAuthTokenController, err)
				return err
			}
			return nil
		})
		clusters.AddHandler(ctx, clusterController, func(key string, obj *v3.Cluster) (runtime.Object, error) {
			if obj != nil &&
				obj.Name == cluster.ClusterName &&
				obj.Spec.LocalClusterAuthEndpoint.Enabled {
				return obj, starter()
			}
			return obj, nil
		})
	})
}

// registerDeferred sets up the handlers for the new remote cluster which sync
// tokens (v3 and ext) to the cluster auth tokens in that remote.
func registerDeferred(ctx context.Context, cluster *config.UserContext, extAPIContext *wrangler.EXTAPIContext) error {
	tokenInformer := cluster.Management.Management.Tokens("").Controller().Informer()
	tokenCache := cluster.Management.Wrangler.Mgmt.Token().Cache()
	tokenClient := cluster.Management.Wrangler.Mgmt.Token()

	namespace := common.DefaultNamespace
	clusterName := cluster.ClusterName
	clusterAuthToken := cluster.Cluster.ClusterAuthTokens(namespace)
	clusterAuthTokenLister := cluster.Cluster.ClusterAuthTokens(namespace).Controller().Lister()
	clusterUserAttribute := cluster.Cluster.ClusterUserAttributes(namespace)
	clusterUserAttributeLister := cluster.Cluster.ClusterUserAttributes(namespace).Controller().Lister()
	clusterConfigMap := cluster.Corew.ConfigMap()
	clusterConfigMapLister := cluster.Corew.ConfigMap().Cache()
	clusterSecret := cluster.Corew.Secret()
	tokenIndexer := tokenInformer.GetIndexer()
	userLister := cluster.Management.Management.Users("").Controller().Lister()
	userAttribute := cluster.Management.Management.UserAttributes("")
	userAttributeLister := cluster.Management.Management.UserAttributes("").Controller().Lister()
	settingInterface := cluster.Management.Management.Settings("")

	// We use a separate controller factory that only watches a single namespace. This ensures that the cache does not contain all the secrets, just those of that namespace.
	// The default controller factory does not allow caching secrets for all namespaces, see https://github.com/rancher/rancher/issues/46827
	clientFactory := cluster.ControllerFactory.SharedCacheFactory().SharedClientFactory()
	clusterSecretLister, controllerFactory := newDedicatedSecretsCache(clientFactory, namespace)
	if err := cluster.RegisterExtraControllerFactory("clusterauthtoken", controllerFactory); err != nil {
		return err
	}

	cluster.Management.Management.Settings("").AddHandler(ctx, settingController, (&settingHandler{
		namespace,
		clusterConfigMap,
		clusterConfigMapLister,
		settingInterface,
	}).Sync)

	handler := &tokenHandler{
		namespace:                  namespace,
		clusterAuthToken:           clusterAuthToken,
		clusterAuthTokenLister:     clusterAuthTokenLister,
		clusterUserAttribute:       clusterUserAttribute,
		clusterUserAttributeLister: clusterUserAttributeLister,
		tokenIndexer:               tokenIndexer,
		userLister:                 userLister,
		userAttributeLister:        userAttributeLister,
		clusterSecret:              clusterSecret,
		clusterSecretLister:        clusterSecretLister,
	}

	cluster.Management.Management.Tokens("").AddClusterScopedLifecycle(ctx,
		tokenController,
		clusterName,
		handler)

	extToken := extAPIContext.Client.Token()
	handler.extTokenIndexer = extToken.Informer().GetIndexer()
	extTokenLifecycle(ctx, extToken, extTokenController, clusterName, handler)

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

	catHandler := &clusterAuthTokenHandler{
		tokenCache:  tokenCache,
		tokenClient: tokenClient,
	}

	catHandler.extTokenCache = extAPIContext.Client.Token().Cache()
	catHandler.extTokenStore = extstore.NewSystemFromWrangler(cluster.Management.Wrangler)

	cluster.Cluster.ClusterAuthTokens(namespace).AddHandler(ctx, clusterAuthTokenController, catHandler.sync)

	return nil
}

func newDedicatedSecretsCache(clientFactory client.SharedClientFactory, namespace string) (corecontrollers.SecretCache, controller.SharedControllerFactory) {
	cacheFactory := lassocache.NewSharedCachedFactory(clientFactory, &lassocache.SharedCacheFactoryOptions{
		KindNamespace: map[schema.GroupVersionKind]string{
			corev1.SchemeGroupVersion.WithKind("Secret"): namespace,
		},
	})

	controllerFactory := controller.NewSharedControllerFactory(cacheFactory, controllers.GetOptsFromEnv(controllers.User))
	return corecontrollers.New(controllerFactory).Secret().Cache(), controllerFactory
}

// tokenUserClusterKey computes the v3 token's key for indexing by user and
// cluster
func tokenUserClusterKey(token *managementv3.Token) string {
	return fmt.Sprintf("%s/%s", token.UserID, token.ClusterName)
}

// tokenByUserAndCluster indexes v3 tokens by the user and cluster they belong to
func tokenByUserAndCluster(obj any) ([]string, error) {
	t, ok := obj.(*managementv3.Token)
	if !ok {
		return []string{}, nil
	}
	return []string{tokenUserClusterKey(t)}, nil
}

// extTokenByUserAndCluster indexes ext tokens by the user and cluster they belong to
func extTokenByUserAndCluster(obj any) ([]string, error) {
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

// extTokenLifecycle registers handlers watching for tokens scoped to the given
// cluster. The handlers sync changes in these tokens to the remote cluster, as
// cluster auth tokens.
func extTokenLifecycle(ctx context.Context, tok ext.TokenController, controller, clusterName string, h *tokenHandler) {
	logrus.Debugf("[%s] WATCH CLUSTER %q", clusterAuthTokenController, clusterName)

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
			logrus.Debugf("[%s] CLUSTER %q, TOKEN %q, SYNC DOWN", clusterAuthTokenController, obj.Name, clusterName)
			return h.ExtUpdated(obj)
		})

	tok.OnRemove(ctx,
		controller+"-remove-"+clusterName,
		func(key string, obj *extv1.Token) (*extv1.Token, error) {
			// handle only tokens referencing the watched cluster
			if clusterName != obj.Spec.ClusterName {
				return obj, nil
			}
			logrus.Debugf("[%s] CLUSTER %q, TOKEN %q, REMOVE DOWN", clusterAuthTokenController, obj.Name, clusterName)
			return h.ExtRemove(obj)
		})
}
