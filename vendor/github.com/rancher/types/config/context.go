package config

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient/dynamic"
	"github.com/rancher/norman/restwatch"
	"github.com/rancher/norman/store/proxy"
	"github.com/rancher/norman/types"
	apiregistrationv1 "github.com/rancher/types/apis/apiregistration.k8s.io/v1"
	appsv1 "github.com/rancher/types/apis/apps/v1"
	autoscaling "github.com/rancher/types/apis/autoscaling/v2beta2"
	batchv1 "github.com/rancher/types/apis/batch/v1"
	batchv1beta1 "github.com/rancher/types/apis/batch/v1beta1"
	clusterv3 "github.com/rancher/types/apis/cluster.cattle.io/v3"
	clusterSchema "github.com/rancher/types/apis/cluster.cattle.io/v3/schema"
	corev1 "github.com/rancher/types/apis/core/v1"
	extv1beta1 "github.com/rancher/types/apis/extensions/v1beta1"
	managementv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	managementSchema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	monitoringv1 "github.com/rancher/types/apis/monitoring.coreos.com/v1"
	istiov1alpha3 "github.com/rancher/types/apis/networking.istio.io/v1alpha3"
	knetworkingv1 "github.com/rancher/types/apis/networking.k8s.io/v1"
	policyv1beta1 "github.com/rancher/types/apis/policy/v1beta1"
	projectv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	projectSchema "github.com/rancher/types/apis/project.cattle.io/v3/schema"
	rbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	storagev1 "github.com/rancher/types/apis/storage.k8s.io/v1"
	"github.com/rancher/types/config/dialer"
	"github.com/rancher/types/peermanager"
	"github.com/rancher/types/user"
	"github.com/rancher/wrangler-api/pkg/generated/controllers/rbac"
	wrbacv1 "github.com/rancher/wrangler-api/pkg/generated/controllers/rbac/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/runtime"
	k8dynamic "k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var (
	UserStorageContext       types.StorageContext = "user"
	ManagementStorageContext types.StorageContext = "mgmt"
)

type ScaledContext struct {
	ClientGetter      proxy.ClientGetter
	KubeConfig        clientcmdapi.Config
	RESTConfig        rest.Config
	UnversionedClient rest.Interface
	K8sClient         kubernetes.Interface
	APIExtClient      clientset.Interface
	Schemas           *types.Schemas
	AccessControl     types.AccessControl
	Dialer            dialer.Factory
	UserManager       user.Manager
	PeerManager       peermanager.PeerManager

	Management managementv3.Interface
	Project    projectv3.Interface
	RBAC       rbacv1.Interface
	Core       corev1.Interface
	Storage    storagev1.Interface

	RunContext        context.Context
	managementContext *ManagementContext
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
	if c.managementContext != nil {
		return c.managementContext, nil
	}
	mgmt, err := NewManagementContext(c.RESTConfig)
	if err != nil {
		return nil, err
	}
	mgmt.Dialer = c.Dialer
	mgmt.UserManager = c.UserManager
	c.managementContext = mgmt
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
	return controller.SyncThenStart(ctx, 5, c.controllers()...)
}

type ManagementContext struct {
	ClientGetter      proxy.ClientGetter
	RESTConfig        rest.Config
	UnversionedClient rest.Interface
	DynamicClient     k8dynamic.Interface
	K8sClient         kubernetes.Interface
	APIExtClient      clientset.Interface
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
	runContext        context.Context

	APIAggregation apiregistrationv1.Interface
	Apps           appsv1.Interface
	Autoscaling    autoscaling.Interface
	Project        projectv3.Interface
	Core           corev1.Interface
	RBAC           rbacv1.Interface
	Extensions     extv1beta1.Interface
	BatchV1        batchv1.Interface
	BatchV1Beta1   batchv1beta1.Interface
	Networking     knetworkingv1.Interface
	Monitoring     monitoringv1.Interface
	Cluster        clusterv3.Interface
	Istio          istiov1alpha3.Interface
	Storage        storagev1.Interface
	Policy         policyv1beta1.Interface

	RBACw wrbacv1.Interface
	rbacw *rbac.Factory
}

func (w *UserContext) controllers() []controller.Starter {
	return []controller.Starter{
		w.APIAggregation,
		w.Apps,
		w.Project,
		w.Core,
		w.RBAC,
		w.Extensions,
		w.BatchV1,
		w.BatchV1Beta1,
		w.Networking,
		w.Monitoring,
		w.Cluster,
		w.Storage,
		w.Policy,
		w.rbacw,
	}
}

func (w *UserContext) UserOnlyContext() *UserOnlyContext {
	return &UserOnlyContext{
		Schemas:           w.Management.Schemas,
		ClusterName:       w.ClusterName,
		RESTConfig:        w.RESTConfig,
		UnversionedClient: w.UnversionedClient,
		K8sClient:         w.K8sClient,

		Autoscaling:  w.Autoscaling,
		Apps:         w.Apps,
		Project:      w.Project,
		Core:         w.Core,
		RBAC:         w.RBAC,
		Extensions:   w.Extensions,
		BatchV1:      w.BatchV1,
		BatchV1Beta1: w.BatchV1Beta1,
		Monitoring:   w.Monitoring,
		Cluster:      w.Cluster,
		Istio:        w.Istio,
		Storage:      w.Storage,
		Policy:       w.Policy,
	}
}

type UserOnlyContext struct {
	Schemas           *types.Schemas
	ClusterName       string
	RESTConfig        rest.Config
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
	BatchV1Beta1    batchv1beta1.Interface
	Monitoring      monitoringv1.Interface
	Cluster         clusterv3.Interface
	Istio           istiov1alpha3.Interface
	Storage         storagev1.Interface
	Policy          policyv1beta1.Interface
}

func (w *UserOnlyContext) controllers() []controller.Starter {
	return []controller.Starter{
		w.APIRegistration,
		w.Apps,
		w.Project,
		w.Core,
		w.RBAC,
		w.Extensions,
		w.BatchV1,
		w.BatchV1Beta1,
		w.Monitoring,
		w.Storage,
		w.Policy,
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

	context.DynamicClient, err = k8dynamic.NewForConfig(&config)
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

	context.Scheme = runtime.NewScheme()
	managementv3.AddToScheme(context.Scheme)
	projectv3.AddToScheme(context.Scheme)

	return context, err
}

func (c *ManagementContext) Start(ctx context.Context) error {
	logrus.Info("Starting management controllers")

	return controller.SyncThenStart(ctx, 50, c.controllers()...)
}

func NewUserContext(scaledContext *ScaledContext, config rest.Config, clusterName string) (*UserContext, error) {
	var err error
	context := &UserContext{
		RESTConfig:  config,
		ClusterName: clusterName,
		runContext:  scaledContext.RunContext,
	}

	context.Management, err = scaledContext.NewManagementContext()
	if err != nil {
		return nil, err
	}

	context.K8sClient, err = kubernetes.NewForConfig(&config)
	if err != nil {
		return nil, err
	}

	context.Apps, err = appsv1.NewForConfig(config)
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

	context.Storage, err = storagev1.NewForConfig(config)
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

	context.Policy, err = policyv1beta1.NewForConfig(config)
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

	context.Autoscaling, err = autoscaling.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	context.Monitoring, err = monitoringv1.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	context.Cluster, err = clusterv3.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	context.Istio, err = istiov1alpha3.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	context.APIAggregation, err = apiregistrationv1.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	wranglerConf := config
	wranglerConf.Timeout = 30 * time.Minute
	context.rbacw, err = rbac.NewFactoryFromConfig(&wranglerConf)
	if err != nil {
		return nil, err
	}
	context.RBACw = context.rbacw.Rbac().V1()

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
	if err := controller.SyncThenStart(w.runContext, 50, w.Management.controllers()...); err != nil {
		return err
	}
	return controller.SyncThenStart(ctx, 5, w.controllers()...)
}

func NewUserOnlyContext(config rest.Config) (*UserOnlyContext, error) {
	var err error
	context := &UserOnlyContext{
		RESTConfig: config,
	}

	context.K8sClient, err = kubernetes.NewForConfig(&config)
	if err != nil {
		return nil, err
	}

	context.Apps, err = appsv1.NewForConfig(config)
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

	context.Storage, err = storagev1.NewForConfig(config)
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

	context.Policy, err = policyv1beta1.NewForConfig(config)
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

	context.Autoscaling, err = autoscaling.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	context.Monitoring, err = monitoringv1.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	context.Cluster, err = clusterv3.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	context.Istio, err = istiov1alpha3.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	context.APIRegistration, err = apiregistrationv1.NewForConfig(config)
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

	context.Schemas = types.NewSchemas().
		AddSchemas(managementSchema.Schemas).
		AddSchemas(clusterSchema.Schemas).
		AddSchemas(projectSchema.Schemas)

	return context, err
}

func (w *UserOnlyContext) Start(ctx context.Context) error {
	logrus.Info("Starting workload controllers")
	return controller.SyncThenStart(ctx, 5, w.controllers()...)
}
