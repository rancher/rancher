package manager

import (
	helmlib "github.com/rancher/rancher/pkg/catalog/helm"
	"github.com/rancher/rancher/pkg/namespace"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
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

	// if the catalog was processed but some templates had errors due to local/file urls or chart names
	// that cannot be used as labels - the catalog is up to date, but had errors so we don't refresh
	// to give time for the user to make the necessary corrections.
	if v3.CatalogConditionProcessed.IsFalse(catalog) && v3.CatalogConditionRefreshed.IsTrue(catalog) {
		return nil, nil
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
