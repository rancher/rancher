package config

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/signal"
	appsv1beta2 "github.com/rancher/types/apis/apps/v1beta2"
	corev1 "github.com/rancher/types/apis/core/v1"
	extv1beta1 "github.com/rancher/types/apis/extensions/v1beta1"
	managementv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	projectv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	rbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type ManagementContext struct {
	LocalConfig       *rest.Config
	RESTConfig        rest.Config
	UnversionedClient rest.Interface

	Management managementv3.Interface
}

func (c *ManagementContext) controllers() []controller.Starter {
	return []controller.Starter{
		c.Management,
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

func NewManagementContext(config rest.Config) (*ManagementContext, error) {
	var err error

	context := &ManagementContext{
		RESTConfig: config,
	}

	context.Management, err = managementv3.NewForConfig(config)
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

func (c *ManagementContext) Start(ctx context.Context) error {
	logrus.Info("Starting management controllers")
	return controller.SyncThenSync(ctx, 5, c.controllers()...)
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
	return controller.SyncThenSync(ctx, 5, controllers...)
}

func (w *ClusterContext) StartAndWait(ctx context.Context) error {
	ctx = signal.SigTermCancelContext(ctx)
	w.Start(ctx)
	<-ctx.Done()
	return ctx.Err()
}
