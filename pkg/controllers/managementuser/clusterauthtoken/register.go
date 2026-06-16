package clusterauthtoken

import (
	"context"
	"fmt"

	lassocache "github.com/rancher/lasso/pkg/cache"
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	extv1 "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

const (
	tokenByUserAndClusterIndex = "auth.management.cattle.io/token-by-user-and-cluster"

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

// RegisterFactory creates the dedicated namespace-scoped secrets cache for
// clusterauthtoken handlers and registers it with the UserContext so it is
// started by UserContext.Start() as part of doStart()'s normal factory-start
// sequence.
//
// Must be called before UserContext.Start() — i.e. during managementuser.Register,
// while the transaction is open and startContext is still nil. When Start() is
// later called, the factory starts alongside ControllerFactory and all others;
// if it fails, the error surfaces through doStart() to the cluster manager,
// which logs the failure, marks the cluster unavailable, and retries.
//
// Returns the SecretCache to pass to Register() for handler wiring.
func RegisterFactory(cluster *config.UserContext) (corecontrollers.SecretCache, error) {
	clientFactory := cluster.ControllerFactory.SharedCacheFactory().SharedClientFactory()
	secretsCache, controllerFactory := newDedicatedSecretsCache(clientFactory, common.DefaultNamespace)
	// startContext is nil here: factory is added to extraControllerFactories
	// without being started. UserContext.Start() starts it in its factory loop.
	if err := cluster.RegisterExtraControllerFactory("clusterauthtoken", controllerFactory); err != nil {
		return nil, err
	}
	return secretsCache, nil
}

// Register wires up clusterauthtoken event handlers once the EXT API is ready.
// secretsCache must be the value returned by RegisterFactory on the same
// UserContext. Handler wiring is pure in-process Go with no network calls, so
// no retry machinery is needed.
func Register(ctx context.Context, cluster *config.UserContext, secretsCache corecontrollers.SecretCache) {
	cluster.Management.Wrangler.DeferredEXTAPIRegistration.DeferFunc(func(w *wrangler.EXTAPIContext) {
		registerHandlers(ctx, cluster, secretsCache, w)
	})
}

// registerHandlers wires all event handlers. It has no factory lifecycle
// operations and makes no network calls, so it is called synchronously from
// the DeferFunc goroutine without additional wrapping or a retry gate.
func registerHandlers(ctx context.Context, cluster *config.UserContext, secretsCache corecontrollers.SecretCache, extAPIContext *wrangler.EXTAPIContext) {
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
		clusterSecretLister:        secretsCache,
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

func tokenUserClusterKey(token *managementv3.Token) string {
	return fmt.Sprintf("%s/%s", token.UserID, token.ClusterName)
}

func tokenByUserAndCluster(obj any) ([]string, error) {
	t, ok := obj.(*managementv3.Token)
	if !ok {
		return []string{}, nil
	}
	return []string{tokenUserClusterKey(t)}, nil
}

func extTokenByUserAndCluster(obj any) ([]string, error) {
	t, ok := obj.(*extv1.Token)
	if !ok {
		return []string{}, nil
	}
	return []string{extTokenUserClusterKey(t)}, nil
}

func extTokenUserClusterKey(token *extv1.Token) string {
	return fmt.Sprintf("%s/%s", token.Spec.UserID, token.Spec.ClusterName)
}

func extTokenLifecycle(ctx context.Context, tok ext.TokenController, controller, clusterName string, h *tokenHandler) {
	logrus.Debugf("[%s] WATCH CLUSTER %q", clusterAuthTokenController, clusterName)

	tok.OnChange(ctx,
		controller+"-change-"+clusterName,
		func(key string, obj *extv1.Token) (*extv1.Token, error) {
			if obj == nil {
				return obj, nil
			}
			if clusterName != obj.Spec.ClusterName {
				return obj, nil
			}
			logrus.Debugf("[%s] CLUSTER %q, TOKEN %q, SYNC DOWN", clusterAuthTokenController, obj.Name, clusterName)
			return h.ExtUpdated(obj)
		})

	tok.OnRemove(ctx,
		controller+"-remove-"+clusterName,
		func(key string, obj *extv1.Token) (*extv1.Token, error) {
			if clusterName != obj.Spec.ClusterName {
				return obj, nil
			}
			logrus.Debugf("[%s] CLUSTER %q, TOKEN %q, REMOVE DOWN", clusterAuthTokenController, obj.Name, clusterName)
			return h.ExtRemove(obj)
		})
}
