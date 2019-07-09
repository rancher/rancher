package systemimage

import (
	"context"

	"github.com/rancher/types/config"
)

func Register(ctx context.Context, cluster *config.UserContext) {
	projClient := cluster.Management.Management.Projects("")
	catalogClient := cluster.Management.Management.Catalogs("")
	systemServices := getSystemService()
	for _, v := range systemServices {
		v.Init(cluster)
	}

	syncer := Syncer{
		clusterName:      cluster.ClusterName,
		projects:         projClient,
		projectLister:    projClient.Controller().Lister(),
		daemonsets:       cluster.Apps.DaemonSets(""),
		daemonsetLister:  cluster.Apps.DaemonSets("").Controller().Lister(),
		deployments:      cluster.Apps.Deployments(""),
		deploymentLister: cluster.Apps.Deployments("").Controller().Lister(),
		systemSercices:   systemServices,
	}
	projClient.AddClusterScopedHandler(ctx, "system-image-upgrade-controller", cluster.ClusterName, syncer.SyncProject)
	catalogClient.AddHandler(ctx, "system-image-upgrade-catalog-controller", syncer.SyncCatalog)
}
