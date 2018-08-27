package systemimage

import (
	"context"

	"github.com/rancher/types/config"
)

func Register(ctx context.Context, cluster *config.UserContext) {
	for _, v := range systemServices {
		v.Init(ctx, cluster)
	}

	projClient := cluster.Management.Management.Projects("")
	syncer := Syncer{
		clusterName:      cluster.ClusterName,
		projects:         projClient,
		projectLister:    projClient.Controller().Lister(),
		daemonsets:       cluster.Apps.DaemonSets(""),
		daemonsetLister:  cluster.Apps.DaemonSets("").Controller().Lister(),
		deployments:      cluster.Apps.Deployments(""),
		deploymentLister: cluster.Apps.Deployments("").Controller().Lister(),
	}
	projClient.AddClusterScopedHandler("system-image-upgrade-controller", cluster.ClusterName, syncer.Sync)

}
