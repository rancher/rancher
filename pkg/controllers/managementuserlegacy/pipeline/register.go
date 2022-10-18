package pipeline

import (
	"context"

	v3 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/pipeline/controller/pipeline"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/pipeline/controller/pipelineexecution"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/pipeline/controller/project"
	"github.com/rancher/rancher/pkg/types/config"
	"k8s.io/apimachinery/pkg/runtime"
)

func Register(ctx context.Context, cluster *config.UserContext) {
	starter := cluster.DeferredStart(ctx, func(ctx context.Context) error {
		registerDeferred(ctx, cluster)
		return nil
	})
	AddStarter(ctx, cluster, starter)
}

func AddStarter(ctx context.Context, cluster *config.UserContext, starter func() error) {
	pipelines := cluster.Management.Project.Pipelines("")
	pipelines.AddClusterScopedHandler(ctx, "pipeline-deferred", cluster.ClusterName, func(key string, obj *v3.Pipeline) (runtime.Object, error) {
		return obj, starter()
	})
}

func registerDeferred(ctx context.Context, cluster *config.UserContext) {
	pipeline.Register(ctx, cluster)
	pipelineexecution.Register(ctx, cluster)
	project.Register(ctx, cluster)
}
