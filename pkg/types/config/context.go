package config

import (
	"context"
	"time"

	catalogv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"

	"github.com/rancher/wrangler/pkg/generic"

	prommonitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	istiov1alpha3api "github.com/knative/pkg/apis/istio/v1alpha3"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/norman/objectclient/dynamic"
	"github.com/rancher/norman/restwatch"
	"github.com/rancher/norman/store/proxy"
	"github.com/rancher/norman/types"
	clusterv3api "github.com/rancher/rancher/pkg/apis/cluster.cattle.io/v3"
	managementv3api "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	projectv3api "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	apiregistrationv1 "github.com/rancher/rancher/pkg/generated/norman/apiregistration.k8s.io/v1"
	appsv1 "github.com/rancher/rancher/pkg/generated/norman/apps/v1"
	autoscaling "github.com/rancher/rancher/pkg/generated/norman/autoscaling/v2beta2"
	batchv1 "github.com/rancher/rancher/pkg/generated/norman/batch/v1"
	batchv1beta1 "github.com/rancher/rancher/pkg/generated/norman/batch/v1beta1"
	clusterv3 "github.com/rancher/rancher/pkg/generated/norman/cluster.cattle.io/v3"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	extv1beta1 "github.com/rancher/rancher/pkg/generated/norman/extensions/v1beta1"
	managementv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	monitoringv1 "github.com/rancher/rancher/pkg/generated/norman/monitoring.coreos.com/v1"
	istiov1alpha3 "github.com/rancher/rancher/pkg/generated/norman/networking.istio.io/v1alpha3"
	knetworkingv1 "github.com/rancher/rancher/pkg/generated/norman/networking.k8s.io/v1"
	policyv1beta1 "github.com/rancher/rancher/pkg/generated/norman/policy/v1beta1"
	projectv3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	rbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	storagev1 "github.com/rancher/rancher/pkg/generated/norman/storage.k8s.io/v1"
	clusterSchema "github.com/rancher/rancher/pkg/schemas/cluster.cattle.io/v3"
	managementSchema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	projectSchema "github.com/rancher/rancher/pkg/schemas/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config/dialer"
	"github.com/rancher/rancher/pkg/types/peermanager"
	"github.com/rancher/rancher/pkg/types/user"
	"github.com/rancher/wrangler/pkg/generated/controllers/rbac"
	wrbacv1 "github.com/rancher/wrangler/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/pkg/schemes"
	"github.com/sirupsen/logrus"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	k8dynamic "k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	apiregistrationv12 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
)

var (
	UserStorageContext       types.StorageContext = "user"
	ManagementStorageContext types.StorageContext = "mgmt"
	localSchemeBuilder                            = runtime.SchemeBuilder{
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

type ScaledContext struct {
	ClientGetter      proxy.ClientGetter
	KubeConfig        clientcmdapi.Config
	RESTConfig        rest.Config
	ControllerFactory controller.SharedControllerFactory
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
	c.managementContext = mgmt
	return mgmt, nil
}

func NewScaledContext(config rest.Config) (*ScaledContext, error) {
	var err error

	context := &ScaledContext{
		RESTConfig: config,
	}

	controllerFactory, err := controller.NewSharedControllerFactoryFromConfig(&context.RESTConfig, Scheme)
	if err != nil {
		return nil, err
	}
	context.ControllerFactory = controllerFactory

	context.Management, err = managementv3.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.Project, err = projectv3.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.K8sClient, err = kubernetes.NewForConfig(&config)
	if err != nil {
		return nil, err
	}

	context.RBAC, err = rbacv1.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.Core, err = corev1.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}
	context.Project, err = projectv3.NewFromControllerFactory(controllerFactory)
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

	Management managementv3.Interface
	Project    projectv3.Interface
	RBAC       rbacv1.Interface
	Core       corev1.Interface
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
	BatchV1Beta1    batchv1beta1.Interface
	Monitoring      monitoringv1.Interface
	Cluster         clusterv3.Interface
	Istio           istiov1alpha3.Interface
	Storage         storagev1.Interface
	Policy          policyv1beta1.Interface
}

func newManagementContext(c *ScaledContext) (*ManagementContext, error) {
	var err error

	context := &ManagementContext{
		RESTConfig: c.RESTConfig,
	}

	config := c.RESTConfig
	controllerFactory := c.ControllerFactory
	context.ControllerFactory = controllerFactory

	context.Management, err = managementv3.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.Project, err = projectv3.NewFromControllerFactory(controllerFactory)
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

	context.RBAC, err = rbacv1.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.Core, err = corev1.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}
	context.Project, err = projectv3.NewFromControllerFactory(controllerFactory)
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

	context.Scheme = Scheme

	return context, err
}

func (c *ManagementContext) Start(ctx context.Context) error {
	logrus.Info("Starting management controllers")
	return c.ControllerFactory.Start(ctx, 50)
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

	controllerFactory, err := controller.NewSharedControllerFactoryFromConfig(&config, Scheme)
	if err != nil {
		return nil, err
	}
	context.ControllerFactory = controllerFactory

	context.K8sClient, err = kubernetes.NewForConfig(&config)
	if err != nil {
		return nil, err
	}

	context.Apps, err = appsv1.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.Core, err = corev1.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.Project, err = projectv3.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.Storage, err = storagev1.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.RBAC, err = rbacv1.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.Networking, err = knetworkingv1.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.Extensions, err = extv1beta1.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.Policy, err = policyv1beta1.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.BatchV1, err = batchv1.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.BatchV1Beta1, err = batchv1beta1.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.Autoscaling, err = autoscaling.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.Monitoring, err = monitoringv1.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.Cluster, err = clusterv3.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.Istio, err = istiov1alpha3.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.APIAggregation, err = apiregistrationv1.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	wranglerConf := config
	wranglerConf.Timeout = 30 * time.Minute
	opts := &generic.FactoryOptions{
		SharedControllerFactory: controllerFactory,
	}
	context.rbacw, err = rbac.NewFactoryFromConfigWithOptions(&wranglerConf, opts)
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
	if err := w.Management.ControllerFactory.Start(w.runContext, 50); err != nil {
		return err
	}
	return w.ControllerFactory.Start(ctx, 5)
}

func NewUserOnlyContext(config rest.Config) (*UserOnlyContext, error) {
	var err error
	context := &UserOnlyContext{
		RESTConfig: config,
	}

	controllerFactory, err := controller.NewSharedControllerFactoryFromConfig(&config, Scheme)
	if err != nil {
		return nil, err
	}
	context.ControllerFactory = controllerFactory

	context.K8sClient, err = kubernetes.NewForConfig(&config)
	if err != nil {
		return nil, err
	}

	context.Apps, err = appsv1.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.Core, err = corev1.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.Project, err = projectv3.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.Storage, err = storagev1.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.RBAC, err = rbacv1.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.Extensions, err = extv1beta1.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.Policy, err = policyv1beta1.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.BatchV1, err = batchv1.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.BatchV1Beta1, err = batchv1beta1.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.Autoscaling, err = autoscaling.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.Monitoring, err = monitoringv1.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.Cluster, err = clusterv3.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.Istio, err = istiov1alpha3.NewFromControllerFactory(controllerFactory)
	if err != nil {
		return nil, err
	}

	context.APIRegistration, err = apiregistrationv1.NewFromControllerFactory(controllerFactory)
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
	return w.ControllerFactory.Start(ctx, 5)
}
