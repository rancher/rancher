package catalog

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/rancher/rancher/pkg/catalog/manager"
	"github.com/rancher/rancher/pkg/ticker"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/labels"
)

func Register(ctx context.Context, management *config.ManagementContext) {
	// TODO: Get values from settings
	if err := Run(ctx, "", 3600, management); err != nil {
		panic(err)
	}
}

func runRefreshCatalog(ctx context.Context, interval int, controller v3.CatalogController, m *manager.Manager) {
	for range ticker.Context(ctx, time.Duration(interval)*time.Second) {
		catalogs, err := m.CatalogLister.List("", labels.NewSelector())
		if err != nil {
			logrus.Error(err)
			continue
		}
		for _, catalog := range catalogs {
			controller.Enqueue("", catalog.Name)
		}
	}
}

func runRefreshProjectCatalog(ctx context.Context, interval int, controller v3.ProjectCatalogController, m *manager.Manager) {
	for range ticker.Context(ctx, time.Duration(interval)*time.Second) {
		projectCatalogs, err := m.ProjectCatalogLister.List("", labels.NewSelector())
		if err != nil {
			logrus.Error(err)
			continue
		}
		for _, pc := range projectCatalogs {
			controller.Enqueue("", pc.Name)
		}
	}
}

func runRefreshClusterCatalog(ctx context.Context, interval int, controller v3.ClusterCatalogController, m *manager.Manager) {
	for range ticker.Context(ctx, time.Duration(interval)*time.Second) {
		clusterCatalogs, err := m.ClusterCatalogLister.List("", labels.NewSelector())
		if err != nil {
			logrus.Error(err)
			continue
		}
		for _, cc := range clusterCatalogs {
			controller.Enqueue("", cc.Name)
		}
	}
}

func Run(ctx context.Context, cacheRoot string, refreshInterval int, management *config.ManagementContext) error {
	if cacheRoot == "" {
		cacheRoot = path.Join("./management-state", "catalog-controller", "cache")
	}
	if err := os.MkdirAll(filepath.Join(cacheRoot, "templateContent"), 0777); err != nil {
		return err
	}

	logrus.Infof("Starting catalog controller")
	m := manager.New(management, cacheRoot)

	controller := management.Management.Catalogs("").Controller()
	controller.AddHandler(ctx, "catalog", m.Sync)

	logrus.Infof("Starting project-level catalog controller")
	projectCatalogController := management.Management.ProjectCatalogs("").Controller()
	projectCatalogController.AddHandler(ctx, "projectCatalog", m.ProjectCatalogSync)

	logrus.Infof("Starting cluster-level catalog controller")
	clusterCatalogController := management.Management.ClusterCatalogs("").Controller()
	clusterCatalogController.AddHandler(ctx, "clusterCatalog", m.ClusterCatalogSync)

	go runRefreshCatalog(ctx, refreshInterval, controller, m)
	go runRefreshProjectCatalog(ctx, refreshInterval, projectCatalogController, m)
	go runRefreshClusterCatalog(ctx, refreshInterval, clusterCatalogController, m)

	return nil
}
