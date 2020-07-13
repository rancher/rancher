package v1beta1

import (
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/norman/objectclient"
)

type Interface interface {
	PodSecurityPoliciesGetter
}

type Client struct {
	controllerFactory controller.SharedControllerFactory
	clientFactory     client.SharedClientFactory
}

func NewFromControllerFactory(factory controller.SharedControllerFactory) (Interface, error) {
	return &Client{
		controllerFactory: factory,
		clientFactory:     factory.SharedCacheFactory().SharedClientFactory(),
	}, nil
}

type PodSecurityPoliciesGetter interface {
	PodSecurityPolicies(namespace string) PodSecurityPolicyInterface
}

func (c *Client) PodSecurityPolicies(namespace string) PodSecurityPolicyInterface {
	sharedClient := c.clientFactory.ForResourceKind(PodSecurityPolicyGroupVersionResource, PodSecurityPolicyGroupVersionKind.Kind, false)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &PodSecurityPolicyResource, PodSecurityPolicyGroupVersionKind, podSecurityPolicyFactory{})
	return &podSecurityPolicyClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
