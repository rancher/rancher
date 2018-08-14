package catalog

import (
	"context"
	"path"
	"time"

	"github.com/rancher/rancher/pkg/catalog/manager"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
)

func Register(ctx context.Context, management *config.ManagementContext) {
	// TODO: Get values from settings
	if err := Run(ctx, "", 3600, management); err != nil {
		panic(err)
	}
}

func runRefresh(ctx context.Context, interval int, controller v3.CatalogController, m *manager.Manager) {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			break
		case <-ticker.C:
			catalogs, err := m.GetCatalogs()
			if err != nil {
				logrus.Error(err)
				continue
			}
			for _, catalog := range catalogs {
				controller.Enqueue("", catalog.Name)
			}
		}
	}
}

func runRefreshProject(ctx context.Context, interval int, controller v3.ProjectCatalogController, m *manager.Manager) {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			break
		case <-ticker.C:
			projCatalogs, err := m.GetProjectCatalogs()
			if err != nil {
				logrus.Error(err)
				continue
			}
			for _, pcatalog := range projCatalogs {
				controller.Enqueue(pcatalog.Namespace, pcatalog.Name)
			}
		}
	}
}

func runRefreshCluster(ctx context.Context, interval int, controller v3.ClusterCatalogController, m *manager.Manager) {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			break
		case <-ticker.C:
			clCatalogs, err := m.GetClusterCatalogs()
			if err != nil {
				logrus.Error(err)
				continue
			}
			for _, ccatalog := range clCatalogs {
				controller.Enqueue(ccatalog.Namespace, ccatalog.Name)
			}
		}
	}
}

func Run(ctx context.Context, cacheRoot string, refreshInterval int, management *config.ManagementContext) error {
	if cacheRoot == "" {
		cacheRoot = path.Join("./management-state", "catalog-controller", "cache")
	}

	logrus.Infof("Starting catalog controller")
	m := manager.New(management, cacheRoot)

	controller := management.Management.Catalogs("").Controller()
	controller.AddHandler("catalog", m.Sync)

	logrus.Infof("Starting project-level catalog controller")
	projectCatalogController := management.Management.ProjectCatalogs("").Controller()
	projectCatalogController.AddHandler("projectCatalog", m.PrjCatalogSync)

	logrus.Infof("Starting cluster-level catalog controller")
	clusterCatalogController := management.Management.ClusterCatalogs("").Controller()
	clusterCatalogController.AddHandler("clusterCatalog", m.ClusterCatalogSync)

	go runRefresh(ctx, refreshInterval, controller, m)
	go runRefreshProject(ctx, refreshInterval, projectCatalogController, m)
	go runRefreshCluster(ctx, refreshInterval, clusterCatalogController, m)

	return nil
}
