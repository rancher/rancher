package v1

import (
	"context"
	"sync"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/objectclient/dynamic"
	"github.com/rancher/norman/restwatch"
	"k8s.io/client-go/rest"
)

type (
	contextKeyType        struct{}
	contextClientsKeyType struct{}
)

type Interface interface {
	RESTClient() rest.Interface
	controller.Starter

	ClusterRoleBindingsGetter
	ClusterRolesGetter
	RoleBindingsGetter
	RolesGetter
}

type Clients struct {
	Interface Interface

	ClusterRoleBinding ClusterRoleBindingClient
	ClusterRole        ClusterRoleClient
	RoleBinding        RoleBindingClient
	Role               RoleClient
}

type Client struct {
	sync.Mutex
	restClient rest.Interface
	starters   []controller.Starter

	clusterRoleBindingControllers map[string]ClusterRoleBindingController
	clusterRoleControllers        map[string]ClusterRoleController
	roleBindingControllers        map[string]RoleBindingController
	roleControllers               map[string]RoleController
}

func Factory(ctx context.Context, config rest.Config) (context.Context, controller.Starter, error) {
	c, err := NewForConfig(config)
	if err != nil {
		return ctx, nil, err
	}

	cs := NewClientsFromInterface(c)

	ctx = context.WithValue(ctx, contextKeyType{}, c)
	ctx = context.WithValue(ctx, contextClientsKeyType{}, cs)
	return ctx, c, nil
}

func ClientsFrom(ctx context.Context) *Clients {
	return ctx.Value(contextClientsKeyType{}).(*Clients)
}

func From(ctx context.Context) Interface {
	return ctx.Value(contextKeyType{}).(Interface)
}

func NewClients(config rest.Config) (*Clients, error) {
	iface, err := NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return NewClientsFromInterface(iface), nil
}

func NewClientsFromInterface(iface Interface) *Clients {
	return &Clients{
		Interface: iface,

		ClusterRoleBinding: &clusterRoleBindingClient2{
			iface: iface.ClusterRoleBindings(""),
		},
		ClusterRole: &clusterRoleClient2{
			iface: iface.ClusterRoles(""),
		},
		RoleBinding: &roleBindingClient2{
			iface: iface.RoleBindings(""),
		},
		Role: &roleClient2{
			iface: iface.Roles(""),
		},
	}
}

func NewForConfig(config rest.Config) (Interface, error) {
	if config.NegotiatedSerializer == nil {
		config.NegotiatedSerializer = dynamic.NegotiatedSerializer
	}

	restClient, err := restwatch.UnversionedRESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	return &Client{
		restClient: restClient,

		clusterRoleBindingControllers: map[string]ClusterRoleBindingController{},
		clusterRoleControllers:        map[string]ClusterRoleController{},
		roleBindingControllers:        map[string]RoleBindingController{},
		roleControllers:               map[string]RoleController{},
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

type ClusterRoleBindingsGetter interface {
	ClusterRoleBindings(namespace string) ClusterRoleBindingInterface
}

func (c *Client) ClusterRoleBindings(namespace string) ClusterRoleBindingInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ClusterRoleBindingResource, ClusterRoleBindingGroupVersionKind, clusterRoleBindingFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ClusterRoleResource, ClusterRoleGroupVersionKind, clusterRoleFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &RoleBindingResource, RoleBindingGroupVersionKind, roleBindingFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &RoleResource, RoleGroupVersionKind, roleFactory{})
	return &roleClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
