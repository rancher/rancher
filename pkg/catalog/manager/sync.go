package manager

import (
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/client/management/v3"
	"github.com/sirupsen/logrus"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func (m *Manager) Sync(key string, obj *v3.Catalog) error {
	m.commonSync(key, obj)
	cmt := &CatalogManagerType{
		catalog:        obj,
		projectCatalog: nil,
		catalogType:    client.CatalogType,
	}

	// always get a refresh catalog from etcd
	catalog, err := m.catalogClient.Get(key, metav1.GetOptions{})
	if err != nil {
		return err
	}

	repoPath, commit, err := m.prepareRepoPath(*catalog)
	if err != nil {
		v3.CatalogConditionRefreshed.False(catalog)
		v3.CatalogConditionRefreshed.ReasonAndMessageFromError(catalog, err)
		m.catalogClient.Update(catalog)
		return err
	}

	if commit == catalog.Status.Commit {
		logrus.Debugf("Catalog %s is already up to date", catalog.Name)
		if v3.CatalogConditionRefreshed.IsUnknown(catalog) {
			v3.CatalogConditionRefreshed.True(catalog)
			v3.CatalogConditionRefreshed.Reason(catalog, "")
			m.catalogClient.Update(catalog)
		}
		return nil
	}

	logrus.Infof("Updating catalog %s", catalog.Name)
	return m.traverseAndUpdate(repoPath, commit, cmt)
}

func (m *Manager) PrjCatalogSync(key string, obj *v3.ProjectCatalog) error {
	m.commonSync(key, &obj.Catalog)
	cmt := &CatalogManagerType{
		catalog:        &obj.Catalog,
		projectCatalog: obj,
		catalogType:    client.ProjectCatalogType,
	}

	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	projectCatalog, err := m.projectCatalogClient.GetNamespaced(ns, name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	repoPath, commit, err := m.prepareRepoPath(obj.Catalog)
	if err != nil {
		v3.CatalogConditionRefreshed.False(projectCatalog)
		v3.CatalogConditionRefreshed.ReasonAndMessageFromError(projectCatalog, err)
		m.projectCatalogClient.Update(projectCatalog)
		return err
	}

	if commit == projectCatalog.Status.Commit {
		logrus.Debugf("Catalog %s is already up to date", projectCatalog.Name)
		if v3.CatalogConditionRefreshed.IsUnknown(projectCatalog) {
			v3.CatalogConditionRefreshed.True(projectCatalog)
			v3.CatalogConditionRefreshed.Reason(projectCatalog, "")
			m.projectCatalogClient.Update(projectCatalog)
		}
		return nil
	}

	logrus.Infof("Updating catalog %s", projectCatalog.Name)
	return m.traverseAndUpdate(repoPath, commit, cmt)
}

func (m *Manager) ClusterCatalogSync(key string, obj *v3.ClusterCatalog) error {
	m.commonSync(key, &obj.Catalog)
	cmt := &CatalogManagerType{
		catalog:        &obj.Catalog,
		clusterCatalog: obj,
		catalogType:    client.ClusterCatalogType,
	}

	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	clusterCatalog, err := m.clusterCatalogClient.GetNamespaced(ns, name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	repoPath, commit, err := m.prepareRepoPath(obj.Catalog)
	if err != nil {
		v3.CatalogConditionRefreshed.False(clusterCatalog)
		v3.CatalogConditionRefreshed.ReasonAndMessageFromError(clusterCatalog, err)
		m.clusterCatalogClient.Update(clusterCatalog)
		return err
	}

	if commit == clusterCatalog.Status.Commit {
		logrus.Debugf("Catalog %s is already up to date", clusterCatalog.Name)
		if v3.CatalogConditionRefreshed.IsUnknown(clusterCatalog) {
			v3.CatalogConditionRefreshed.True(clusterCatalog)
			v3.CatalogConditionRefreshed.Reason(clusterCatalog, "")
			m.clusterCatalogClient.Update(clusterCatalog)
		}
		return nil
	}

	logrus.Infof("Updating catalog %s", clusterCatalog.Name)
	return m.traverseAndUpdate(repoPath, commit, cmt)
}

func (m *Manager) commonSync(key string, obj *v3.Catalog) error {
	if obj == nil {
		return nil
	}
	if obj.DeletionTimestamp != nil {
		templates, err := m.getTemplateMap(obj.Name)
		if err != nil {
			return err
		}
		tvToDelete := map[string]struct{}{}
		for _, t := range templates {
			tvs, err := m.getTemplateVersion(t.Name)
			if err != nil {
				return err
			}
			for k := range tvs {
				tvToDelete[k] = struct{}{}
			}
		}
		go func() {
			for {
				for k := range templates {
					if err := m.templateClient.Delete(k, &metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
						logrus.Warnf("Deleting template %v doesn't succeed. Continue loop", k)
						continue
					}
				}
				for k := range tvToDelete {
					if err := m.templateVersionClient.Delete(k, &metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
						logrus.Warnf("Deleting templateVersion %v doesn't succeed. Continue loop", k)
						continue
					}
				}
				break
			}
		}()
		return nil
	}
	return nil
}
