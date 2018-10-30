package manager

import (
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (m *Manager) Sync(key string, obj *v3.Catalog) (*v3.Catalog, error) {
	if obj == nil {
		return nil, m.deleteTemplates(key)
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

	if commit == catalog.Status.Commit {
		logrus.Debugf("Catalog %s is already up to date", catalog.Name)
		if v3.CatalogConditionRefreshed.IsUnknown(catalog) {
			v3.CatalogConditionRefreshed.True(catalog)
			v3.CatalogConditionRefreshed.Reason(catalog, "")
			m.catalogClient.Update(catalog)
		}
		return nil, nil
	}

	cmt := &CatalogInfo{
		catalog: catalog,
	}

	logrus.Infof("Updating catalog %s", catalog.Name)
	return nil, m.traverseAndUpdate(repoPath, commit, cmt)
}
