package systemimage

import (
	"context"

	"github.com/rancher/types/config"
)

func Register(ctx context.Context, cluster *config.UserContext) {
	sMap := make(map[string]SystemService)
	for k, v := range systemServices {
		sMap[k] = v.Init(ctx, cluster)
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
		systemServices:   sMap,
	}
	projClient.AddClusterScopedHandler(ctx, "system-image-upgrade-controller", cluster.ClusterName, syncer.Sync)

}
