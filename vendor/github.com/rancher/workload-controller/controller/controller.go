package controller

import (
	"context"

	"github.com/rancher/types/config"
	"github.com/rancher/workload-controller/controller/workload"
)

func Register(ctx context.Context, workloadContext *config.WorkloadContext) {
	workload.Register(ctx, workloadContext)
}
