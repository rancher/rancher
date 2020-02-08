package client

import (
	"fmt"
	"net/http"
	"time"

	"github.com/rancher/steve/pkg/attributes"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/rest"
)

type Factory struct {
	impersonate    bool
	clientCfg      *rest.Config
	watchClientCfg *rest.Config
	metadata       metadata.Interface
	Config         *rest.Config
}

type addQuery struct {
	values map[string]string
	next   http.RoundTripper
}

func (a *addQuery) RoundTrip(req *http.Request) (*http.Response, error) {
	q := req.URL.Query()
	for k, v := range a.values {
		q.Set(k, v)
	}
	req.Header.Set("Accept", "application/json;as=Table;v=v1;g=meta.k8s.io")
	req.URL.RawQuery = q.Encode()
	return a.next.RoundTrip(req)
}

func NewFactory(cfg *rest.Config, impersonate bool) (*Factory, error) {
	clientCfg := rest.CopyConfig(cfg)
	clientCfg.QPS = 10000
	clientCfg.Burst = 100
	clientCfg.AcceptContentTypes = "application/json;as=Table;v=v1;g=meta.k8s.io"
	clientCfg.Wrap(func(rt http.RoundTripper) http.RoundTripper {
		return &addQuery{
			values: map[string]string{
				"includeObject": "Object",
			},
			next: rt,
		}
	})

	watchClientCfg := rest.CopyConfig(cfg)
	watchClientCfg.Timeout = 30 * time.Minute

	md, err := metadata.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return &Factory{
		metadata:       md,
		impersonate:    impersonate,
		clientCfg:      clientCfg,
		watchClientCfg: watchClientCfg,
		Config:         watchClientCfg,
	}, nil
}

func (p *Factory) MetadataClient() metadata.Interface {
	return p.metadata
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
