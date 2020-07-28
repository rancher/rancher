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
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io"
	catalogcontrollers "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io"
	managementv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/controllers/project.cattle.io"
	projectv3 "github.com/rancher/rancher/pkg/generated/controllers/project.cattle.io/v3"
	"github.com/rancher/steve/pkg/accesscontrol"
	"github.com/rancher/steve/pkg/server"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/leader"
	"github.com/rancher/wrangler/pkg/schemes"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
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
	Project             projectv3.Interface
	Catalog             catalogcontrollers.Interface
	ControllerFactory   controller.SharedControllerFactory
	MultiClusterManager MultiClusterManager

	ASL            accesscontrol.AccessSetLookup
	leadership     *leader.Manager
	controllerLock sync.Mutex
}

type MultiClusterManager interface {
	ClusterDialer(clusterID string) func(ctx context.Context, network, address string) (net.Conn, error)
	Start(ctx context.Context) error
	Middleware(next http.Handler) http.Handler
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

func NewContext(ctx context.Context, restConfig *rest.Config) (*Context, error) {
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

	asl := accesscontrol.NewAccessStore(ctx, features.Steve.Enabled(), steveControllers.RBAC)

	return &Context{
		Controllers:         steveControllers,
		Apply:               apply,
		Mgmt:                mgmt.Management().V3(),
		Project:             project.Project().V3(),
		Catalog:             helm.Catalog().V1(),
		ControllerFactory:   controllerFactory,
		ASL:                 asl,
		MultiClusterManager: noopMCM{},
		leadership:          leader.NewManager("", "cattle-controllers", steveControllers.K8s),
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
