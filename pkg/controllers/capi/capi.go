package capi

import (
	"context"

	controllerruntime "github.com/rancher/lasso/controller-runtime"
	"github.com/rancher/rancher/pkg/controllers/capi/logger"
	rkecontrollers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/schemes"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	clusterv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	clusterv1alpha4 "sigs.k8s.io/cluster-api/api/v1alpha4"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/api/v1beta1/index"
	"sigs.k8s.io/cluster-api/controllers"
	"sigs.k8s.io/cluster-api/controllers/remote"
	addonsv1alpha3 "sigs.k8s.io/cluster-api/exp/addons/api/v1alpha3"
	addonsv1alpha4 "sigs.k8s.io/cluster-api/exp/addons/api/v1alpha4"
	addonsv1beta1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
	expv1alpha3 "sigs.k8s.io/cluster-api/exp/api/v1alpha3"
	expv1alpha4 "sigs.k8s.io/cluster-api/exp/api/v1alpha4"
	expv1beta1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

func init() {
	_ = clientgoscheme.AddToScheme(schemes.All)
	_ = clusterv1alpha3.AddToScheme(schemes.All)
	_ = clusterv1alpha4.AddToScheme(schemes.All)
	_ = clusterv1beta1.AddToScheme(schemes.All)
	_ = expv1alpha3.AddToScheme(schemes.All)
	_ = expv1alpha4.AddToScheme(schemes.All)
	_ = expv1beta1.AddToScheme(schemes.All)
	_ = addonsv1alpha3.AddToScheme(schemes.All)
	_ = addonsv1alpha4.AddToScheme(schemes.All)
	_ = addonsv1beta1.AddToScheme(schemes.All)
	_ = apiextensionsv1.AddToScheme(schemes.All)
}

type connectedAgentClusterCacheClient struct {
	client.Client
	rkeControlPlanesCache rkecontrollers.RKEControlPlaneCache
}

func (t *connectedAgentClusterCacheClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	rkeCP, err := t.rkeControlPlanesCache.Get(key.Namespace, key.Name)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	// obj should be a CAPI cluster. If this cluster doesn't have an RKEControlPlane associated with it,
	// then no agent check is required.
	if err == nil && !rkeCP.Status.AgentConnected {
		// If the agent is not connected, then returning a NotFound error will cause CAPI to stop its caches
		// They will be refreshed once the agent reconnects.
		// See the ClusterCacheReconciler: https://github.com/kubernetes-sigs/cluster-api/blob/5026786ee809c5def466049f5befd2a786fbcefa/controllers/remote/cluster_cache_reconciler.go#L60
		return apierrors.NewNotFound(schema.GroupResource{
			Group:    obj.GetObjectKind().GroupVersionKind().Group,
			Resource: obj.GetObjectKind().GroupVersionKind().Kind,
		}, key.Name)
	}

	return t.Client.Get(ctx, key, obj)
}

func Register(ctx context.Context, clients *wrangler.Context) (func(ctx context.Context) error, error) {
	mgr, err := ctrl.NewManager(clients.RESTConfig, ctrl.Options{
		MetricsBindAddress: "0",
		NewCache: controllerruntime.NewNewCacheFunc(clients.SharedControllerFactory.SharedCacheFactory(),
			clients.Dynamic),
		Scheme: schemes.All,
		ClientDisableCacheFor: []client.Object{
			&corev1.ConfigMap{},
			&corev1.Secret{},
		},
		Logger: logger.New(2),
		// Work around a panic where the broadcaster is immediately closed
		EventBroadcaster: record.NewBroadcaster(),
	})
	if err != nil {
		return nil, err
	}

	// add the node ref indexer for health checks
	err = index.AddDefaultIndexes(ctx, mgr)
	if err != nil {
		return nil, err
	}

	reconcilers, err := reconcilers(mgr, clients)
	if err != nil {
		return nil, err
	}

	for _, reconciler := range reconcilers {
		if err := reconciler.SetupWithManager(ctx, mgr, concurrency(5)); err != nil {
			return nil, err
		}
	}

	return mgr.Start, nil
}

func reconcilers(mgr ctrl.Manager, clients *wrangler.Context) ([]reconciler, error) {
	l := ctrl.Log.WithName("remote").WithName("ClusterCacheTracker")
	tracker, err := remote.NewClusterCacheTracker(
		mgr,
		remote.ClusterCacheTrackerOptions{
			Log:     &l,
			Indexes: remote.DefaultIndexes,
		},
	)
	if err != nil {
		return nil, err
	}

	return []reconciler{
		&remote.ClusterCacheReconciler{
			// If the cluster agent gets disconnected, then the caches that CAPI uses could get out of sync.
			// By using a connectedAgentClusterCacheClient (which returns a NotFound error if the agent is not connected), then
			// we can force CAPI to refresh its caches once the agent is connected again.
			Client:  &connectedAgentClusterCacheClient{Client: mgr.GetClient(), rkeControlPlanesCache: clients.RKE.RKEControlPlane().Cache()},
			Log:     ctrl.Log.WithName("remote").WithName("ClusterCacheReconciler"),
			Tracker: tracker,
		},
		&controllers.ClusterReconciler{
			Client:    mgr.GetClient(),
			APIReader: mgr.GetAPIReader(),
		},
		&controllers.MachineReconciler{
			Client:    mgr.GetClient(),
			APIReader: mgr.GetAPIReader(),
			Tracker:   tracker,
		},
		&controllers.MachineSetReconciler{
			Client:    mgr.GetClient(),
			APIReader: mgr.GetAPIReader(),
			Tracker:   tracker,
		},
		&controllers.MachineDeploymentReconciler{
			Client:    mgr.GetClient(),
			APIReader: mgr.GetAPIReader(),
		},
		&controllers.MachineHealthCheckReconciler{
			Client:  mgr.GetClient(),
			Tracker: tracker,
		},
	}, nil
}

func concurrency(c int) controller.Options {
	return controller.Options{MaxConcurrentReconciles: c}
}

type reconciler interface {
	SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error
}
