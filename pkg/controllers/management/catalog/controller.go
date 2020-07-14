package catalog

import (
	"context"
	"os"
	"path"
	"time"

	"github.com/rancher/rancher/pkg/catalog/manager"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/ticker"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/labels"
)

func Register(ctx context.Context, management *config.ManagementContext) {
	// remove old disk cache
	oldCache := path.Join("management-state", "catalog-controller")
	os.RemoveAll(oldCache)

	// TODO: Get values from settings
	if err := Run(ctx, 3600, management); err != nil {
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
			controller.Enqueue(pc.Namespace, pc.Name)
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
			controller.Enqueue(cc.Namespace, cc.Name)
		}
	}
}

func doUntilSucceeds(ctx context.Context, retryPeriod time.Duration, f func() bool) {
	for {
		if f() {
			return
		}
		select {
		case <-time.After(retryPeriod):
		case <-ctx.Done():
			return
		}
	}
}

func Run(ctx context.Context, refreshInterval int, management *config.ManagementContext) error {
	logrus.Infof("Starting catalog controller")
	m := manager.New(management)

	controller := management.Management.Catalogs("").Controller()
	controller.AddHandler(ctx, "catalog", m.Sync)

	logrus.Infof("Starting project-level catalog controller")
	projectCatalogController := management.Management.ProjectCatalogs("").Controller()
	projectCatalogController.AddHandler(ctx, "projectCatalog", m.ProjectCatalogSync)

	logrus.Infof("Starting cluster-level catalog controller")
	clusterCatalogController := management.Management.ClusterCatalogs("").Controller()
	clusterCatalogController.AddHandler(ctx, "clusterCatalog", m.ClusterCatalogSync)

	var failureRetryPeriod = 15 * time.Minute
	go doUntilSucceeds(ctx, failureRetryPeriod, m.DeleteOldTemplateContent)
	go doUntilSucceeds(ctx, failureRetryPeriod, m.DeleteBadCatalogTemplates)

	go runRefreshCatalog(ctx, refreshInterval, controller, m)
	go runRefreshProjectCatalog(ctx, refreshInterval, projectCatalogController, m)
	go runRefreshClusterCatalog(ctx, refreshInterval, clusterCatalogController, m)

	return nil
}
