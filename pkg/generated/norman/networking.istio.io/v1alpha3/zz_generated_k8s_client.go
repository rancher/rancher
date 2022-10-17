package v1alpha3

import (
	"github.com/knative/pkg/apis/istio/v1alpha3"
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/norman/generator"
	"github.com/rancher/norman/objectclient"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

type Interface interface {
	VirtualServicesGetter
	DestinationRulesGetter
}

type Client struct {
	controllerFactory controller.SharedControllerFactory
	clientFactory     client.SharedClientFactory
}

func NewForConfig(cfg rest.Config) (Interface, error) {
	scheme := runtime.NewScheme()
	if err := v1alpha3.AddToScheme(scheme); err != nil {
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
		controllerFactory: factory,
		clientFactory:     client.NewSharedClientFactoryWithAgent(userAgent, factory.SharedCacheFactory().SharedClientFactory()),
	}
}

type VirtualServicesGetter interface {
	VirtualServices(namespace string) VirtualServiceInterface
}

func (c *Client) VirtualServices(namespace string) VirtualServiceInterface {
	sharedClient := c.clientFactory.ForResourceKind(VirtualServiceGroupVersionResource, VirtualServiceGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &VirtualServiceResource, VirtualServiceGroupVersionKind, virtualServiceFactory{})
	return &virtualServiceClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type DestinationRulesGetter interface {
	DestinationRules(namespace string) DestinationRuleInterface
}

func (c *Client) DestinationRules(namespace string) DestinationRuleInterface {
	sharedClient := c.clientFactory.ForResourceKind(DestinationRuleGroupVersionResource, DestinationRuleGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &DestinationRuleResource, DestinationRuleGroupVersionKind, destinationRuleFactory{})
	return &destinationRuleClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
