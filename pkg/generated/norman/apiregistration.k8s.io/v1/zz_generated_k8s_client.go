package v1

import (
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/norman/generator"
	"github.com/rancher/norman/objectclient"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
)

type Interface interface {
	APIServicesGetter
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

type APIServicesGetter interface {
	APIServices(namespace string) APIServiceInterface
}

func (c *Client) APIServices(namespace string) APIServiceInterface {
	sharedClient := c.clientFactory.ForResourceKind(APIServiceGroupVersionResource, APIServiceGroupVersionKind.Kind, false)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [APIServices] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &APIServiceResource, APIServiceGroupVersionKind, apiServiceFactory{})
	return &apiServiceClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
