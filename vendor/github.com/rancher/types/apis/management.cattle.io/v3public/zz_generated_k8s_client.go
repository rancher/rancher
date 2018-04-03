package v3public

import (
	"context"
	"sync"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/restwatch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type Interface interface {
	RESTClient() rest.Interface
	controller.Starter

	AuthProvidersGetter
}

type Client struct {
	sync.Mutex
	restClient rest.Interface
	starters   []controller.Starter

	authProviderControllers map[string]AuthProviderController
}

func NewForConfig(config rest.Config) (Interface, error) {
	if config.NegotiatedSerializer == nil {
		configConfig := dynamic.ContentConfig()
		config.NegotiatedSerializer = configConfig.NegotiatedSerializer
	}

	restClient, err := restwatch.UnversionedRESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	return &Client{
		restClient: restClient,

		authProviderControllers: map[string]AuthProviderController{},
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

type AuthProvidersGetter interface {
	AuthProviders(namespace string) AuthProviderInterface
}

func (c *Client) AuthProviders(namespace string) AuthProviderInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &AuthProviderResource, AuthProviderGroupVersionKind, authProviderFactory{})
	return &authProviderClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
