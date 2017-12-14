package agent

import (
	"context"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
)

func Register(ctx context.Context, management *config.ManagementContext) {
	lifecycle := &ClusterLifecycle{
		Manager: NewManager(management),
		ctx:     ctx,
	}

	clusterClient := management.Management.Clusters("")
	handler := v3.NewClusterLifecycleAdapter("cluster-agent-controller", clusterClient, lifecycle)
	clusterClient.Controller().AddHandler(handler)
}

type ClusterLifecycle struct {
	Manager *Manager
	ctx     context.Context
}

func (c *ClusterLifecycle) Create(obj *v3.Cluster) (*v3.Cluster, error) {
	return nil, nil
}

func (c *ClusterLifecycle) Remove(obj *v3.Cluster) (*v3.Cluster, error) {
	c.Manager.Stop(c.ctx, obj)
	return nil, nil
}

func (c *ClusterLifecycle) Updated(obj *v3.Cluster) (*v3.Cluster, error) {
	return nil, c.Manager.Start(c.ctx, obj)
}
