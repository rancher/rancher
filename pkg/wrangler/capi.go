package wrangler

import "github.com/rancher/rancher/pkg/types/config"

type CAPIContext = config.CAPIContext
type DeferredCAPIInitializer = config.DeferredCAPIInitializer

func NewCAPIInitializer(clients *Context) *DeferredCAPIInitializer {
	return config.NewCAPIInitializer(clients)
}
