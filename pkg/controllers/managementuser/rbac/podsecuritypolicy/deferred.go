package podsecuritypolicy

import (
	"context"

	"github.com/rancher/rancher/pkg/types/config"
)

func registerDeferred(ctx context.Context, context *config.UserContext) {
	RegisterNamespace(ctx, context)
}
