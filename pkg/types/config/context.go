/*
Package config contains functions for creating management, scaled, and user contexts that user norman controllers.
*/
package config

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rancher/lasso/pkg/cache"
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/norman/objectclient/dynamic"
	"github.com/rancher/norman/restwatch"
	"github.com/rancher/norman/store/proxy"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/catalog/manager"
	"github.com/rancher/rancher/pkg/controllers"
	"github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io"
	apiregistrationv1 "github.com/rancher/rancher/pkg/generated/norman/apiregistration.k8s.io/v1"
	appsv1 "github.com/rancher/rancher/pkg/generated/norman/apps/v1"
	autoscaling "github.com/rancher/rancher/pkg/generated/norman/autoscaling/v2"
	batchv1 "github.com/rancher/rancher/pkg/generated/norman/batch/v1"
	clusterv3 "github.com/rancher/rancher/pkg/generated/norman/cluster.cattle.io/v3"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	extv1beta1 "github.com/rancher/rancher/pkg/generated/norman/extensions/v1beta1"
	managementv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	monitoringv1 "github.com/rancher/rancher/pkg/generated/norman/monitoring.coreos.com/v1"
	knetworkingv1 "github.com/rancher/rancher/pkg/generated/norman/networking.k8s.io/v1"
	policyv1beta1 "github.com/rancher/rancher/pkg/generated/norman/policy/v1beta1"
	projectv3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	rbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	storagev1 "github.com/rancher/rancher/pkg/generated/norman/storage.k8s.io/v1"
	"github.com/rancher/rancher/pkg/peermanager"
	clusterSchema "github.com/rancher/rancher/pkg/schemas/cluster.cattle.io/v3"
	managementSchema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	projectSchema "github.com/rancher/rancher/pkg/schemas/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config/dialer"
	"github.com/rancher/rancher/pkg/types/config/systemtokens"
	"github.com/rancher/rancher/pkg/user"
	"github.com/rancher/rancher/pkg/wrangler"
	steve "github.com/rancher/steve/pkg/server"
	"github.com/rancher/wrangler/v2/pkg/generated/controllers/core"
	wcorev1 "github.com/rancher/wrangler/v2/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v2/pkg/generated/controllers/rbac"
	wrbacv1 "github.com/rancher/wrangler/v2/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/v2/pkg/generic"
	"github.com/sirupsen/logrus"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8dynamic "k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	UserStorageContext       types.StorageContext = "user"
	ManagementStorageContext types.StorageContext = "mgmt"
)

type ScaledContext struct {
	ClientGetter      proxy.ClientGetter
	RESTConfig        rest.Config
	ControllerFactory controller.SharedControllerFactory
	UnversionedClient rest.Interface
	K8sClient         kubernetes.Interface
	APIExtClient      clientset.Interface
	Schemas           *types.Schemas
	AccessControl     types.AccessControl
	Dialer            dialer.Factory
	SystemTokens      systemtokens.Interface
	UserManager       user.Manager
	PeerManager       peermanager.PeerManager
	CatalogManager    manager.CatalogManager

	Management managementv3.Interface
	Project    projectv3.Interface
	RBAC       rbacv1.Interface
	Core       corev1.Interface
	Storage    storagev1.Interface

	Wrangler          *wrangler.Context
	RunContext        context.Context
	managementContext *ManagementContext
}

func (c *ScaledContext) NewManagementContext() (*ManagementContext, error) {
	if c.managementContext != nil {
		return c.managementContext, nil
	}
	mgmt, err := newManagementContext(c)
	if err != nil {
		return nil, err
	}
	mgmt.Dialer = c.Dialer
	mgmt.UserManager = c.UserManager
	mgmt.SystemTokens = c.SystemTokens
	mgmt.CatalogManager = c.CatalogManager
	mgmt.Wrangler = c.Wrangler
	c.managementContext = mgmt
	return mgmt, nil
}

type ScaleContextOptions struct {
	ControllerFactory controller.SharedControllerFactory
}

func enableProtobuf(cfg *rest.Config) *rest.Config {
	cpy := rest.CopyConfig(cfg)
	cpy.AcceptContentTypes = "application/vnd.kubernetes.protobuf, application/json"
	cpy.ContentType = "application/json"
	return cpy
}

func NewScaledContext(config rest.Config, opts *ScaleContextOptions) (*ScaledContext, error) {
	var err error

	if opts == nil {
		opts = &ScaleContextOptions{}
	}

	context := &ScaledContext{
		RESTConfig: *steve.RestConfigDefaults(&config),
	}

	if opts.ControllerFactory == nil {
		controllerFactoryOpts := controllers.GetOptsFromEnv(controllers.Scaled)
		controllerFactory, err := controller.NewSharedControllerFactoryFromConfigWithOptions(enableProtobuf(&context.RESTConfig), wrangler.Scheme, controllerFactoryOpts)
		if err != nil {
			return nil, err
		}
		context.ControllerFactory = controllerFactory
	} else {
		context.ControllerFactory = opts.ControllerFactory
	}

	context.Management = managementv3.NewFromControllerFactory(context.ControllerFactory)
	context.Project = projectv3.NewFromControllerFactory(context.ControllerFactory)
	context.RBAC = rbacv1.NewFromControllerFactory(context.ControllerFactory)
	context.Core = corev1.NewFromControllerFactory(context.ControllerFactory)

	context.K8sClient, err = kubernetes.NewForConfig(&config)
	if err != nil {
		return nil, err
	}

	dynamicConfig := config
	if dynamicConfig.NegotiatedSerializer == nil {
		dynamicConfig.NegotiatedSerializer = dynamic.NegotiatedSerializer
	}

	context.UnversionedClient, err = restwatch.UnversionedRESTClientFor(&dynamicConfig)
	if err != nil {
		return nil, err
	}

	context.APIExtClient, err = clientset.NewForConfig(&dynamicConfig)
	if err != nil {
		return nil, err
	}

	context.Schemas = types.NewSchemas().
		AddSchemas(managementSchema.Schemas).
		AddSchemas(clusterSchema.Schemas).
		AddSchemas(projectSchema.Schemas)

	return context, err
}

func (c *ScaledContext) Start(ctx context.Context) error {
	logrus.Info("Starting API controllers")
	return c.ControllerFactory.Start(ctx, 50)
}

type ManagementContext struct {
	ClientGetter      proxy.ClientGetter
	RESTConfig        rest.Config
	ControllerFactory controller.SharedControllerFactory
	UnversionedClient rest.Interface
	DynamicClient     k8dynamic.Interface
	K8sClient         kubernetes.Interface
	APIExtClient      clientset.Interface
	Schemas           *types.Schemas
	Scheme            *runtime.Scheme
	Dialer            dialer.Factory
	UserManager       user.Manager
	SystemTokens      systemtokens.Interface
	CatalogManager    manager.CatalogManager

	Management managementv3.Interface
	Project    projectv3.Interface
	RBAC       rbacv1.Interface
	Core       corev1.Interface
	Apps       appsv1.Interface
	Wrangler   *wrangler.Context
}

type UserContext struct {
	Management        *ManagementContext
	ClusterName       string
	RESTConfig        rest.Config
	ControllerFactory controller.SharedControllerFactory
	UnversionedClient rest.Interface
	APIExtClient      clientset.Interface
	K8sClient         kubernetes.Interface
	runContext        context.Context

	APIAggregation apiregistrationv1.Interface
	Apps           appsv1.Interface
	Autoscaling    autoscaling.Interface
	Catalog        catalog.Interface
	Project        projectv3.Interface
	Core           corev1.Interface
	RBAC           rbacv1.Interface
	Extensions     extv1beta1.Interface
	BatchV1        batchv1.Interface
	Networking     knetworkingv1.Interface
	Monitoring     monitoringv1.Interface
	Cluster        clusterv3.Interface
	Storage        storagev1.Interface
	Policy         policyv1beta1.Interface

	RBACw          wrbacv1.Interface
	Corew          wcorev1.Interface
	KindNamespaces map[schema.GroupVersionKind]string
}

// WithAgent returns a shallow copy of the Context that has been configured to use a user agent in its
// clients that is the given userAgent appended to "rancher-%s-%s".
func (c *ManagementContext) WithAgent(userAgent string) *ManagementContext {
	mgmtCopy := *c
	fullUserAgent := fmt.Sprintf("rancher-%s-%s", settings.ServerVersion.Get(), userAgent)
	mgmtCopy.Management = managementv3.NewFromControllerFactoryWithAgent(fullUserAgent, c.ControllerFactory)
	mgmtCopy.Project = projectv3.NewFromControllerFactoryWithAgent(fullUserAgent, c.ControllerFactory)
	mgmtCopy.RBAC = rbacv1.NewFromControllerFactoryWithAgent(fullUserAgent, c.ControllerFactory)
	mgmtCopy.Core = corev1.NewFromControllerFactoryWithAgent(fullUserAgent, c.ControllerFactory)
	mgmtCopy.Apps = appsv1.NewFromControllerFactoryWithAgent(fullUserAgent, c.ControllerFactory)

	mgmtCopy.Wrangler = mgmtCopy.Wrangler.WithAgent(userAgent)

	return &mgmtCopy
}

func (w *UserContext) DeferredStart(ctx context.Context, register func(ctx context.Context) error) func() error {
	f := w.deferredStartAsync(ctx, register)
	return func() error {
		go f()
		return nil
	}
}

func (w *UserContext) deferredStartAsync(ctx context.Context, register func(ctx context.Context) error) func() error {
	var (
		startLock sync.Mutex
		started   = false
	)

	return func() error {
		startLock.Lock()
		defer startLock.Unlock()

		if started {
			return nil
		}

		cancelCtx, cancel := context.WithCancel(ctx)
		transaction := controller.NewHandlerTransaction(cancelCtx)
		if err := register(transaction); err != nil {
			cancel()
			transaction.Rollback()
			return err
		}

		if err := w.Start(cancelCtx); err != nil {
			cancel()
			transaction.Rollback()
			return err
		}

		transaction.Commit()
		started = true
		go func() {
			// this is make go vet happy that we aren't leaking a context
			<-ctx.Done()
			cancel()
		}()
		return nil
	}
}

func (w *UserContext) UserOnlyContext() *UserOnlyContext {
	return &UserOnlyContext{
		Schemas:           w.Management.Schemas,
		ClusterName:       w.ClusterName,
		RESTConfig:        w.RESTConfig,
		UnversionedClient: w.UnversionedClient,
		K8sClient:         w.K8sClient,

		Autoscaling: w.Autoscaling,
		Apps:        w.Apps,
		Project:     w.Project,
		Core:        w.Core,
		RBAC:        w.RBAC,
		Extensions:  w.Extensions,
		Networking:  w.Networking,
		BatchV1:     w.BatchV1,
		Monitoring:  w.Monitoring,
		Cluster:     w.Cluster,
		Storage:     w.Storage,
		Policy:      w.Policy,
	}
}

type UserOnlyContext struct {
	Schemas           *types.Schemas
	ClusterName       string
	RESTConfig        rest.Config
	ControllerFactory controller.SharedControllerFactory
	UnversionedClient rest.Interface
	K8sClient         kubernetes.Interface

	APIRegistration apiregistrationv1.Interface
	Apps            appsv1.Interface
	Autoscaling     autoscaling.Interface
	Project         projectv3.Interface
	Core            corev1.Interface
	RBAC            rbacv1.Interface
	Extensions      extv1beta1.Interface
	BatchV1         batchv1.Interface
	Networking      knetworkingv1.Interface
	Monitoring      monitoringv1.Interface
	Cluster         clusterv3.Interface
	Storage         storagev1.Interface
	Policy          policyv1beta1.Interface
}

func newManagementContext(c *ScaledContext) (*ManagementContext, error) {
	var err error

	context := &ManagementContext{
		RESTConfig: *steve.RestConfigDefaults(&c.RESTConfig),
	}

	config := c.RESTConfig
	controllerFactory := c.ControllerFactory
	context.ControllerFactory = controllerFactory

	context.Management = managementv3.NewFromControllerFactory(controllerFactory)
	context.Project = projectv3.NewFromControllerFactory(controllerFactory)
	context.RBAC = rbacv1.NewFromControllerFactory(controllerFactory)
	context.Core = corev1.NewFromControllerFactory(controllerFactory)
	context.Apps = appsv1.NewFromControllerFactory(controllerFactory)

	context.K8sClient, err = kubernetes.NewForConfig(&config)
	if err != nil {
		return nil, err
	}

	context.DynamicClient, err = k8dynamic.NewForConfig(&config)
	if err != nil {
		return nil, err
	}

	dynamicConfig := config
	if dynamicConfig.NegotiatedSerializer == nil {
		dynamicConfig.NegotiatedSerializer = dynamic.NegotiatedSerializer
	}

	context.UnversionedClient, err = restwatch.UnversionedRESTClientFor(&dynamicConfig)
	if err != nil {
		return nil, err
	}

	context.APIExtClient, err = clientset.NewForConfig(&dynamicConfig)
	if err != nil {
		return nil, err
	}

	context.Schemas = types.NewSchemas().
		AddSchemas(managementSchema.Schemas).
		AddSchemas(clusterSchema.Schemas).
		AddSchemas(projectSchema.Schemas)

	context.Scheme = wrangler.Scheme

	return context, err
}

func NewUserContext(scaledContext *ScaledContext, config rest.Config, clusterName string) (*UserContext, error) {
	var err error
	context := &UserContext{
		RESTConfig:     *steve.RestConfigDefaults(&config),
		ClusterName:    clusterName,
		runContext:     scaledContext.RunContext,
		KindNamespaces: map[schema.GroupVersionKind]string{},
	}

	context.Management, err = scaledContext.NewManagementContext()
	if err != nil {
		return nil, err
	}

	clientFactory, err := client.NewSharedClientFactory(enableProtobuf(&context.RESTConfig), &client.SharedClientFactoryOptions{
		Scheme: wrangler.Scheme,
	})
	if err != nil {
		return nil, err
	}

	cacheFactory := cache.NewSharedCachedFactory(clientFactory, &cache.SharedCacheFactoryOptions{
		KindNamespace: context.KindNamespaces,
	})

	controllerFactory := controller.NewSharedControllerFactory(cacheFactory, controllers.GetOptsFromEnv(controllers.User))
	context.ControllerFactory = controllerFactory

	context.K8sClient, err = kubernetes.NewForConfig(&config)
	if err != nil {
		return nil, err
	}

	context.Apps = appsv1.NewFromControllerFactory(controllerFactory)
	context.Core = corev1.NewFromControllerFactory(controllerFactory)
	context.Project = projectv3.NewFromControllerFactory(controllerFactory)
	context.Storage = storagev1.NewFromControllerFactory(controllerFactory)
	context.RBAC = rbacv1.NewFromControllerFactory(controllerFactory)
	context.Networking = knetworkingv1.NewFromControllerFactory(controllerFactory)
	context.Extensions = extv1beta1.NewFromControllerFactory(controllerFactory)
	context.Policy = policyv1beta1.NewFromControllerFactory(controllerFactory)
	context.BatchV1 = batchv1.NewFromControllerFactory(controllerFactory)
	context.Autoscaling = autoscaling.NewFromControllerFactory(controllerFactory)
	context.Monitoring = monitoringv1.NewFromControllerFactory(controllerFactory)
	context.Cluster = clusterv3.NewFromControllerFactory(controllerFactory)
	context.APIAggregation = apiregistrationv1.NewFromControllerFactory(controllerFactory)

	wranglerConf := config
	wranglerConf.Timeout = 30 * time.Minute
	opts := &generic.FactoryOptions{
		SharedControllerFactory: controllerFactory,
	}
	rbacw, err := rbac.NewFactoryFromConfigWithOptions(&wranglerConf, opts)
	if err != nil {
		return nil, err
	}
	context.RBACw = rbacw.Rbac().V1()
	corew, err := core.NewFactoryFromConfigWithOptions(&wranglerConf, opts)
	if err != nil {
		return nil, err
	}
	context.Corew = corew.Core().V1()

	ctlg, err := catalog.NewFactoryFromConfigWithOptions(&context.RESTConfig, &catalog.FactoryOptions{SharedControllerFactory: controllerFactory})
	if err != nil {
		return nil, err
	}
	context.Catalog = ctlg.Catalog()

	dynamicConfig := config
	if dynamicConfig.NegotiatedSerializer == nil {
		dynamicConfig.NegotiatedSerializer = dynamic.NegotiatedSerializer
	}

	context.UnversionedClient, err = restwatch.UnversionedRESTClientFor(&dynamicConfig)
	if err != nil {
		return nil, err
	}

	context.APIExtClient, err = clientset.NewForConfig(&dynamicConfig)
	if err != nil {
		return nil, err
	}

	return context, err
}

func (w *UserContext) Start(ctx context.Context) error {
	logrus.Info("Starting cluster controllers for ", w.ClusterName)
	if err := w.Management.ControllerFactory.Start(w.runContext, 50); err != nil {
		return err
	}
	return w.ControllerFactory.Start(ctx, 5)
}

func NewUserOnlyContext(config *wrangler.Context) (*UserOnlyContext, error) {
	var err error
	context := &UserOnlyContext{
		RESTConfig:        *config.RESTConfig,
		ControllerFactory: config.ControllerFactory,
		K8sClient:         config.K8s,
	}

	context.Apps = appsv1.NewFromControllerFactory(context.ControllerFactory)
	context.Core = corev1.NewFromControllerFactory(context.ControllerFactory)
	context.Project = projectv3.NewFromControllerFactory(context.ControllerFactory)
	context.Storage = storagev1.NewFromControllerFactory(context.ControllerFactory)
	context.RBAC = rbacv1.NewFromControllerFactory(context.ControllerFactory)
	context.Extensions = extv1beta1.NewFromControllerFactory(context.ControllerFactory)
	context.Policy = policyv1beta1.NewFromControllerFactory(context.ControllerFactory)
	context.BatchV1 = batchv1.NewFromControllerFactory(context.ControllerFactory)
	context.Autoscaling = autoscaling.NewFromControllerFactory(context.ControllerFactory)
	context.Monitoring = monitoringv1.NewFromControllerFactory(context.ControllerFactory)
	context.Cluster = clusterv3.NewFromControllerFactory(context.ControllerFactory)
	context.APIRegistration = apiregistrationv1.NewFromControllerFactory(context.ControllerFactory)
	context.Networking = knetworkingv1.NewFromControllerFactory(context.ControllerFactory)

	dynamicConfig := context.RESTConfig
	if dynamicConfig.NegotiatedSerializer == nil {
		dynamicConfig.NegotiatedSerializer = dynamic.NegotiatedSerializer
	}

	context.UnversionedClient, err = restwatch.UnversionedRESTClientFor(&dynamicConfig)
	if err != nil {
		return nil, err
	}

	context.Schemas = types.NewSchemas().
		AddSchemas(managementSchema.Schemas).
		AddSchemas(clusterSchema.Schemas).
		AddSchemas(projectSchema.Schemas)

	return context, err
}

func (w *UserOnlyContext) Start(ctx context.Context) error {
	logrus.Info("Starting workload controllers")
	return w.ControllerFactory.Start(ctx, 5)
}
