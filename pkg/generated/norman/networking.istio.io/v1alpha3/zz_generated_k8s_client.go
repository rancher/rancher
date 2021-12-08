package v1alpha3

import (
	"github.com/knative/pkg/apis/istio/v1alpha3"
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
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
	controllerFactory, err := controller.NewSharedControllerFactoryFromConfig(&cfg, scheme)
	if err != nil {
		return nil, err
	}
	return NewFromControllerFactory(controllerFactory)
}

func NewFromControllerFactory(factory controller.SharedControllerFactory) (Interface, error) {
	return &Client{
		controllerFactory: factory,
		clientFactory:     factory.SharedCacheFactory().SharedClientFactory(),
	}, nil
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
