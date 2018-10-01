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

type Interface interface {
	RESTClient() rest.Interface
	controller.Starter

	NetworkPoliciesGetter
}

type Client struct {
	sync.Mutex
	restClient rest.Interface
	starters   []controller.Starter

	networkPolicyControllers map[string]NetworkPolicyController
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

		networkPolicyControllers: map[string]NetworkPolicyController{},
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

type NetworkPoliciesGetter interface {
	NetworkPolicies(namespace string) NetworkPolicyInterface
}

func (c *Client) NetworkPolicies(namespace string) NetworkPolicyInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &NetworkPolicyResource, NetworkPolicyGroupVersionKind, networkPolicyFactory{})
	return &networkPolicyClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
