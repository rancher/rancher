package wrangler

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"

	istiov1alpha3api "github.com/knative/pkg/apis/istio/v1alpha3"
	prommonitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	fleetv1alpha1api "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/lasso/pkg/dynamic"
	"github.com/rancher/norman/types"
	catalogv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	clusterv3api "github.com/rancher/rancher/pkg/apis/cluster.cattle.io/v3"
	managementv3api "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	projectv3api "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	provisioningv1api "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1api "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/catalogv2/content"
	"github.com/rancher/rancher/pkg/catalogv2/helmop"
	"github.com/rancher/rancher/pkg/catalogv2/system"
	"github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io"
	catalogcontrollers "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	capi "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	"github.com/rancher/rancher/pkg/generated/controllers/fleet.cattle.io"
	fleetv1alpha1 "github.com/rancher/rancher/pkg/generated/controllers/fleet.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io"
	managementv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/controllers/project.cattle.io"
	projectv3 "github.com/rancher/rancher/pkg/generated/controllers/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io"
	provisioningv1 "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io"
	rkecontrollers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/peermanager"
	"github.com/rancher/rancher/pkg/tunnelserver"
	"github.com/rancher/remotedialer"
	"github.com/rancher/steve/pkg/accesscontrol"
	"github.com/rancher/steve/pkg/client"
	"github.com/rancher/steve/pkg/server"
	"github.com/rancher/wrangler/pkg/apply"
	admissionreg "github.com/rancher/wrangler/pkg/generated/controllers/admissionregistration.k8s.io"
	admissionregcontrollers "github.com/rancher/wrangler/pkg/generated/controllers/admissionregistration.k8s.io/v1"
	"github.com/rancher/wrangler/pkg/generated/controllers/apps"
	appsv1 "github.com/rancher/wrangler/pkg/generated/controllers/apps/v1"
	"github.com/rancher/wrangler/pkg/generated/controllers/batch"
	batchv1 "github.com/rancher/wrangler/pkg/generated/controllers/batch/v1"
	"github.com/rancher/wrangler/pkg/generated/controllers/rbac"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/leader"
	"github.com/rancher/wrangler/pkg/schemes"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	capiv1beta1api "sigs.k8s.io/cluster-api/api/v1beta1"
)

var (
	localSchemeBuilder = runtime.SchemeBuilder{
		provisioningv1api.AddToScheme,
		capiv1beta1api.AddToScheme,
		fleetv1alpha1api.AddToScheme,
		managementv3api.AddToScheme,
		projectv3api.AddToScheme,
		clusterv3api.AddToScheme,
		rkev1api.AddToScheme,
		scheme.AddToScheme,
		apiextensionsv1.AddToScheme,
		apiregistrationv12.AddToScheme,
		prommonitoringv1.AddToScheme,
		istiov1alpha3api.AddToScheme,
		catalogv1.AddToScheme,
	}
	AddToScheme = localSchemeBuilder.AddToScheme
	Scheme      = runtime.NewScheme()
)

func init() {
	metav1.AddToGroupVersion(Scheme, schema.GroupVersion{Version: "v1"})
	utilruntime.Must(AddToScheme(Scheme))
	utilruntime.Must(schemes.AddToScheme(Scheme))
}

type Context struct {
	*server.Controllers

	Apply               apply.Apply
	Dynamic             *dynamic.Controller
	CAPI                capicontrollers.Interface
	RKE                 rkecontrollers.Interface
	Mgmt                managementv3.Interface
	Apps                appsv1.Interface
	Admission           admissionregcontrollers.Interface
	Batch               batchv1.Interface
	Fleet               fleetv1alpha1.Interface
	Project             projectv3.Interface
	Catalog             catalogcontrollers.Interface
	ControllerFactory   controller.SharedControllerFactory
	MultiClusterManager MultiClusterManager
	TunnelServer        *remotedialer.Server
	TunnelAuthorizer    *tunnelserver.Authorizers
	PeerManager         peermanager.PeerManager
	Provisioning        provisioningv1.Interface

	ASL                     accesscontrol.AccessSetLookup
	ClientConfig            clientcmd.ClientConfig
	CachedDiscovery         discovery.CachedDiscoveryInterface
	RESTMapper              meta.RESTMapper
	SharedControllerFactory controller.SharedControllerFactory
	leadership              *leader.Manager
	controllerLock          sync.Mutex

	RESTClientGetter      genericclioptions.RESTClientGetter
	CatalogContentManager *content.Manager
	HelmOperations        *helmop.Operations
	SystemChartsManager   *system.Manager

	started bool
}

type MultiClusterManager interface {
	NormanSchemas() *types.Schemas
	ClusterDialer(clusterID string) func(ctx context.Context, network, address string) (net.Conn, error)
	Start(ctx context.Context) error
	Wait(ctx context.Context)
	Middleware(next http.Handler) http.Handler
	K8sClient(clusterName string) (kubernetes.Interface, error)
}

func (w *Context) OnLeader(f func(ctx context.Context) error) {
	w.leadership.OnLeader(f)
}

func (w *Context) StartWithTransaction(ctx context.Context, f func(context.Context) error) (err error) {
	transaction := controller.NewHandlerTransaction(ctx)
	if err := f(transaction); err != nil {
		transaction.Rollback()
		return err
	}

	if err := w.ControllerFactory.SharedCacheFactory().Start(ctx); err != nil {
		transaction.Rollback()
		return err
	}

	w.ControllerFactory.SharedCacheFactory().WaitForCacheSync(ctx)
	transaction.Commit()
	return w.Start(ctx)
}

func (w *Context) Start(ctx context.Context) error {
	w.controllerLock.Lock()
	defer w.controllerLock.Unlock()

	if !w.started {
		if err := w.Dynamic.Register(ctx, w.SharedControllerFactory); err != nil {
			return err
		}
		w.SystemChartsManager.Start(ctx)
		w.started = true
	}

	if err := w.ControllerFactory.Start(ctx, 50); err != nil {
		return err
	}
	w.leadership.Start(ctx)
	return nil
}

func enableProtobuf(cfg *rest.Config) *rest.Config {
	cpy := rest.CopyConfig(cfg)
	cpy.AcceptContentTypes = "application/vnd.kubernetes.protobuf, application/json"
	cpy.ContentType = "application/json"
	return cpy
}

func NewContext(ctx context.Context, clientConfig clientcmd.ClientConfig, restConfig *rest.Config) (*Context, error) {
	controllerFactory, err := controller.NewSharedControllerFactoryFromConfig(enableProtobuf(restConfig), Scheme)
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

	ctlg, err := catalog.NewFactoryFromConfigWithOptions(restConfig, opts)
	if err != nil {
		return nil, err
	}

	apps, err := apps.NewFactoryFromConfigWithOptions(restConfig, opts)
	if err != nil {
		return nil, err
	}

	rbac, err := rbac.NewFactoryFromConfigWithOptions(restConfig, opts)
	if err != nil {
		return nil, err
	}

	adminReg, err := admissionreg.NewFactoryFromConfigWithOptions(restConfig, opts)
	if err != nil {
		return nil, err
	}

	project, err := project.NewFactoryFromConfigWithOptions(restConfig, opts)
	if err != nil {
		return nil, err
	}

	capi, err := capi.NewFactoryFromConfigWithOptions(restConfig, opts)
	if err != nil {
		return nil, err
	}

	rke, err := rke.NewFactoryFromConfigWithOptions(restConfig, opts)
	if err != nil {
		return nil, err
	}

	fleet, err := fleet.NewFactoryFromConfigWithOptions(restConfig, opts)
	if err != nil {
		return nil, err
	}

	provisioning, err := provisioning.NewFactoryFromConfigWithOptions(restConfig, opts)
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
		steveControllers.K8s.Discovery(),
		steveControllers.Core.ConfigMap().Cache(),
		steveControllers.Core.Secret().Cache(),
		helm.Catalog().V1().ClusterRepo().Cache())

	helmop := helmop.NewOperations(cg,
		helm.Catalog().V1(),
		rbac.Rbac().V1(),
		content,
		steveControllers.Core.Pod())

	restClientGetter := &SimpleRESTClientGetter{
		ClientConfig:    clientConfig,
		RESTConfig:      restConfig,
		CachedDiscovery: cache,
		RESTMapper:      restMapper,
	}

	systemCharts, err := system.NewManager(ctx, restClientGetter, content, helmop, steveControllers.Core.Pod(),
		mgmt.Management().V3().Setting(), ctlg.Catalog().V1().ClusterRepo())
	if err != nil {
		return nil, err
	}

	tunnelAuth := &tunnelserver.Authorizers{}
	tunnelServer := remotedialer.New(tunnelAuth.Authorize, tunnelserver.ErrorWriter)
	peerManager, err := tunnelserver.NewPeerManager(ctx, steveControllers.Core.Endpoints(), tunnelServer)
	if err != nil {
		return nil, err
	}

	leadership := leader.NewManager("", "cattle-controllers", steveControllers.K8s)
	leadership.OnLeader(func(ctx context.Context) error {
		if peerManager != nil {
			peerManager.Leader()
		}
		return nil
	})

	return &Context{
		Controllers:             steveControllers,
		Apply:                   apply,
		SharedControllerFactory: controllerFactory,
		Dynamic:                 dynamic.New(steveControllers.K8s.Discovery()),
		CAPI:                    capi.Cluster().V1beta1(),
		RKE:                     rke.Rke().V1(),
		Mgmt:                    mgmt.Management().V3(),
		Apps:                    apps.Apps().V1(),
		Admission:               adminReg.Admissionregistration().V1(),
		Project:                 project.Project().V3(),
		Fleet:                   fleet.Fleet().V1alpha1(),
		Provisioning:            provisioning.Provisioning().V1(),
		Catalog:                 helm.Catalog().V1(),
		Batch:                   batch.Batch().V1(),
		ControllerFactory:       controllerFactory,
		ASL:                     asl,
		ClientConfig:            clientConfig,
		MultiClusterManager:     noopMCM{},
		CachedDiscovery:         cache,
		RESTMapper:              restMapper,
		leadership:              leadership,
		PeerManager:             peerManager,
		RESTClientGetter:        restClientGetter,
		CatalogContentManager:   content,
		HelmOperations:          helmop,
		SystemChartsManager:     systemCharts,
		TunnelAuthorizer:        tunnelAuth,
		TunnelServer:            tunnelServer,
	}, nil
}

type noopMCM struct {
}

func (n noopMCM) NormanSchemas() *types.Schemas {
	return nil
}

func (n noopMCM) ClusterDialer(clusterID string) func(ctx context.Context, network string, address string) (net.Conn, error) {
	return func(ctx context.Context, network string, address string) (net.Conn, error) {
		return nil, fmt.Errorf("no cluster manager")
	}
}

func (n noopMCM) Wait(ctx context.Context) {
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
