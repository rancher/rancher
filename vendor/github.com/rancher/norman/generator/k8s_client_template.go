package generator

var k8sClientTemplate = `package {{.version.Version}}

import (
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/norman/objectclient"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

type Interface interface {
	{{range .schemas}}
	{{.CodeNamePlural}}Getter{{end}}
}

type Client struct {
	controllerFactory controller.SharedControllerFactory
	clientFactory     client.SharedClientFactory
}

{{ if not .external }}
func NewForConfig(cfg rest.Config) (Interface, error) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		return nil, err
	}
	controllerFactory, err := controller.NewSharedControllerFactoryFromConfig(&cfg, scheme)
	if err != nil {
		return nil, err
	}
	return NewFromControllerFactory(controllerFactory)
}
{{ end }}

func NewFromControllerFactory(factory controller.SharedControllerFactory) (Interface, error) {
	return &Client{
		controllerFactory: factory,
		clientFactory:     factory.SharedCacheFactory().SharedClientFactory(),
	}, nil
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
