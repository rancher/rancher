package k8s

import (
	"context"
)

var serverConfig configKey

type configKey struct{}

func SetK3sConfig(ctx context.Context, conf interface{}) context.Context {
	return context.WithValue(ctx, serverConfig, conf)
}
