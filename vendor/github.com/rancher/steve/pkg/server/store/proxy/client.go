package proxy

import (
	"fmt"

	"github.com/rancher/steve/pkg/attributes"
	"github.com/rancher/steve/pkg/schemaserver/httperror"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type ClientFactory struct {
	cfg         rest.Config
	client      dynamic.Interface
	impersonate bool
	idToGVR     map[string]schema.GroupVersionResource
}

func NewClientFactory(cfg *rest.Config, impersonate bool) *ClientFactory {
	return &ClientFactory{
		impersonate: impersonate,
		cfg:         *cfg,
		idToGVR:     map[string]schema.GroupVersionResource{},
	}
}

func (p *ClientFactory) Client(ctx *types.APIRequest, schema *types.APISchema) (dynamic.ResourceInterface, error) {
	gvr := attributes.GVR(schema)
	if gvr.Resource == "" {
		return nil, httperror.NewAPIError(validation.NotFound, "Failed to find gvr for "+schema.ID)
	}

	user, ok := request.UserFrom(ctx.Request.Context())
	if !ok {
		return nil, fmt.Errorf("failed to find user context for client")
	}

	client, err := p.getClient(user)
	if err != nil {
		return nil, err
	}

	return client.Resource(gvr), nil
}

func (p *ClientFactory) getClient(user user.Info) (dynamic.Interface, error) {
	if p.impersonate {
		return p.client, nil
	}

	if user.GetName() == "" {
		return nil, fmt.Errorf("failed to determine current user")
	}

	newCfg := p.cfg
	newCfg.Impersonate.UserName = user.GetName()
	newCfg.Impersonate.Groups = user.GetGroups()
	newCfg.Impersonate.Extra = user.GetExtra()

	return dynamic.NewForConfig(&newCfg)
}
