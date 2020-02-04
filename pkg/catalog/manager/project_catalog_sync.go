package manager

import (
	helmlib "github.com/rancher/rancher/pkg/catalog/helm"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

func (m *Manager) updateProjectCatalogError(projectCatalog *v3.ProjectCatalog, err error) (runtime.Object, error) {
	setRefreshedError(&projectCatalog.Catalog, err)
	m.projectCatalogClient.Update(projectCatalog)
	return nil, err
}

func (m *Manager) ProjectCatalogSync(key string, obj *v3.ProjectCatalog) (runtime.Object, error) {
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, err
	}

	if obj == nil {
		return nil, m.deleteTemplates(name, ns)
	}

	// always get a refresh catalog from etcd
	projectCatalog, err := m.projectCatalogClient.GetNamespaced(ns, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// if the catalog was processed but some templates had errors due to local/file urls or chart names
	// that cannot be used as labels - the catalog is up to date, but had errors so we don't refresh
	// to give time for the user to make the necessary corrections.
	if v3.CatalogConditionProcessed.IsFalse(projectCatalog) && v3.CatalogConditionRefreshed.IsTrue(projectCatalog) {
		return nil, nil
	}

	commit, helm, err := helmlib.NewForceUpdate(&projectCatalog.Catalog)
	if err != nil {
		return m.updateProjectCatalogError(projectCatalog, err)
	}
	logrus.Debugf("Chart hash comparison for project catalog %v: new -- %v --- current -- %v", projectCatalog.Name, commit, &projectCatalog.Catalog.Status.Commit)

	if isUpToDate(commit, &projectCatalog.Catalog) {
		if setRefreshed(&projectCatalog.Catalog) {
			m.projectCatalogClient.Update(projectCatalog)
		}
		return nil, nil
	}

	cmt := &CatalogInfo{
		catalog:        &projectCatalog.Catalog,
		projectCatalog: projectCatalog,
	}

	logrus.Infof("Updating project catalog %s", projectCatalog.Name)
	return nil, m.traverseAndUpdate(helm, commit, cmt)
}
