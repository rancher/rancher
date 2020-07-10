package systemimage

import (
	"context"

	"github.com/rancher/rancher/pkg/types/config"
)

func Register(ctx context.Context, cluster *config.UserContext) {
	projClient := cluster.Management.Management.Projects(cluster.ClusterName)
	catalogClient := cluster.Management.Management.Catalogs("")
	systemServices := getSystemService()
	for _, v := range systemServices {
		v.Init(cluster)
	}

	syncer := Syncer{
		clusterName:    cluster.ClusterName,
		projects:       projClient,
		projectLister:  projClient.Controller().Lister(),
		systemServices: systemServices,
	}
	projClient.AddClusterScopedHandler(ctx, "system-image-upgrade-controller", cluster.ClusterName, syncer.SyncProject)
	catalogClient.AddHandler(ctx, "system-image-upgrade-catalog-controller", syncer.SyncCatalog)
}
