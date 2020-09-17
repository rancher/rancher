package wrangler

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"

	prommonitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	istiov1alpha3api "github.com/knative/pkg/apis/istio/v1alpha3"
	"github.com/rancher/lasso/pkg/controller"
	catalogv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	clusterv3api "github.com/rancher/rancher/pkg/apis/cluster.cattle.io/v3"
	managementv3api "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	projectv3api "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/catalogv2/content"
	"github.com/rancher/rancher/pkg/catalogv2/helmop"
	"github.com/rancher/rancher/pkg/catalogv2/system"
	"github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io"
	catalogcontrollers "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io"
	managementv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/controllers/project.cattle.io"
	projectv3 "github.com/rancher/rancher/pkg/generated/controllers/project.cattle.io/v3"
	"github.com/rancher/steve/pkg/accesscontrol"
	"github.com/rancher/steve/pkg/client"
	"github.com/rancher/steve/pkg/server"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/generated/controllers/batch"
	batchv1 "github.com/rancher/wrangler/pkg/generated/controllers/batch/v1"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/leader"
	"github.com/rancher/wrangler/pkg/schemes"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	apiregistrationv12 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
)

var (
	localSchemeBuilder = runtime.SchemeBuilder{
		managementv3api.AddToScheme,
		projectv3api.AddToScheme,
		clusterv3api.AddToScheme,
		scheme.AddToScheme,
		apiextensionsv1beta1.AddToScheme,
		apiregistrationv12.AddToScheme,
		prommonitoringv1.AddToScheme,
		istiov1alpha3api.AddToScheme,
		catalogv1.AddToScheme,
	}
	AddToScheme = localSchemeBuilder.AddToScheme
	Scheme      = runtime.NewScheme()
)

func init() {
	utilruntime.Must(AddToScheme(Scheme))
	utilruntime.Must(schemes.AddToScheme(Scheme))
}

type Context struct {
	*server.Controllers

	Apply               apply.Apply
	Mgmt                managementv3.Interface
	Batch               batchv1.Interface
	Project             projectv3.Interface
	Catalog             catalogcontrollers.Interface
	ControllerFactory   controller.SharedControllerFactory
	MultiClusterManager MultiClusterManager

	ASL             accesscontrol.AccessSetLookup
	ClientConfig    clientcmd.ClientConfig
	CachedDiscovery discovery.CachedDiscoveryInterface
	RESTMapper      meta.RESTMapper
	leadership      *leader.Manager
	controllerLock  sync.Mutex

	RESTClientGetter      genericclioptions.RESTClientGetter
	CatalogContentManager *content.Manager
	HelmOperations        *helmop.Operations
	SystemChartsManager   *system.Manager
}

type MultiClusterManager interface {
	ClusterDialer(clusterID string) func(ctx context.Context, network, address string) (net.Conn, error)
	Start(ctx context.Context) error
	Middleware(next http.Handler) http.Handler
	K8sClient(clusterName string) (kubernetes.Interface, error)
}

func (w *Context) OnLeader(f func(ctx context.Context) error) {
	w.leadership.OnLeader(f)
}

func (w *Context) StartWithTransaction(ctx context.Context, f func(context.Context) error) (err error) {
	defer func() {
		if err == nil {
			err = w.Start(ctx)
		}
	}()

	w.controllerLock.Lock()
	defer w.controllerLock.Unlock()

	transaction := controller.NewHandlerTransaction(ctx)
	if err := f(transaction); err != nil {
		transaction.Rollback()
		return err
	}
	transaction.Commit()
	return nil
}

func (w *Context) Start(ctx context.Context) error {
	w.controllerLock.Lock()
	defer w.controllerLock.Unlock()

	if err := w.ControllerFactory.Start(ctx, 5); err != nil {
		return err
	}
	w.leadership.Start(ctx)
	return nil
}

func NewContext(ctx context.Context, clientConfig clientcmd.ClientConfig, restConfig *rest.Config) (*Context, error) {
	controllerFactory, err := controller.NewSharedControllerFactoryFromConfig(restConfig, Scheme)
	if err != nil {
		return nil, err
	}

	opts := &generic.FactoryOptions{
		SharedControllerFactory: controllerFactory,
	}

	steveControllers, err := server.NewController(restConfig, opts)
	if err != nil {
		return nil, err
	}

	cache := memory.NewMemCacheClient(steveControllers.K8s.Discovery())
	restMapper := restmapper.NewDeferredDiscoveryRESTMapper(cache)

	apply, err := apply.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	mgmt, err := management.NewFactoryFromConfigWithOptions(restConfig, opts)
	if err != nil {
		return nil, err
	}

	project, err := project.NewFactoryFromConfigWithOptions(restConfig, opts)
	if err != nil {
		return nil, err
	}

	helm, err := catalog.NewFactoryFromConfigWithOptions(restConfig, opts)
	if err != nil {
		return nil, err
	}

	batch, err := batch.NewFactoryFromConfigWithOptions(restConfig, opts)
	if err != nil {
		return nil, err
	}

	asl := accesscontrol.NewAccessStore(ctx, true, steveControllers.RBAC)

	cg, err := client.NewFactory(restConfig, false)
	if err != nil {
		return nil, err
	}

	content := content.NewManager(
		steveControllers.Core.ConfigMap().Cache(),
		steveControllers.Core.Secret().Cache(),
		helm.Catalog().V1().ClusterRepo().Cache())

	helmop := helmop.NewOperations(cg,
		helm.Catalog().V1(),
		content,
		steveControllers.Core.Pod())

	restClientGetter := &SimpleRESTClientGetter{
		ClientConfig:    clientConfig,
		RESTConfig:      restConfig,
		CachedDiscovery: cache,
		RESTMapper:      restMapper,
	}

	systemCharts, err := system.NewManager(ctx, restClientGetter, content, helmop, steveControllers.Core.Pod())
	if err != nil {
		return nil, err
	}

	return &Context{
		Controllers:           steveControllers,
		Apply:                 apply,
		Mgmt:                  mgmt.Management().V3(),
		Project:               project.Project().V3(),
		Catalog:               helm.Catalog().V1(),
		Batch:                 batch.Batch().V1(),
		ControllerFactory:     controllerFactory,
		ASL:                   asl,
		ClientConfig:          clientConfig,
		MultiClusterManager:   noopMCM{},
		CachedDiscovery:       cache,
		RESTMapper:            restMapper,
		leadership:            leader.NewManager("", "cattle-controllers", steveControllers.K8s),
		RESTClientGetter:      restClientGetter,
		CatalogContentManager: content,
		HelmOperations:        helmop,
		SystemChartsManager:   systemCharts,
	}, nil
}

type noopMCM struct {
}

func (n noopMCM) ClusterDialer(clusterID string) func(ctx context.Context, network string, address string) (net.Conn, error) {
	return func(ctx context.Context, network string, address string) (net.Conn, error) {
		return nil, fmt.Errorf("no cluster manager")
	}
}

func (n noopMCM) Start(ctx context.Context) error {
	return nil
}

func (n noopMCM) Middleware(next http.Handler) http.Handler {
	return next
}

func (n noopMCM) K8sClient(clusterName string) (kubernetes.Interface, error) {
	return nil, nil
}

type SimpleRESTClientGetter struct {
	ClientConfig    clientcmd.ClientConfig
	RESTConfig      *rest.Config
	CachedDiscovery discovery.CachedDiscoveryInterface
	RESTMapper      meta.RESTMapper
}

func (s *SimpleRESTClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return s.ClientConfig
}

func (s *SimpleRESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	return s.RESTConfig, nil
}

func (s *SimpleRESTClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	return s.CachedDiscovery, nil
}

func (s *SimpleRESTClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	return s.RESTMapper, nil
}
