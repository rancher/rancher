package pipeline

import (
	"context"

	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/pipeline/controller/pipeline"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/pipeline/controller/pipelineexecution"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/pipeline/controller/project"
	"github.com/rancher/rancher/pkg/types/config"
)

func Register(ctx context.Context, cluster *config.UserContext) {
	pipeline.Register(ctx, cluster)
	pipelineexecution.Register(ctx, cluster)
	project.Register(ctx, cluster)
}
