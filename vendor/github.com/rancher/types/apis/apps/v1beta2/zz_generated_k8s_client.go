package v1beta2

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

	DeploymentsGetter
	DaemonSetsGetter
	StatefulSetsGetter
	ReplicaSetsGetter
}

type Clients struct {
	Interface Interface

	Deployment  DeploymentClient
	DaemonSet   DaemonSetClient
	StatefulSet StatefulSetClient
	ReplicaSet  ReplicaSetClient
}

type Client struct {
	sync.Mutex
	restClient rest.Interface
	starters   []controller.Starter

	deploymentControllers  map[string]DeploymentController
	daemonSetControllers   map[string]DaemonSetController
	statefulSetControllers map[string]StatefulSetController
	replicaSetControllers  map[string]ReplicaSetController
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

		Deployment: &deploymentClient2{
			iface: iface.Deployments(""),
		},
		DaemonSet: &daemonSetClient2{
			iface: iface.DaemonSets(""),
		},
		StatefulSet: &statefulSetClient2{
			iface: iface.StatefulSets(""),
		},
		ReplicaSet: &replicaSetClient2{
			iface: iface.ReplicaSets(""),
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

		deploymentControllers:  map[string]DeploymentController{},
		daemonSetControllers:   map[string]DaemonSetController{},
		statefulSetControllers: map[string]StatefulSetController{},
		replicaSetControllers:  map[string]ReplicaSetController{},
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

type DeploymentsGetter interface {
	Deployments(namespace string) DeploymentInterface
}

func (c *Client) Deployments(namespace string) DeploymentInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &DeploymentResource, DeploymentGroupVersionKind, deploymentFactory{})
	return &deploymentClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type DaemonSetsGetter interface {
	DaemonSets(namespace string) DaemonSetInterface
}

func (c *Client) DaemonSets(namespace string) DaemonSetInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &DaemonSetResource, DaemonSetGroupVersionKind, daemonSetFactory{})
	return &daemonSetClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type StatefulSetsGetter interface {
	StatefulSets(namespace string) StatefulSetInterface
}

func (c *Client) StatefulSets(namespace string) StatefulSetInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &StatefulSetResource, StatefulSetGroupVersionKind, statefulSetFactory{})
	return &statefulSetClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ReplicaSetsGetter interface {
	ReplicaSets(namespace string) ReplicaSetInterface
}

func (c *Client) ReplicaSets(namespace string) ReplicaSetInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ReplicaSetResource, ReplicaSetGroupVersionKind, replicaSetFactory{})
	return &replicaSetClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
