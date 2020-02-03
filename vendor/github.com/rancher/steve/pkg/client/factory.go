package client

import (
	"fmt"
	"time"

	"github.com/rancher/steve/pkg/attributes"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type Factory struct {
	impersonate    bool
	clientCfg      *rest.Config
	watchClientCfg *rest.Config
	client         dynamic.Interface
	Config         *rest.Config
}

func NewFactory(cfg *rest.Config, impersonate bool) (*Factory, error) {
	clientCfg := rest.CopyConfig(cfg)
	clientCfg.QPS = 10000
	clientCfg.Burst = 100

	watchClientCfg := rest.CopyConfig(cfg)
	watchClientCfg.Timeout = 30 * time.Minute

	dc, err := dynamic.NewForConfig(watchClientCfg)
	if err != nil {
		return nil, err
	}

	return &Factory{
		client:         dc,
		impersonate:    impersonate,
		clientCfg:      clientCfg,
		watchClientCfg: watchClientCfg,
		Config:         watchClientCfg,
	}, nil
}

func (p *Factory) DynamicClient() dynamic.Interface {
	return p.client
}

func (p *Factory) Client(ctx *types.APIRequest, s *types.APISchema, namespace string) (dynamic.ResourceInterface, error) {
	return p.newClient(ctx, p.clientCfg, s, namespace)
}

func (p *Factory) ClientForWatch(ctx *types.APIRequest, s *types.APISchema, namespace string) (dynamic.ResourceInterface, error) {
	return p.newClient(ctx, p.watchClientCfg, s, namespace)
}

func (p *Factory) newClient(ctx *types.APIRequest, cfg *rest.Config, s *types.APISchema, namespace string) (dynamic.ResourceInterface, error) {
	if p.impersonate {
		user, ok := request.UserFrom(ctx.Context())
		if !ok {
			return nil, fmt.Errorf("user not found for impersonation")
		}
		cfg = rest.CopyConfig(cfg)
		cfg.Impersonate.UserName = user.GetName()
		cfg.Impersonate.Groups = user.GetGroups()
		cfg.Impersonate.Extra = user.GetExtra()
	}

	client, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	gvr := attributes.GVR(s)
	return client.Resource(gvr).Namespace(namespace), nil
}
