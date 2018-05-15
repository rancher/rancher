package logging

import (
	"context"

	"github.com/rancher/types/config"
)

func Register(ctx context.Context, cluster *config.UserContext) {
	registerClusterLogging(ctx, cluster)
	registerProjectLogging(cluster)
}
