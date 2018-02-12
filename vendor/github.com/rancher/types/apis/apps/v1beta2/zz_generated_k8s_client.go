package v1beta2

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

	DeploymentsGetter
	DaemonSetsGetter
	StatefulSetsGetter
}

type Client struct {
	sync.Mutex
	restClient rest.Interface
	starters   []controller.Starter

	deploymentControllers  map[string]DeploymentController
	daemonSetControllers   map[string]DaemonSetController
	statefulSetControllers map[string]StatefulSetController
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

		deploymentControllers:  map[string]DeploymentController{},
		daemonSetControllers:   map[string]DaemonSetController{},
		statefulSetControllers: map[string]StatefulSetController{},
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
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &DeploymentResource, DeploymentGroupVersionKind, deploymentFactory{})
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
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &DaemonSetResource, DaemonSetGroupVersionKind, daemonSetFactory{})
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
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &StatefulSetResource, StatefulSetGroupVersionKind, statefulSetFactory{})
	return &statefulSetClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
