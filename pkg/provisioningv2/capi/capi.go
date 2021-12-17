package capi

import (
	"context"

	controllerruntime "github.com/rancher/lasso/controller-runtime"
	"github.com/rancher/rancher/pkg/provisioningv2/capi/logger"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/schemes"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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

	reconcilers, err := reconcilers(mgr)
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

func reconcilers(mgr ctrl.Manager) ([]reconciler, error) {
	tracker, err := remote.NewClusterCacheTracker(
		mgr,
		remote.ClusterCacheTrackerOptions{
			Log:     ctrl.Log.WithName("remote").WithName("ClusterCacheTracker"),
			Indexes: remote.DefaultIndexes,
		},
	)
	if err != nil {
		return nil, err
	}

	return []reconciler{
		&remote.ClusterCacheReconciler{
			Client:  mgr.GetClient(),
			Log:     ctrl.Log.WithName("remote").WithName("ClusterCacheReconciler"),
			Tracker: tracker,
		},
		&controllers.ClusterReconciler{
			Client: mgr.GetClient(),
		},
		&controllers.MachineReconciler{
			Client:  mgr.GetClient(),
			Tracker: tracker,
		},
		&controllers.MachineSetReconciler{
			Client:  mgr.GetClient(),
			Tracker: tracker,
		},
		&controllers.MachineDeploymentReconciler{
			Client: mgr.GetClient(),
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
