package wrangler

import "github.com/rancher/rancher/pkg/types/config"

type DeferredInitializer[T any] = config.DeferredInitializer[T]
type DeferredRegistration[T any, I DeferredInitializer[T]] = config.DeferredRegistration[T, I]

func NewDeferredRegistration[T any, I DeferredInitializer[T]](clients *Context, init I, name string) *DeferredRegistration[T, I] {
	return config.NewDeferredRegistration(clients, init, name)
}
