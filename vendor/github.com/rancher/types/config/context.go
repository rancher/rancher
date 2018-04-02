package config

import (
	"context"

	"github.com/pkg/errors"
	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/event"
	"github.com/rancher/norman/restwatch"
	"github.com/rancher/norman/signal"
	"github.com/rancher/norman/store/proxy"
	"github.com/rancher/norman/types"
	appsv1beta2 "github.com/rancher/types/apis/apps/v1beta2"
	batchv1 "github.com/rancher/types/apis/batch/v1"
	batchv1beta1 "github.com/rancher/types/apis/batch/v1beta1"
	clusterSchema "github.com/rancher/types/apis/cluster.cattle.io/v3/schema"
	corev1 "github.com/rancher/types/apis/core/v1"
	extv1beta1 "github.com/rancher/types/apis/extensions/v1beta1"
	managementv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	managementSchema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	knetworkingv1 "github.com/rancher/types/apis/networking.k8s.io/v1"
	projectv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	projectSchema "github.com/rancher/types/apis/project.cattle.io/v3/schema"
	rbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/rancher/types/config/dialer"
	"github.com/rancher/types/user"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
)

var (
	UserStorageContext       types.StorageContext = "user"
	ManagementStorageContext types.StorageContext = "mgmt"
)

type ScaledContext struct {
	ClientGetter      proxy.ClientGetter
	LocalConfig       *rest.Config
	RESTConfig        rest.Config
	UnversionedClient rest.Interface
	K8sClient         kubernetes.Interface
	APIExtClient      clientset.Interface
	Schemas           *types.Schemas
	AccessControl     types.AccessControl
	Dialer            dialer.Factory
	UserManager       user.Manager
	Leader            bool

	Management managementv3.Interface
	Project    projectv3.Interface
	RBAC       rbacv1.Interface
	Core       corev1.Interface
}

func (c *ScaledContext) controllers() []controller.Starter {
	return []controller.Starter{
		c.Management,
		c.Project,
		c.RBAC,
		c.Core,
	}
}

func (c *ScaledContext) NewManagementContext() (*ManagementContext, error) {
	mgmt, err := NewManagementContext(c.RESTConfig)
	if err != nil {
		return nil, err
	}
	mgmt.Dialer = c.Dialer
	mgmt.UserManager = c.UserManager
	return mgmt, nil
}

func NewScaledContext(config rest.Config) (*ScaledContext, error) {
	var err error

	context := &ScaledContext{
		RESTConfig: config,
	}

	context.Management, err = managementv3.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	context.Project, err = projectv3.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	context.K8sClient, err = kubernetes.NewForConfig(&config)
	if err != nil {
		return nil, err
	}

	context.RBAC, err = rbacv1.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	context.Core, err = corev1.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	context.Project, err = projectv3.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	dynamicConfig := config
	if dynamicConfig.NegotiatedSerializer == nil {
		configConfig := dynamic.ContentConfig()
		dynamicConfig.NegotiatedSerializer = configConfig.NegotiatedSerializer
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
	return controller.SyncThenStart(ctx, 5, c.controllers()...)
}

type ManagementContext struct {
	eventBroadcaster record.EventBroadcaster

	ClientGetter      proxy.ClientGetter
	LocalConfig       *rest.Config
	RESTConfig        rest.Config
	UnversionedClient rest.Interface
	K8sClient         kubernetes.Interface
	APIExtClient      clientset.Interface
	Events            record.EventRecorder
	EventLogger       event.Logger
	Schemas           *types.Schemas
	Scheme            *runtime.Scheme
	Dialer            dialer.Factory
	UserManager       user.Manager

	Management managementv3.Interface
	Project    projectv3.Interface
	RBAC       rbacv1.Interface
	Core       corev1.Interface
}

func (c *ManagementContext) controllers() []controller.Starter {
	return []controller.Starter{
		c.Management,
		c.Project,
		c.RBAC,
		c.Core,
	}
}

type UserContext struct {
	Management        *ManagementContext
	ClusterName       string
	RESTConfig        rest.Config
	UnversionedClient rest.Interface
	APIExtClient      clientset.Interface
	K8sClient         kubernetes.Interface

	Apps         appsv1beta2.Interface
	Project      projectv3.Interface
	Core         corev1.Interface
	RBAC         rbacv1.Interface
	Extensions   extv1beta1.Interface
	BatchV1      batchv1.Interface
	BatchV1Beta1 batchv1beta1.Interface
	Networking   knetworkingv1.Interface
}

func (w *UserContext) controllers() []controller.Starter {
	return []controller.Starter{
		w.Apps,
		w.Project,
		w.Core,
		w.RBAC,
		w.Extensions,
		w.BatchV1,
		w.BatchV1Beta1,
		w.Networking,
	}
}

func (w *UserContext) UserOnlyContext() *UserOnlyContext {
	return &UserOnlyContext{
		Schemas:           w.Management.Schemas,
		ClusterName:       w.ClusterName,
		RESTConfig:        w.RESTConfig,
		UnversionedClient: w.UnversionedClient,
		K8sClient:         w.K8sClient,

		Apps:         w.Apps,
		Project:      w.Project,
		Core:         w.Core,
		RBAC:         w.RBAC,
		Extensions:   w.Extensions,
		BatchV1:      w.BatchV1,
		BatchV1Beta1: w.BatchV1Beta1,
	}
}

type UserOnlyContext struct {
	Schemas           *types.Schemas
	ClusterName       string
	RESTConfig        rest.Config
	UnversionedClient rest.Interface
	K8sClient         kubernetes.Interface

	Apps         appsv1beta2.Interface
	Project      projectv3.Interface
	Core         corev1.Interface
	RBAC         rbacv1.Interface
	Extensions   extv1beta1.Interface
	BatchV1      batchv1.Interface
	BatchV1Beta1 batchv1beta1.Interface
}

func (w *UserOnlyContext) controllers() []controller.Starter {
	return []controller.Starter{
		w.Apps,
		w.Project,
		w.Core,
		w.RBAC,
		w.Extensions,
		w.BatchV1,
		w.BatchV1Beta1,
	}
}

func NewManagementContext(config rest.Config) (*ManagementContext, error) {
	var err error

	context := &ManagementContext{
		RESTConfig: config,
	}

	context.Management, err = managementv3.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	context.Project, err = projectv3.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	context.K8sClient, err = kubernetes.NewForConfig(&config)
	if err != nil {
		return nil, err
	}

	context.RBAC, err = rbacv1.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	context.Core, err = corev1.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	context.Project, err = projectv3.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	dynamicConfig := config
	if dynamicConfig.NegotiatedSerializer == nil {
		configConfig := dynamic.ContentConfig()
		dynamicConfig.NegotiatedSerializer = configConfig.NegotiatedSerializer
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

	context.Scheme = runtime.NewScheme()
	managementv3.AddToScheme(context.Scheme)
	projectv3.AddToScheme(context.Scheme)

	context.eventBroadcaster = record.NewBroadcaster()
	context.Events = context.eventBroadcaster.NewRecorder(context.Scheme, v1.EventSource{
		Component: "CattleManagementServer",
	})
	context.EventLogger = event.NewLogger(context.Events)

	return context, err
}

func (c *ManagementContext) Start(ctx context.Context) error {
	logrus.Info("Starting management controllers")

	watcher := c.eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{
		Interface: c.K8sClient.CoreV1().Events(""),
	})

	go func() {
		<-ctx.Done()
		watcher.Stop()
	}()

	return controller.SyncThenStart(ctx, 5, c.controllers()...)
}

func (c *ManagementContext) StartAndWait() error {
	ctx := signal.SigTermCancelContext(context.Background())
	c.Start(ctx)
	<-ctx.Done()
	return ctx.Err()
}

func NewUserContext(scaledContext *ScaledContext, config rest.Config, clusterName string) (*UserContext, error) {
	var err error
	context := &UserContext{
		RESTConfig:  config,
		ClusterName: clusterName,
	}

	context.Management, err = scaledContext.NewManagementContext()
	if err != nil {
		return nil, err
	}

	context.K8sClient, err = kubernetes.NewForConfig(&config)
	if err != nil {
		return nil, err
	}

	_, err = context.K8sClient.Discovery().ServerVersion()
	if err != nil {
		return nil, errors.Wrap(err, "could not contact server")
	}

	context.Apps, err = appsv1beta2.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	context.Core, err = corev1.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	context.Project, err = projectv3.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	context.RBAC, err = rbacv1.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	context.Networking, err = knetworkingv1.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	context.Extensions, err = extv1beta1.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	context.BatchV1, err = batchv1.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	context.BatchV1Beta1, err = batchv1beta1.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	dynamicConfig := config
	if dynamicConfig.NegotiatedSerializer == nil {
		configConfig := dynamic.ContentConfig()
		dynamicConfig.NegotiatedSerializer = configConfig.NegotiatedSerializer
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
	controllers := w.Management.controllers()
	controllers = append(controllers, w.controllers()...)
	return controller.SyncThenStart(ctx, 5, controllers...)
}

func (w *UserContext) StartAndWait(ctx context.Context) error {
	ctx = signal.SigTermCancelContext(ctx)
	w.Start(ctx)
	<-ctx.Done()
	return ctx.Err()
}

func (w *UserOnlyContext) Start(ctx context.Context) error {
	logrus.Info("Starting workload controllers")
	return controller.SyncThenStart(ctx, 5, w.controllers()...)
}

func (w *UserOnlyContext) StartAndWait(ctx context.Context) error {
	ctx = signal.SigTermCancelContext(ctx)
	w.Start(ctx)
	<-ctx.Done()
	return ctx.Err()
}
