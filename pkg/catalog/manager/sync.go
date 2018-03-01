package manager

import (
	"github.com/pkg/errors"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

func (m *Manager) Sync(key string, obj *v3.Catalog) error {
	// if catalog was deleted, do nothing
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil
	}

	catalog := obj.DeepCopy()

	repoPath, commit, err := m.prepareRepoPath(*catalog)
	if commit == catalog.Status.Commit {
		logrus.Debugf("Catalog %s is already up to date", catalog.Name)
		if v3.CatalogConditionRefreshed.IsUnknown(catalog) {
			v3.CatalogConditionRefreshed.True(catalog)
			m.catalogClient.Update(catalog)
		}
		return nil
	}

	catalog.Status.Commit = commit
	templates, errs, err := traverseFiles(repoPath, catalog)
	if err != nil {
		return errors.Wrap(err, "Repo traversal failed")
	}
	if len(errs) != 0 {
		logrus.Errorf("Errors while parsing repo: %v", errs)
	}

	logrus.Infof("Updating catalog %s", catalog.Name)
	return m.update(catalog, templates, true)
}
