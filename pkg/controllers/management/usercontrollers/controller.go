package usercontrollers

import (
	"context"

	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
)

/*
RegisterLate registers ClusterLifecycle controller which is responsible for starting rancher agent in user cluster,
and registering k8s controllers, on cluster.update
*/
func RegisterLate(ctx context.Context, management *config.ManagementContext, manager *clustermanager.Manager) {
	lifecycle := &ClusterLifecycle{
		Manager: manager,
		ctx:     ctx,
	}

	clusterClient := management.Management.Clusters("")
	clusterClient.AddLifecycle("cluster-agent-controller", lifecycle)
}

/*
RegisterEarly registers ClusterLifecycleCleanup controller which is responsible for stopping rancher agent in user cluster,
and de-registering k8s controllers, on cluster.remove
*/
func RegisterEarly(ctx context.Context, management *config.ManagementContext, manager *clustermanager.Manager) {
	lifecycle := &ClusterLifecycleCleanup{
		Manager: manager,
		ctx:     ctx,
	}

	clusterClient := management.Management.Clusters("")
	clusterClient.AddLifecycle("cluster-agent-controller-cleanup", lifecycle)
}

type ClusterLifecycle struct {
	Manager *clustermanager.Manager
	ctx     context.Context
}

type ClusterLifecycleCleanup struct {
	Manager *clustermanager.Manager
	ctx     context.Context
}

func (c *ClusterLifecycle) Create(obj *v3.Cluster) (*v3.Cluster, error) {
	return nil, nil
}

func (c *ClusterLifecycle) Remove(obj *v3.Cluster) (*v3.Cluster, error) {
	return nil, nil
}

func (c *ClusterLifecycle) Updated(obj *v3.Cluster) (*v3.Cluster, error) {
	return nil, c.Manager.Start(c.ctx, obj, false)
}

func (c *ClusterLifecycleCleanup) Create(obj *v3.Cluster) (*v3.Cluster, error) {
	return nil, nil
}

func (c *ClusterLifecycleCleanup) Remove(obj *v3.Cluster) (*v3.Cluster, error) {
	c.Manager.Stop(obj)
	return nil, nil
}

func (c *ClusterLifecycleCleanup) Updated(obj *v3.Cluster) (*v3.Cluster, error) {
	return nil, nil
}
