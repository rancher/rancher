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

	catalog = catalog.DeepCopy()
	repoPath, commit, err := m.prepareRepoPath(*catalog)
	if commit == catalog.Status.Commit {
		logrus.Debugf("Catalog %s is already up to date", catalog.Name)
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
