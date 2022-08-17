package v1

import (
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/norman/generator"
	"github.com/rancher/norman/objectclient"
	"github.com/sirupsen/logrus"
	"k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

type Interface interface {
	ClusterRoleBindingsGetter
	ClusterRolesGetter
	RoleBindingsGetter
	RolesGetter
}

type Client struct {
	userAgent         string
	controllerFactory controller.SharedControllerFactory
	clientFactory     client.SharedClientFactory
}

func NewForConfig(cfg rest.Config) (Interface, error) {
	scheme := runtime.NewScheme()
	if err := v1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	sharedOpts := &controller.SharedControllerFactoryOptions{
		SyncOnlyChangedObjects: generator.SyncOnlyChangedObjects(),
	}
	controllerFactory, err := controller.NewSharedControllerFactoryFromConfigWithOptions(&cfg, scheme, sharedOpts)
	if err != nil {
		return nil, err
	}
	return NewFromControllerFactory(controllerFactory), nil
}

func NewFromControllerFactory(factory controller.SharedControllerFactory) Interface {
	return &Client{
		controllerFactory: factory,
		clientFactory:     factory.SharedCacheFactory().SharedClientFactory(),
	}
}

func NewFromControllerFactoryWithAgent(userAgent string, factory controller.SharedControllerFactory) Interface {
	return &Client{
		userAgent:         userAgent,
		controllerFactory: factory,
		clientFactory:     factory.SharedCacheFactory().SharedClientFactory(),
	}
}

type ClusterRoleBindingsGetter interface {
	ClusterRoleBindings(namespace string) ClusterRoleBindingInterface
}

func (c *Client) ClusterRoleBindings(namespace string) ClusterRoleBindingInterface {
	sharedClient := c.clientFactory.ForResourceKind(ClusterRoleBindingGroupVersionResource, ClusterRoleBindingGroupVersionKind.Kind, false)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [ClusterRoleBindings] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &ClusterRoleBindingResource, ClusterRoleBindingGroupVersionKind, clusterRoleBindingFactory{})
	return &clusterRoleBindingClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ClusterRolesGetter interface {
	ClusterRoles(namespace string) ClusterRoleInterface
}

func (c *Client) ClusterRoles(namespace string) ClusterRoleInterface {
	sharedClient := c.clientFactory.ForResourceKind(ClusterRoleGroupVersionResource, ClusterRoleGroupVersionKind.Kind, false)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [ClusterRoles] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &ClusterRoleResource, ClusterRoleGroupVersionKind, clusterRoleFactory{})
	return &clusterRoleClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type RoleBindingsGetter interface {
	RoleBindings(namespace string) RoleBindingInterface
}

func (c *Client) RoleBindings(namespace string) RoleBindingInterface {
	sharedClient := c.clientFactory.ForResourceKind(RoleBindingGroupVersionResource, RoleBindingGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [RoleBindings] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &RoleBindingResource, RoleBindingGroupVersionKind, roleBindingFactory{})
	return &roleBindingClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type RolesGetter interface {
	Roles(namespace string) RoleInterface
}

func (c *Client) Roles(namespace string) RoleInterface {
	sharedClient := c.clientFactory.ForResourceKind(RoleGroupVersionResource, RoleGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [Roles] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &RoleResource, RoleGroupVersionKind, roleFactory{})
	return &roleClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
