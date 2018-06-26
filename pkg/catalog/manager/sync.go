package manager

import (
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (m *Manager) Sync(key string, obj *v3.Catalog) error {
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
	return m.traverseAndUpdate(repoPath, commit, catalog)
}
