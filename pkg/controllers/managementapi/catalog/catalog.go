package catalog

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bep/debounce"
	helmlib "github.com/rancher/rancher/pkg/catalog/helm"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/ticker"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type CacheCleaner struct {
	catalogClient        v3.CatalogInterface
	projectCatalogClient v3.ProjectCatalogInterface
	clusterCatalogClient v3.ClusterCatalogInterface
	debounce             func(func())
}

func Register(ctx context.Context, context *config.ScaledContext) {
	cleaner := &CacheCleaner{
		catalogClient:        context.Management.Catalogs(""),
		projectCatalogClient: context.Management.ProjectCatalogs(""),
		clusterCatalogClient: context.Management.ClusterCatalogs(""),
		debounce:             debounce.New(time.Minute),
	}
	go cleaner.runPeriodicCatalogCacheCleaner(ctx, time.Hour)

	context.Management.Catalogs("").Controller().AddHandler(ctx, "catalogCache", cleaner.destroyCatalogSync)
	context.Management.ClusterCatalogs("").Controller().AddHandler(ctx, "clusterCatalogCache", cleaner.destroyClusterCatalogSync)
	context.Management.ProjectCatalogs("").Controller().AddHandler(ctx, "projectCatalogCache", cleaner.destroyProjectCatalogSync)
}

func (c *CacheCleaner) runPeriodicCatalogCacheCleaner(ctx context.Context, interval time.Duration) {
	c.GoCleanupLogError()
	for range ticker.Context(ctx, interval) {
		c.GoCleanupLogError()
	}
}

func (c *CacheCleaner) destroyCatalogSync(key string, obj *v3.Catalog) (runtime.Object, error) {
	c.debounce(c.GoCleanupLogError)
	return nil, nil
}

func (c *CacheCleaner) destroyClusterCatalogSync(key string, obj *v3.ClusterCatalog) (runtime.Object, error) {
	c.debounce(c.GoCleanupLogError)
	return nil, nil
}

func (c *CacheCleaner) destroyProjectCatalogSync(key string, obj *v3.ProjectCatalog) (runtime.Object, error) {
	c.debounce(c.GoCleanupLogError)
	return nil, nil
}

func (c *CacheCleaner) GoCleanupLogError() {
	go func() {
		if err := c.Cleanup(); err != nil {
			logrus.Errorf("Catalog-cache cleanup error: %s", err)
		}
	}()
}

func (c *CacheCleaner) Cleanup() error {
	logrus.Debug("Catalog-cache running cleanup")

	catalogCacheFiles, err := readDirNames(helmlib.CatalogCache)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	iconCacheFiles, err := readDirNames(helmlib.IconCache)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(catalogCacheFiles) == 0 && len(iconCacheFiles) == 0 {
		return nil
	}

	var catalogHashes = map[string]bool{}

	catalogs, err := c.catalogClient.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, catalog := range catalogs.Items {
		catalogHashes[helmlib.CatalogSHA256Hash(&catalog)] = true
	}
	clusterCatalogs, err := c.clusterCatalogClient.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, clusterCatalog := range clusterCatalogs.Items {
		catalogHashes[helmlib.CatalogSHA256Hash(&clusterCatalog.Catalog)] = true
	}
	projectCatalogs, err := c.projectCatalogClient.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, projectCatalog := range projectCatalogs.Items {
		catalogHashes[helmlib.CatalogSHA256Hash(&projectCatalog.Catalog)] = true
	}

	var cleanupCount int
	cleanupCount += cleanupPath(helmlib.CatalogCache, catalogCacheFiles, catalogHashes)
	cleanupCount += cleanupPath(helmlib.IconCache, iconCacheFiles, catalogHashes)
	if cleanupCount > 0 {
		logrus.Infof("Catalog-cache removed %d entries from disk", cleanupCount)
	}
	return nil
}

func readDirNames(dir string) ([]string, error) {
	pathFile, err := os.Open(dir)
	defer pathFile.Close()
	if err != nil {
		return nil, err
	}
	return pathFile.Readdirnames(0)
}

func cleanupPath(dir string, files []string, valid map[string]bool) int {
	var count int
	for _, file := range files {
		if valid[file] || strings.HasPrefix(file, ".") {
			continue
		}

		dirFile := filepath.Join(dir, file)
		helmlib.Locker.Lock(file)
		os.RemoveAll(dirFile)
		helmlib.Locker.Unlock(file)

		count++
		logrus.Debugf("Catalog-cache removed entry from disk: %s", dirFile)
	}
	return count
}
