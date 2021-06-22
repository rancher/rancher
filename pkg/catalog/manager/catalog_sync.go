package manager

import (
	"github.com/rancher/rancher/pkg/catalog/utils"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	helmlib "github.com/rancher/rancher/pkg/helm"
	"github.com/rancher/rancher/pkg/image"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (m *Manager) updateCatalogError(catalog *v3.Catalog, err error) (runtime.Object, error) {
	setRefreshedError(catalog, err)
	m.catalogClient.Update(catalog)
	return nil, err
}

func (m *Manager) Sync(key string, obj *v3.Catalog) (runtime.Object, error) {
	if obj == nil {
		return nil, m.deleteTemplates(key, namespace.GlobalNamespace)
	}

	// always get a refresh catalog from etcd
	catalog, err := m.catalogClient.Get(key, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// When setting SystemCatalog is set to bundled, always force our catalogs to keep running that way
	if m.bundledMode {
		if (catalog.Name == "helm3-library" || catalog.Name == "library" || catalog.Name == "system-library") && catalog.Spec.CatalogKind != helmlib.KindHelmInternal {
			catalog.Spec.CatalogKind = helmlib.KindHelmInternal
			catalog, err = m.catalogClient.Update(catalog)
			if err != nil {
				return nil, err
			}
		}
	}

	commit, helm, err := helmlib.NewForceUpdate(catalog)
	if err != nil {
		return m.updateCatalogError(catalog, err)
	}
	logrus.Debugf("Chart hash comparison for global catalog %v: new -- %v --- current -- %v", catalog.Name, commit, catalog.Status.Commit)

	if isUpToDate(commit, catalog) {
		if setRefreshed(catalog) {
			m.catalogClient.Update(catalog)
		}
		return nil, nil
	}

	cmt := &CatalogInfo{
		catalog: catalog,
	}

	logrus.Infof("Updating global catalog %s", catalog.Name)
	err = m.traverseAndUpdate(helm, commit, cmt)
	if err != nil {
		return nil, err
	}

	// ensure the system catalog image cache exists
	var systemCatalogImageCache *v1.ConfigMap
	if catalog.Name == utils.SystemLibraryName {
		var systemCatalogImageCacheCreated bool
		systemCatalogImageCacheName := utils.GetCatalogImageCacheName(catalog.Name)
		systemCatalogImageCache, err = m.ConfigMapLister.Get(namespace.System, systemCatalogImageCacheName)

		// if the cache does not exist generate it
		if err != nil && errors.IsNotFound(err) {
			logrus.Debug("system catalog image cache configmap not found")

			systemCatalogImageCache = &v1.ConfigMap{}
			systemCatalogImageCache.Name = systemCatalogImageCacheName
			systemCatalogImageCache.Namespace = namespace.System
			err = image.CreateCatalogImageListConfigMap(systemCatalogImageCache, catalog)
			if err != nil {
				return nil, err
			}

			systemCatalogImageCache, err = m.ConfigMap.Create(systemCatalogImageCache)
			if err != nil && !errors.IsAlreadyExists(err) {
				return nil, err
			}
			systemCatalogImageCacheCreated = true
			logrus.Debug("system catalog image cache configmap created")

		} else {
			return nil, err
		}

		// if the cache exists and is out of date update it
		if !isUpToDate(commit, catalog) || systemCatalogImageCacheCreated {
			err = image.CreateCatalogImageListConfigMap(systemCatalogImageCache, catalog)
			if err != nil {
				return nil, err
			}

			// update
			_, err = m.ConfigMap.Update(systemCatalogImageCache)
			if err != nil {
				return nil, err
			}
			logrus.Debug("system catalog image cache updated")
		}
	}

	return nil, nil
}
