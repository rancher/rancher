package manager

import (
	helmlib "github.com/rancher/rancher/pkg/catalog/helm"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/namespace"
	"github.com/sirupsen/logrus"
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
	return nil, m.traverseAndUpdate(helm, commit, cmt)
}
