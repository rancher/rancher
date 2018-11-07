package manager

import (
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"

	helmlib "github.com/rancher/rancher/pkg/catalog/helm"
	"github.com/rancher/rancher/pkg/namespace"
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

	commit, helm, err := helmlib.NewForceUpdate(catalog)
	if err != nil {
		return m.updateCatalogError(catalog, err)
	}

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
	return nil, m.traverseAndUpdate(helm, commit, cmt)
}
