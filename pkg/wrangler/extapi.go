package wrangler

import "github.com/rancher/rancher/pkg/types/config"

type EXTAPIContext = config.EXTAPIContext
type DeferredEXTAPIInitializer = config.DeferredEXTAPIInitializer

func NewEXTAPIInitializer(clients *Context) *DeferredEXTAPIInitializer {
	return config.NewEXTAPIInitializer(clients)
}
