package v1beta1

import (
	"context"
	"sync"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/objectclient/dynamic"
	"github.com/rancher/norman/restwatch"
	"k8s.io/client-go/rest"
)

type contextKeyType struct{}

type Interface interface {
	RESTClient() rest.Interface
	controller.Starter

	PodSecurityPoliciesGetter
	IngressesGetter
}

type Client struct {
	sync.Mutex
	restClient rest.Interface
	starters   []controller.Starter

	podSecurityPolicyControllers map[string]PodSecurityPolicyController
	ingressControllers           map[string]IngressController
}

func Factory(ctx context.Context, config rest.Config) (context.Context, controller.Starter, error) {
	c, err := NewForConfig(config)
	if err != nil {
		return ctx, nil, err
	}

	return context.WithValue(ctx, contextKeyType{}, c), c, nil
}

func From(ctx context.Context) Interface {
	return ctx.Value(contextKeyType{}).(Interface)
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

		podSecurityPolicyControllers: map[string]PodSecurityPolicyController{},
		ingressControllers:           map[string]IngressController{},
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

type PodSecurityPoliciesGetter interface {
	PodSecurityPolicies(namespace string) PodSecurityPolicyInterface
}

func (c *Client) PodSecurityPolicies(namespace string) PodSecurityPolicyInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &PodSecurityPolicyResource, PodSecurityPolicyGroupVersionKind, podSecurityPolicyFactory{})
	return &podSecurityPolicyClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type IngressesGetter interface {
	Ingresses(namespace string) IngressInterface
}

func (c *Client) Ingresses(namespace string) IngressInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &IngressResource, IngressGroupVersionKind, ingressFactory{})
	return &ingressClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
