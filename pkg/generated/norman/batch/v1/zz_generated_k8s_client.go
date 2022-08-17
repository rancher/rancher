package v1

import (
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/norman/generator"
	"github.com/rancher/norman/objectclient"
	"github.com/sirupsen/logrus"
	"k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

type Interface interface {
	JobsGetter
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

type JobsGetter interface {
	Jobs(namespace string) JobInterface
}

func (c *Client) Jobs(namespace string) JobInterface {
	sharedClient := c.clientFactory.ForResourceKind(JobGroupVersionResource, JobGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [Jobs] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &JobResource, JobGroupVersionKind, jobFactory{})
	return &jobClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
