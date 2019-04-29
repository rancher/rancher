package catalog

import (
	"context"
	"fmt"
	"github.com/rancher/rancher/pkg/catalog/git"
	"github.com/rancher/types/client/management/v3"
	"k8s.io/client-go/tools/cache"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bep/debounce"
	helmlib "github.com/rancher/rancher/pkg/catalog/helm"
	"github.com/rancher/rancher/pkg/catalog/manager"
	"github.com/rancher/rancher/pkg/ticker"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
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

	// Don't need to watch catalog sync events on leader node of duplicate actions
	if context.PeerManager != nil && !context.PeerManager.IsLeader() {
		logrus.Debug("registering scaled context catalog handler")
		context.Management.Catalogs("").Controller().AddHandler(ctx, "scaledCatalogSync", cleaner.scaledCatalogSync)
		context.Management.ClusterCatalogs("").Controller().AddHandler(ctx, "scaledClusterCatalogSync", cleaner.scaledClusterCatalogSync)
		context.Management.ProjectCatalogs("").Controller().AddHandler(ctx, "scaledProjectCatalogSync", cleaner.scaledProjectCatalogSync)
	}
}

func (c *CacheCleaner) scaledCatalogSync(key string, obj *v3.Catalog) (runtime.Object, error) {
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, err
	}

	if obj == nil {
		return nil, nil
	}

	// always get a refresh catalog from etcd
	catalog, err := c.catalogClient.GetNamespaced(ns, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	cmt := &manager.CatalogInfo{
		Catalog: obj,
	}
	return c.checkCatalogUpdate(catalog, cmt)
}

func (c *CacheCleaner) scaledProjectCatalogSync(key string, obj *v3.ProjectCatalog) (runtime.Object, error) {
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, err
	}

	if obj == nil {
		return nil, nil
	}

	// always get a refresh project catalog from etcd
	projectCatalog, err := c.projectCatalogClient.GetNamespaced(ns, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	cmt := &manager.CatalogInfo{
		ProjectCatalog: projectCatalog,
	}
	return c.checkCatalogUpdate(&projectCatalog.Catalog, cmt)
}

func (c *CacheCleaner) scaledClusterCatalogSync(key string, obj *v3.ClusterCatalog) (runtime.Object, error) {
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, err
	}

	if obj == nil {
		return nil, nil
	}

	// always get a refresh cluster catalog from etcd
	clusterCatalog, err := c.clusterCatalogClient.GetNamespaced(ns, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	cmt := &manager.CatalogInfo{
		ClusterCatalog: clusterCatalog,
	}
	return c.checkCatalogUpdate(&clusterCatalog.Catalog, cmt)
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

func (c CacheCleaner) checkCatalogUpdate(catalog *v3.Catalog, cmt *manager.CatalogInfo) (runtime.Object, error) {
	hashPath := helmlib.CatalogSHA256Hash(catalog)
	path := filepath.Join(helmlib.CatalogCache, hashPath)
	if _, err := os.Stat(path); err == nil {
		if git.IsGitPath(path) {
			// if git path exist, check if the local cache commit is same with the updated commit
			headCommit, err := git.HeadCommit(path)
			if err != nil {
				return nil, err
			}
			if !manager.IsUpToDate(headCommit, catalog) {
				return c.newForceUpdateCatalog(catalog, cmt)
			}
		} else {
			// if it's not a git path repo, update as helm index catalog
			return c.newForceUpdateCatalog(catalog, cmt)
		}
	} else if os.IsNotExist(err) {
		return c.newForceUpdateCatalog(catalog, cmt)
	} else {
		return nil, err
	}
	return nil, nil
}

func (c CacheCleaner) newForceUpdateCatalog(obj *v3.Catalog, cmt *manager.CatalogInfo) (runtime.Object, error) {
	catalogType := manager.GetCatalogType(cmt)
	commit, helm, err := helmlib.NewForceUpdate(obj)
	logrus.Debugf("Helm update local path: %s to commit %s", helm.LocalPath, commit)
	if err != nil {
		switch catalogType {
		case client.CatalogType:
			manager.SetRefreshedError(obj, err)
			return c.catalogClient.Update(cmt.Catalog)
		case client.ClusterCatalogType:
			manager.SetRefreshedError(obj, err)
			return c.clusterCatalogClient.Update(cmt.ClusterCatalog)
		case client.ProjectCatalogType:
			manager.SetRefreshedError(obj, err)
			return c.projectCatalogClient.Update(cmt.ProjectCatalog)
		default:
			return nil, fmt.Errorf("incorrect catalog type")
		}
	}
	return nil, nil
}
