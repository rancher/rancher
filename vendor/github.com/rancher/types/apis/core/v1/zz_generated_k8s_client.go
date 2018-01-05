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

	NodesGetter
	ComponentStatusesGetter
	NamespacesGetter
	EventsGetter
	PodsGetter
	ServicesGetter
	SecretsGetter
}

type Client struct {
	sync.Mutex
	restClient rest.Interface
	starters   []controller.Starter

	nodeControllers            map[string]NodeController
	componentStatusControllers map[string]ComponentStatusController
	namespaceControllers       map[string]NamespaceController
	eventControllers           map[string]EventController
	podControllers             map[string]PodController
	serviceControllers         map[string]ServiceController
	secretControllers          map[string]SecretController
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

		nodeControllers:            map[string]NodeController{},
		componentStatusControllers: map[string]ComponentStatusController{},
		namespaceControllers:       map[string]NamespaceController{},
		eventControllers:           map[string]EventController{},
		podControllers:             map[string]PodController{},
		serviceControllers:         map[string]ServiceController{},
		secretControllers:          map[string]SecretController{},
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

type NodesGetter interface {
	Nodes(namespace string) NodeInterface
}

func (c *Client) Nodes(namespace string) NodeInterface {
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &NodeResource, NodeGroupVersionKind, nodeFactory{})
	return &nodeClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ComponentStatusesGetter interface {
	ComponentStatuses(namespace string) ComponentStatusInterface
}

func (c *Client) ComponentStatuses(namespace string) ComponentStatusInterface {
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &ComponentStatusResource, ComponentStatusGroupVersionKind, componentStatusFactory{})
	return &componentStatusClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type NamespacesGetter interface {
	Namespaces(namespace string) NamespaceInterface
}

func (c *Client) Namespaces(namespace string) NamespaceInterface {
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &NamespaceResource, NamespaceGroupVersionKind, namespaceFactory{})
	return &namespaceClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type EventsGetter interface {
	Events(namespace string) EventInterface
}

func (c *Client) Events(namespace string) EventInterface {
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &EventResource, EventGroupVersionKind, eventFactory{})
	return &eventClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type PodsGetter interface {
	Pods(namespace string) PodInterface
}

func (c *Client) Pods(namespace string) PodInterface {
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &PodResource, PodGroupVersionKind, podFactory{})
	return &podClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ServicesGetter interface {
	Services(namespace string) ServiceInterface
}

func (c *Client) Services(namespace string) ServiceInterface {
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &ServiceResource, ServiceGroupVersionKind, serviceFactory{})
	return &serviceClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type SecretsGetter interface {
	Secrets(namespace string) SecretInterface
}

func (c *Client) Secrets(namespace string) SecretInterface {
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &SecretResource, SecretGroupVersionKind, secretFactory{})
	return &secretClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
