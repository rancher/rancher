package pipeline

import (
	"context"

	"github.com/rancher/rancher/pkg/controllers/user/pipeline/controller/pipeline"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/controller/pipelineexecution"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/controller/project"
	"github.com/rancher/rancher/pkg/types/config"
)

func Register(ctx context.Context, cluster *config.UserContext) {
	pipeline.Register(ctx, cluster)
	pipelineexecution.Register(ctx, cluster)
	project.Register(ctx, cluster)
}
