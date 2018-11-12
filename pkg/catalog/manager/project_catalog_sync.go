package manager

import (
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

func (m *Manager) ProjectCatalogSync(key string, obj *v3.ProjectCatalog) (runtime.Object, error) {
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, err
	}

	if obj == nil {
		return nil, m.deleteTemplates(name)
	}

	// always get a refresh catalog from etcd
	projectCatalog, err := m.projectCatalogClient.GetNamespaced(ns, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	repoPath, commit, err := m.prepareRepoPath(obj.Catalog)
	if err != nil {
		v3.CatalogConditionRefreshed.False(projectCatalog)
		v3.CatalogConditionRefreshed.ReasonAndMessageFromError(projectCatalog, err)
		m.projectCatalogClient.Update(projectCatalog)
		return nil, err
	}

	if commit == projectCatalog.Status.Commit {
		logrus.Debugf("Project catalog %s is already up to date", projectCatalog.Name)
		if !v3.CatalogConditionRefreshed.IsTrue(projectCatalog) {
			v3.CatalogConditionRefreshed.True(projectCatalog)
			v3.CatalogConditionRefreshed.Reason(projectCatalog, "")
			v3.CatalogConditionRefreshed.Message(projectCatalog, "")
			m.projectCatalogClient.Update(projectCatalog)
		}
		return nil, nil
	}

	cmt := &CatalogInfo{
		catalog:        &projectCatalog.Catalog,
		projectCatalog: projectCatalog,
	}

	logrus.Infof("Updating project catalog %s", projectCatalog.Name)
	return nil, m.traverseAndUpdate(repoPath, commit, cmt)
}
