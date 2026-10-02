// Package wrangler provides compatibility aliases for the contexts owned by config.
package wrangler

import (
	"context"

	"github.com/rancher/rancher/pkg/types/config"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Context = config.Context
type MultiClusterManager = config.MultiClusterManager
type SimpleRESTClientGetter = config.SimpleRESTClientGetter

// Scheme shares config's scheme pointer; registrations through either package
// are visible to both.
var (
	Scheme      = config.Scheme
	AddToScheme = config.AddToScheme
)

func NewPrimaryContext(ctx context.Context, clientConfig clientcmd.ClientConfig, restConfig *rest.Config) (*Context, error) {
	return config.NewPrimaryContext(ctx, clientConfig, restConfig)
}

func NewContext(ctx context.Context, clientConfig clientcmd.ClientConfig, restConfig *rest.Config) (*Context, error) {
	return config.NewContext(ctx, clientConfig, restConfig)
}
