package manager

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (m *Manager) Sync(key string, catalog *v3.Catalog) error {
	// if catalog was deleted, do nothing
	if catalog == nil || catalog.DeletionTimestamp != nil {
		// remove all the templates associated with catalog
		logrus.Infof("Cleaning up catalog %s", key)
		templates, err := m.templateClient.List(metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", CatalogNameLabel, key),
		})
		if err != nil {
			return err
		}
		for _, template := range templates.Items {
			logrus.Infof("Cleaning up template %s", template.Name)
			if err := m.templateClient.Delete(template.Name, &metav1.DeleteOptions{}); err != nil {
				return err
			}
			if err := m.deleteTemplateVersions(template); err != nil {
				return err
			}
		}
		return nil
	}
	catalogCopy := catalog.DeepCopy()
	repoPath, commit, catalogType, err := m.prepareRepoPath(*catalogCopy, true)
	if commit == catalogCopy.Status.Commit {
		logrus.Debugf("Catalog %s is already up to date", catalog.Name)
		return nil
	}
	catalogCopy.Status.Commit = commit
	templates, errs, err := traverseFiles(repoPath, catalog.Spec.CatalogKind, catalog.Name, catalogType)
	if err != nil {
		return errors.Wrap(err, "Repo traversal failed")
	}
	if len(errs) != 0 {
		logrus.Errorf("Errors while parsing repo: %v", errs)
	}

	logrus.Debugf("Updating catalog %s", catalog.Name)
	return m.update(catalogCopy, templates)
}
