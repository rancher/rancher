package manager

import (
	"github.com/pkg/errors"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

func (m *Manager) Sync(key string, catalog *v3.Catalog) error {
	// if catalog was deleted, do nothing
	if catalog == nil || catalog.DeletionTimestamp != nil {
		return nil
	}
	catalogCopy := catalog.DeepCopy()
	repoPath, commit, catalogType, err := m.prepareRepoPath(*catalogCopy, true)
	if commit == catalogCopy.Status.Commit {
		logrus.Debugf("Catalog %s is already up to date", catalog.Name)
		return nil
	}
	catalogCopy.Status.Commit = commit
	templates, errs, err := traverseFiles(repoPath, catalogCopy, catalogType)
	if err != nil {
		return errors.Wrap(err, "Repo traversal failed")
	}
	if len(errs) != 0 {
		logrus.Errorf("Errors while parsing repo: %v", errs)
	}

	logrus.Infof("Updating catalog %s", catalog.Name)
	// since helm catalog is purely additive, we add update only flag
	updateOnly := false
	if catalogType == CatalogTypeHelmObjectRepo {
		updateOnly = true
	}
	return m.update(catalogCopy, templates, updateOnly)
}
