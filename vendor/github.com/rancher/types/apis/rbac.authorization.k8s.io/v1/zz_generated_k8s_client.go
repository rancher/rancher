package v1

import (
	"context"
	"sync"

	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/controller"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type Interface interface {
	RESTClient() rest.Interface
	controller.Starter

	RoleBindingsGetter
	ClusterRoleBindingsGetter
	ClusterRolesGetter
}

type Client struct {
	sync.Mutex
	restClient rest.Interface
	starters   []controller.Starter

	roleBindingControllers        map[string]RoleBindingController
	clusterRoleBindingControllers map[string]ClusterRoleBindingController
	clusterRoleControllers        map[string]ClusterRoleController
}

func NewForConfig(config rest.Config) (Interface, error) {
	if config.NegotiatedSerializer == nil {
		configConfig := dynamic.ContentConfig()
		config.NegotiatedSerializer = configConfig.NegotiatedSerializer
	}

	restClient, err := rest.UnversionedRESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	return &Client{
		restClient: restClient,

		roleBindingControllers:        map[string]RoleBindingController{},
		clusterRoleBindingControllers: map[string]ClusterRoleBindingController{},
		clusterRoleControllers:        map[string]ClusterRoleController{},
	}, nil
}

func (c *Client) RESTClient() rest.Interface {
	return c.restClient
}

func (c *Client) Sync(ctx context.Context) error {
	return controller.Sync(ctx, c.starters...)
}

func (c *Client) Start(ctx context.Context, threadiness int) error {
	return controller.Start(ctx, threadiness, c.starters...)
}

type RoleBindingsGetter interface {
	RoleBindings(namespace string) RoleBindingInterface
}

func (c *Client) RoleBindings(namespace string) RoleBindingInterface {
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &RoleBindingResource, RoleBindingGroupVersionKind, roleBindingFactory{})
	return &roleBindingClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ClusterRoleBindingsGetter interface {
	ClusterRoleBindings(namespace string) ClusterRoleBindingInterface
}

func (c *Client) ClusterRoleBindings(namespace string) ClusterRoleBindingInterface {
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &ClusterRoleBindingResource, ClusterRoleBindingGroupVersionKind, clusterRoleBindingFactory{})
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
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &ClusterRoleResource, ClusterRoleGroupVersionKind, clusterRoleFactory{})
	return &clusterRoleClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
