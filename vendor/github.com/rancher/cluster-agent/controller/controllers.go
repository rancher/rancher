package controller

import (
	"context"

	"github.com/rancher/cluster-agent/controller/authz"
	"github.com/rancher/cluster-agent/controller/eventssyncer"
	"github.com/rancher/cluster-agent/controller/healthsyncer"
	"github.com/rancher/cluster-agent/controller/nodesyncer"
	"github.com/rancher/cluster-agent/controller/statsyncer"
	"github.com/rancher/types/config"
	workloadController "github.com/rancher/workload-controller/controller"
)

func Register(ctx context.Context, cluster *config.ClusterContext) {
	nodesyncer.Register(cluster)
	healthsyncer.Register(ctx, cluster)
	authz.Register(cluster)
	statsyncer.Register(cluster)
	eventssyncer.Register(cluster)

	workloadContext := cluster.WorkloadContext()
	workloadController.Register(ctx, workloadContext)
}
