package manager

import (
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"

	"github.com/rancher/rancher/pkg/namespace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (m *Manager) Sync(key string, obj *v3.Catalog) (runtime.Object, error) {
	if obj == nil {
		return nil, m.deleteTemplates(key, namespace.GlobalNamespace)
	}

	// always get a refresh catalog from etcd
	catalog, err := m.catalogClient.Get(key, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	repoPath, commit, err := m.prepareRepoPath(*catalog)

	if err != nil {
		v3.CatalogConditionRefreshed.False(catalog)
		v3.CatalogConditionRefreshed.ReasonAndMessageFromError(catalog, err)
		m.catalogClient.Update(catalog)
		return nil, err
	}

	// the upgraded condition won't be true upon upgrade; this will cause traverse to be called
	upgraded := v3.CatalogConditionUpgraded.IsTrue(obj)
	if commit == catalog.Status.Commit && upgraded {
		logrus.Debugf("Catalog %s is already up to date", catalog.Name)
		if !v3.CatalogConditionRefreshed.IsTrue(catalog) {
			v3.CatalogConditionRefreshed.True(catalog)
			v3.CatalogConditionRefreshed.Reason(catalog, "")
			v3.CatalogConditionRefreshed.Message(catalog, "")
			m.catalogClient.Update(catalog)
		}
		return nil, nil
	}

	cmt := &CatalogInfo{
		catalog: catalog,
	}

	logrus.Infof("Updating catalog %s", catalog.Name)
	return nil, m.traverseAndUpdate(repoPath, commit, cmt, upgraded)
}
