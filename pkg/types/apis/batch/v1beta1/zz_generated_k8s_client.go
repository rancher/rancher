package v1beta1

import (
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/norman/objectclient"
)

type Interface interface {
	CronJobsGetter
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

type CronJobsGetter interface {
	CronJobs(namespace string) CronJobInterface
}

func (c *Client) CronJobs(namespace string) CronJobInterface {
	sharedClient := c.clientFactory.ForResourceKind(CronJobGroupVersionResource, CronJobGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &CronJobResource, CronJobGroupVersionKind, cronJobFactory{})
	return &cronJobClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
