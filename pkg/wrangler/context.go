package wrangler

import (
	"context"

	"github.com/rancher/rancher/pkg/wrangler/generated/controllers/management.cattle.io"
	managementv3 "github.com/rancher/rancher/pkg/wrangler/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/steve/pkg/server"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/start"
	"k8s.io/client-go/rest"
)

type Context struct {
	*server.Controllers

	Apply    apply.Apply
	Mgmt     managementv3.Interface
	starters []start.Starter
}

func (w *Context) Start(ctx context.Context) error {
	if err := w.Controllers.Start(ctx); err != nil {
		return err
	}
	return start.All(ctx, 5, w.starters...)
}

func NewContext(restConfig *rest.Config) (*Context, error) {
	steveControllers, err := server.NewController(restConfig)
	if err != nil {
		return nil, err
	}

	apply, err := apply.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	mgmt, err := management.NewFactoryFromConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return &Context{
		Controllers: steveControllers,
		Apply:       apply,
		Mgmt:        mgmt.Management().V3(),
		starters: []start.Starter{
			mgmt,
		},
	}, nil
}
