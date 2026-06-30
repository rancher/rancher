package clusterauthtoken

import (
	"context"
	"fmt"
	"sync"
	"time"

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
// started by UserContext.Start() as part of the cluster controller's normal
// factory-start sequence.
//
// Must be called before UserContext.Start() — i.e. during managementuser.Register,
// while the transaction is open and startContext is still nil. When Start() is
// later called, the factory starts alongside ControllerFactory and all others;
// if it fails, the error surfaces to the cluster manager, which logs the
// failure, marks the cluster unavailable, and retries.
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

// Register wires up clusterauthtoken event handlers. secretsCache must be the
// value returned by RegisterFactory on the same UserContext.
//
// v3 Token handlers are registered immediately — they have no EXT API
// dependency and are active as soon as the management informers deliver events,
// eliminating the EXT API readiness window for v3 Token → ClusterAuthToken sync.
//
// ext Token handlers are registered once the EXT API is ready, since they
// require the EXT API client context.
func Register(ctx context.Context, cluster *config.UserContext, secretsCache corecontrollers.SecretCache) {
	namespace := common.DefaultNamespace
	clusterName := cluster.ClusterName

	tokenInformer := cluster.Management.Management.Tokens("").Controller().Informer()
	tokenCache := cluster.Management.Wrangler.Mgmt.Token().Cache()
	tokenClient := cluster.Management.Wrangler.Mgmt.Token()

	clusterAuthToken := cluster.Cluster.ClusterAuthTokens(namespace)
	clusterAuthTokenLister := cluster.Cluster.ClusterAuthTokens(namespace).Controller().Lister()
	clusterUserAttribute := cluster.Cluster.ClusterUserAttributes(namespace)
	clusterUserAttributeLister := cluster.Cluster.ClusterUserAttributes(namespace).Controller().Lister()
	clusterConfigMap := cluster.Corew.ConfigMap()
	clusterConfigMapLister := cluster.Corew.ConfigMap().Cache()
	clusterSecret := cluster.Corew.Secret()
	userLister := cluster.Management.Management.Users("").Controller().Lister()
	userAttribute := cluster.Management.Management.UserAttributes("")
	userAttributeLister := cluster.Management.Management.UserAttributes("").Controller().Lister()
	settingInterface := cluster.Management.Management.Settings("")

	// extTokenIndexer starts nil and is set when the EXT API becomes ready.
	// The v3 Remove path already guards against a nil extTokenIndexer.
	handler := &tokenHandler{
		namespace:                  namespace,
		clusterAuthToken:           clusterAuthToken,
		clusterAuthTokenLister:     clusterAuthTokenLister,
		clusterUserAttribute:       clusterUserAttribute,
		clusterUserAttributeLister: clusterUserAttributeLister,
		tokenIndexer:               tokenInformer.GetIndexer(),
		userLister:                 userLister,
		userAttributeLister:        userAttributeLister,
		clusterSecret:              clusterSecret,
		clusterSecretLister:        secretsCache,
	}

	cluster.Management.Management.Settings("").AddHandler(ctx, settingController, (&settingHandler{
		namespace,
		clusterConfigMap,
		clusterConfigMapLister,
		settingInterface,
	}).Sync)

	cluster.Management.Management.Tokens("").AddClusterScopedLifecycle(ctx,
		tokenController,
		clusterName,
		handler)

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

	scheduleStartupCensus(tokenInformer)
	logrus.Infof("[clusterauthtoken-sync] cluster=%s v3 Token handlers registered", clusterName)

	cluster.Management.Wrangler.DeferredEXTAPIRegistration.DeferFunc(func(w *wrangler.EXTAPIContext) {
		logrus.Infof("[clusterauthtoken-sync] cluster=%s ext API ready, registering ext Token handlers", clusterName)
		extToken := w.Client.Token()
		handler.extTokenIndexer.Store(extToken.Informer().GetIndexer())
		extTokenLifecycle(ctx, extToken, extTokenController, clusterName, handler)

		catHandler := &clusterAuthTokenHandler{
			tokenCache:    tokenCache,
			tokenClient:   tokenClient,
			extTokenCache: extToken.Cache(),
			extTokenStore: extstore.NewSystemFromWrangler(cluster.Management.Wrangler),
		}
		cluster.Cluster.ClusterAuthTokens(namespace).AddHandler(ctx, clusterAuthTokenController, catHandler.sync)
		logrus.Infof("[clusterauthtoken-sync] cluster=%s ext Token handlers registered", clusterName)
		scheduleExtStartupCensus(extToken.Informer())
	})
}

var (
	censusOnce    sync.Once
	extCensusOnce sync.Once
)

// scheduleStartupCensus launches a one-shot goroutine that, once the management
// v3.Token informer cache has synced, walks the cache exactly once and reports
// per-cluster counts at Info level: total v3.Tokens scoped to each cluster and
// the subset missing the cat-token-controller lifecycle annotation. The latter
// equals the per-cluster backlog of tokens that have never been processed by
// the downstream sync handler — i.e. the tokens that got stuck. The number
// should drop to zero within seconds of handler registration as the
// re-enqueue-all from AddClusterScopedLifecycle drains the backlog. A non-zero
// residual after a few minutes points to a remaining sync path failure.
//
// The walk is process-wide, not per UserContext: the management Token informer
// cache is shared across all clusters, so a single walk is sufficient. The
// goroutine runs at most once per process (guarded by sync.Once); subsequent
// calls from other clusters' Register are no-ops.
func scheduleStartupCensus(mgmtTokenInformer cache.SharedIndexInformer) {
	censusOnce.Do(func() {
		go runStartupCensus(mgmtTokenInformer)
	})
}

func runStartupCensus(mgmtTokenInformer cache.SharedIndexInformer) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if !cache.WaitForCacheSync(ctx.Done(), mgmtTokenInformer.HasSynced) {
		logrus.Warnf("[clusterauthtoken-sync] startup census skipped: management v3.Token cache did not sync within timeout")
		return
	}

	type counts struct{ total, missing int }
	byCluster := map[string]*counts{}
	for _, obj := range mgmtTokenInformer.GetStore().List() {
		t, ok := obj.(*managementv3.Token)
		if !ok || t.ClusterName == "" {
			continue
		}
		c, ok := byCluster[t.ClusterName]
		if !ok {
			c = &counts{}
			byCluster[t.ClusterName] = c
		}
		c.total++
		annotation := "lifecycle.cattle.io/create.cat-token-controller_" + t.ClusterName
		if t.Annotations[annotation] != "true" {
			c.missing++
		}
	}
	for clusterName, c := range byCluster {
		logrus.Infof("[clusterauthtoken-sync] cluster=%s startup census: v3_tokens=%d missing_lifecycle_annotation=%d",
			clusterName, c.total, c.missing)
	}
}

// scheduleExtStartupCensus mirrors scheduleStartupCensus for ext.Tokens. Called
// from the EXT API DeferFunc once per UserContext; sync.Once ensures the walk
// runs exactly once per process across all clusters. ext.Tokens are managed by
// wrangler OnChange/OnRemove rather than Norman lifecycle, so they carry no
// stuck-state annotation. The census reports inventory only — total ext.Tokens
// per cluster. The actual drain is observable via the per-token "ext create"
// Info events emitted by the sync handler.
func scheduleExtStartupCensus(extTokenInformer cache.SharedIndexInformer) {
	extCensusOnce.Do(func() {
		go runExtStartupCensus(extTokenInformer)
	})
}

func runExtStartupCensus(extTokenInformer cache.SharedIndexInformer) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if !cache.WaitForCacheSync(ctx.Done(), extTokenInformer.HasSynced) {
		logrus.Warnf("[clusterauthtoken-sync] ext startup census skipped: ext.Token cache did not sync within timeout")
		return
	}

	byCluster := map[string]int{}
	for _, obj := range extTokenInformer.GetStore().List() {
		t, ok := obj.(*extv1.Token)
		if !ok || t.Spec.ClusterName == "" {
			continue
		}
		byCluster[t.Spec.ClusterName]++
	}
	for clusterName, total := range byCluster {
		logrus.Infof("[clusterauthtoken-sync] cluster=%s startup census: ext_tokens=%d", clusterName, total)
	}
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
