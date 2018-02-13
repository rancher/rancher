package pipeline

import (
	"context"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/controller/clusterpipeline"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/controller/pipeline"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/controller/pipelineexecution"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/utils"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
)

func Register(ctx context.Context, cluster *config.UserContext) {

	clusterPipelines := cluster.Management.Management.ClusterPipelines("")

	if err := utils.InitClusterPipeline(clusterPipelines, cluster.ClusterName); err != nil {
		logrus.Errorf("init cluster pipeline got error, %v", err)
	}

	clusterpipeline.Register(cluster)
	pipeline.Register(ctx, cluster)
	pipelineexecution.Register(ctx, cluster)
}
