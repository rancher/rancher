package multiclustermanager

import (
	"github.com/rancher/rancher/pkg/multiclustermanager/deferred"
	"github.com/rancher/rancher/pkg/multiclustermanager/options"
	"github.com/rancher/rancher/pkg/wrangler"
)

func NewDeferredServer(wrangler *wrangler.Context, opts *options.Options) wrangler.MultiClusterManager {
	return deferred.NewDeferredServer(wrangler, opts)
}
