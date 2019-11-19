package manager

import (
	"time"

	"k8s.io/apimachinery/pkg/api/errors"

	helmlib "github.com/rancher/rancher/pkg/catalog/helm"
	"github.com/rancher/rancher/pkg/namespace"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
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
		if (catalog.Name == "library" || catalog.Name == "system-library") && catalog.Spec.CatalogKind != helmlib.KindHelmInternal {
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

	if isUpToDate(commit, catalog) {
		if setRefreshed(catalog) {
			m.catalogClient.Update(catalog)
		}
		return nil, nil
	}

	cmt := &CatalogInfo{
		catalog: catalog,
	}

	backoff := wait.Backoff{
		Duration: 1 * time.Second,
		Factor:   2,
		Jitter:   0,
		Steps:    5,
	}

	err = wait.ExponentialBackoff(backoff, func() (bool, error) {
		logrus.Infof("Updating global catalog %s", catalog.Name)
		err := m.traverseAndUpdate(helm, commit, cmt)
		if err != nil {
			logrus.Error(err)
			if errors.IsNotFound(err) || errors.IsGone(err) || errors.IsResourceExpired(err) || errors.IsConflict(err) || errors.IsInvalid(err) || err.Error() == "Catalog is no longer available" {
				return true, nil
			}
			return false, nil
		}
		return true, nil
	})
	return nil, nil

}
