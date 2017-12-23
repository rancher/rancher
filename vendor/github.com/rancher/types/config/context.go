package config

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/event"
	"github.com/rancher/norman/signal"
	"github.com/rancher/norman/types"
	appsv1beta2 "github.com/rancher/types/apis/apps/v1beta2"
	corev1 "github.com/rancher/types/apis/core/v1"
	extv1beta1 "github.com/rancher/types/apis/extensions/v1beta1"
	managementv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	managementSchema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	projectv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	projectSchema "github.com/rancher/types/apis/project.cattle.io/v3/schema"
	rbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	projectClient "github.com/rancher/types/client/project/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
)

var (
	ProjectTypes = []string{
		projectClient.BasicAuthType,
		projectClient.CertificateType,
		projectClient.DockerCredentialType,
		projectClient.ServiceAccountTokenType,
		projectClient.SecretType,
		projectClient.SSHAuthType,
	}
)

type ManagementContext struct {
	eventBroadcaster record.EventBroadcaster

	LocalConfig       *rest.Config
	RESTConfig        rest.Config
	UnversionedClient rest.Interface
	K8sClient         kubernetes.Interface
	Events            record.EventRecorder
	EventLogger       event.Logger
	Schemas           *types.Schemas
	Scheme            *runtime.Scheme

	Management managementv3.Interface
	RBAC       rbacv1.Interface
}

func (c *ManagementContext) controllers() []controller.Starter {
	return []controller.Starter{
		c.Management,
		c.RBAC,
	}
}

type ClusterContext struct {
	Management        *ManagementContext
	ClusterName       string
	RESTConfig        rest.Config
	UnversionedClient rest.Interface
	K8sClient         kubernetes.Interface

	Apps       appsv1beta2.Interface
	Project    projectv3.Interface
	Core       corev1.Interface
	RBAC       rbacv1.Interface
	Extensions extv1beta1.Interface
}

func (w *ClusterContext) controllers() []controller.Starter {
	return []controller.Starter{
		w.Apps,
		w.Project,
		w.Core,
		w.RBAC,
		w.Extensions,
	}
}

func (w *ClusterContext) WorkloadContext() *WorkloadContext {
	return &WorkloadContext{
		ClusterName:       w.ClusterName,
		RESTConfig:        w.RESTConfig,
		UnversionedClient: w.UnversionedClient,
		K8sClient:         w.K8sClient,

		Apps:       w.Apps,
		Project:    w.Project,
		Core:       w.Core,
		RBAC:       w.RBAC,
		Extensions: w.Extensions,
	}
}

type WorkloadContext struct {
	ClusterName       string
	RESTConfig        rest.Config
	UnversionedClient rest.Interface
	K8sClient         kubernetes.Interface

	Apps       appsv1beta2.Interface
	Project    projectv3.Interface
	Core       corev1.Interface
	RBAC       rbacv1.Interface
	Extensions extv1beta1.Interface
}

func (w *WorkloadContext) controllers() []controller.Starter {
	return []controller.Starter{
		w.Apps,
		w.Project,
		w.Core,
		w.RBAC,
		w.Extensions,
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

	context.K8sClient, err = kubernetes.NewForConfig(&config)
	if err != nil {
		return nil, err
	}

	context.RBAC, err = rbacv1.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	dynamicConfig := config
	if dynamicConfig.NegotiatedSerializer == nil {
		configConfig := dynamic.ContentConfig()
		dynamicConfig.NegotiatedSerializer = configConfig.NegotiatedSerializer
	}

	context.UnversionedClient, err = rest.UnversionedRESTClientFor(&dynamicConfig)
	if err != nil {
		return nil, err
	}

	context.Schemas = types.NewSchemas().
		AddSchemas(managementSchema.Schemas)

	for _, projectType := range ProjectTypes {
		schema := projectSchema.Schemas.Schema(&projectSchema.Version, projectType)
		context.Schemas.AddSchema(*schema)
	}

	context.Scheme = runtime.NewScheme()
	managementv3.AddToScheme(context.Scheme)

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

func NewClusterContext(managementConfig, config rest.Config, clusterName string) (*ClusterContext, error) {
	var err error
	context := &ClusterContext{
		RESTConfig:  config,
		ClusterName: clusterName,
	}

	context.Management, err = NewManagementContext(managementConfig)
	if err != nil {
		return nil, err
	}

	context.K8sClient, err = kubernetes.NewForConfig(&config)
	if err != nil {
		return nil, err
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

	context.Extensions, err = extv1beta1.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	dynamicConfig := config
	if dynamicConfig.NegotiatedSerializer == nil {
		configConfig := dynamic.ContentConfig()
		dynamicConfig.NegotiatedSerializer = configConfig.NegotiatedSerializer
	}

	context.UnversionedClient, err = rest.UnversionedRESTClientFor(&dynamicConfig)
	if err != nil {
		return nil, err
	}

	return context, err
}

func (w *ClusterContext) Start(ctx context.Context) error {
	logrus.Info("Starting cluster controllers")
	controllers := w.Management.controllers()
	controllers = append(controllers, w.controllers()...)
	return controller.SyncThenStart(ctx, 5, controllers...)
}

func (w *ClusterContext) StartAndWait(ctx context.Context) error {
	ctx = signal.SigTermCancelContext(ctx)
	w.Start(ctx)
	<-ctx.Done()
	return ctx.Err()
}

func NewWorkloadContext(config rest.Config, clusterName string) (*WorkloadContext, error) {
	var err error
	context := &WorkloadContext{
		RESTConfig:  config,
		ClusterName: clusterName,
	}

	context.K8sClient, err = kubernetes.NewForConfig(&config)
	if err != nil {
		return nil, err
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

	context.Extensions, err = extv1beta1.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	dynamicConfig := config
	if dynamicConfig.NegotiatedSerializer == nil {
		configConfig := dynamic.ContentConfig()
		dynamicConfig.NegotiatedSerializer = configConfig.NegotiatedSerializer
	}

	context.UnversionedClient, err = rest.UnversionedRESTClientFor(&dynamicConfig)
	if err != nil {
		return nil, err
	}

	return context, err
}

func (w *WorkloadContext) Start(ctx context.Context) error {
	logrus.Info("Starting workload controllers")
	return controller.SyncThenStart(ctx, 5, w.controllers()...)
}

func (w *WorkloadContext) StartAndWait(ctx context.Context) error {
	ctx = signal.SigTermCancelContext(ctx)
	w.Start(ctx)
	<-ctx.Done()
	return ctx.Err()
}
