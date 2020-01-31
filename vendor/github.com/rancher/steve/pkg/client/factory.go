package client

import (
	"github.com/rancher/steve/pkg/attributes"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type Factory struct {
	client dynamic.Interface
	Config *rest.Config
}

func NewFactory(cfg *rest.Config) (*Factory, error) {
	newCfg := rest.CopyConfig(cfg)
	newCfg.QPS = 10000
	newCfg.Burst = 100
	c, err := dynamic.NewForConfig(newCfg)
	if err != nil {
		return nil, err
	}
	return &Factory{
		client: c,
		Config: newCfg,
	}, nil
}

func (p *Factory) DynamicClient() dynamic.Interface {
	return p.client
}

func (p *Factory) Client(ctx *types.APIRequest, s *types.APISchema, namespace string) (dynamic.ResourceInterface, error) {
	gvr := attributes.GVR(s)
	return p.client.Resource(gvr).Namespace(namespace), nil
}
