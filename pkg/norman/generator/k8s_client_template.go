package generator

var k8sClientTemplate = `package {{.version.Version}}

import (
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/generator"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	{{.importPackage}}
)

type Interface interface {
	{{range .schemas}}
	{{.CodeNamePlural}}Getter{{end}}
}

type Client struct {
	controllerFactory controller.SharedControllerFactory
	clientFactory     client.SharedClientFactory
}

func NewForConfig(cfg rest.Config) (Interface, error) {
	scheme := runtime.NewScheme()
	if err := {{.prefix}}AddToScheme(scheme); err != nil {
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

{{range .schemas}}
type {{.CodeNamePlural}}Getter interface {
	{{.CodeNamePlural}}(namespace string) {{.CodeName}}Interface
}

func (c *Client) {{.CodeNamePlural}}(namespace string) {{.CodeName}}Interface {
	sharedClient := c.clientFactory.ForResourceKind({{.CodeName}}GroupVersionResource, {{.CodeName}}GroupVersionKind.Kind, {{ . | namespaced }})
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &{{.CodeName}}Resource, {{.CodeName}}GroupVersionKind, {{.ID}}Factory{})
	return &{{.ID}}Client{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
{{end}}
`
