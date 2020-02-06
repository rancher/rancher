package v2beta2

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

	HorizontalPodAutoscalersGetter
}

type Client struct {
	sync.Mutex
	restClient rest.Interface
	starters   []controller.Starter

	horizontalPodAutoscalerControllers map[string]HorizontalPodAutoscalerController
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

		horizontalPodAutoscalerControllers: map[string]HorizontalPodAutoscalerController{},
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

type HorizontalPodAutoscalersGetter interface {
	HorizontalPodAutoscalers(namespace string) HorizontalPodAutoscalerInterface
}

func (c *Client) HorizontalPodAutoscalers(namespace string) HorizontalPodAutoscalerInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &HorizontalPodAutoscalerResource, HorizontalPodAutoscalerGroupVersionKind, horizontalPodAutoscalerFactory{})
	return &horizontalPodAutoscalerClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
