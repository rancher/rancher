package loader

import (
	"context"
	"plugin"

	"github.com/rancher/rancher/pkg/multiclustermanager/deferred"
	"github.com/rancher/rancher/pkg/multiclustermanager/options"
	"github.com/rancher/rancher/pkg/wrangler"
)

func NewMCM(ctx context.Context, wranglerContext *wrangler.Context, cfg *options.Options) (wrangler.MultiClusterManager, error) {
	p, err := plugin.Open("/usr/lib/rancher/mcm.so")
	if err != nil {
		p2, err2 := plugin.Open("./mcm.so")
		if err2 != nil {
			return nil, err
		}
		p = p2
	}

	f, err := p.Lookup("Factory")
	if err != nil {
		return nil, err
	}

	newMCM := f.(*deferred.Factory)
	return (*newMCM)(ctx, wranglerContext, cfg)
}
