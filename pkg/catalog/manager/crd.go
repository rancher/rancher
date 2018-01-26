package manager

import (
	"fmt"

	"strings"

	"strconv"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/catalog/utils"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	CatalogNameLabel  = "catalog.cattle.io/name"
	TemplateNameLabel = "catalog.cattle.io/template_name"
)

// update will sync templates with catalog without costing too much
func (m *Manager) update(catalog *v3.Catalog, templates []v3.Template) error {
	logrus.Debugf("Syncing catalog %s with templates", catalog.Name)
	existingTemplates, err := m.templateClient.List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", CatalogNameLabel, catalog.Name),
	})
	if err != nil {
		return err
	}

	templatesByName := map[string]v3.Template{}
	for _, template := range templates {
		if template.Spec.FolderName == "" {
			continue
		} else if template.Spec.Base == "" && template.Spec.FolderName != "" {
			template.Name = fmt.Sprintf("%s-%s", catalog.Name, template.Spec.FolderName)
		} else {
			template.Name = fmt.Sprintf("%s-%s-%s", catalog.Name, template.Spec.Base, template.Spec.FolderName)
		}
		template.Name = strings.ToLower(template.Name)
		templatesByName[template.Name] = template
	}

	existingTemplatesByName := map[string]v3.Template{}
	for _, template := range existingTemplates.Items {
		existingTemplatesByName[template.Name] = template
	}

	// templates is the one we should update, so for all the templates that were in existingTemplates
	// 1. if it doesn't exist in templates, delete them
	// 2. if it exists but has changed, update it
	// 3. if it exists but not changed, keep it unmodified
	var errs []error
	for name, existingTemplate := range existingTemplatesByName {
		err := m.updateTemplate(name, existingTemplate, templatesByName)
		if err != nil {
			errs = append(errs, err)
		}
	}

	// for templates that exist in template but not in existingTemplates, we should create them
	for name, template := range templatesByName {
		err := m.createTemplate(name, template, existingTemplatesByName, catalog)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Errorf("Multiple errors happens: %v", errs)
	}

	if _, err := m.catalogClient.Update(catalog); err != nil {
		return err
	}
	return nil
}

func (m *Manager) createTemplate(name string, template v3.Template, existingTemplatesByName map[string]v3.Template, catalog *v3.Catalog) error {
	if _, ok := existingTemplatesByName[name]; !ok {
		template.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: catalog.APIVersion,
				Kind:       catalog.Kind,
				Name:       catalog.Name,
				UID:        catalog.UID,
			},
		}
		template.Kind = v3.TemplateGroupVersionKind.Kind
		template.APIVersion = v3.TemplateGroupVersionKind.Group + "/" + v3.TemplateGroupVersionKind.Version
		template.Labels = map[string]string{}
		template.Labels[CatalogNameLabel] = catalog.Name
		versionFiles := template.Spec.Versions
		// we are removing file fields so that the big chunk of data doesn't get stored in two places
		modifiedVersionFiles := []v3.TemplateVersionSpec{}
		for _, version := range template.Spec.Versions {
			version.Files = nil
			version.Readme = ""
			modifiedVersionFiles = append(modifiedVersionFiles, version)
		}
		template.Spec.Versions = modifiedVersionFiles
		logrus.Debugf("Creating template %s", template.Name)
		createdTemplate, err := m.templateClient.Create(&template)
		if err != nil {
			// hack for the image size that are too big
			if strings.Contains(err.Error(), "request is too large") || strings.Contains(err.Error(), "exceeding the max size") {
				logrus.Warnf("Template %s size is too large. Skipping", template.Name)
				return nil
			}
			return err
		}
		if err := m.createTemplateVersions(versionFiles, *createdTemplate); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) updateTemplate(name string, existingTemplate v3.Template, templatesByName map[string]v3.Template) error {
	template, ok := templatesByName[name]
	if !ok {
		// delete the template
		logrus.Debugf("Deleting templates %s", name)
		if err := m.templateClient.Delete(name, &metav1.DeleteOptions{}); err != nil {
			return errors.Wrapf(err, "failed to delete template %s", template.Name)
		}
		if err := m.deleteTemplateVersions(existingTemplate); err != nil {
			return errors.Wrapf(err, "failed to delete templateVersion with template %s", template.Name)
		}
	}

	updateTemplate, err := m.templateClient.Get(name, metav1.GetOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return err
	} else if kerrors.IsNotFound(err) {
		return nil
	}
	updateTemplate.Spec = template.Spec
	template.Spec.Versions = updateTemplate.Spec.Versions
	modifiedVersionFiles := []v3.TemplateVersionSpec{}
	for _, version := range updateTemplate.Spec.Versions {
		version.Files = nil
		version.Readme = ""
		modifiedVersionFiles = append(modifiedVersionFiles, version)
	}
	updateTemplate.Spec.Versions = modifiedVersionFiles
	logrus.Debugf("Updating template %s", name)
	result, err := m.templateClient.Update(updateTemplate)
	if err != nil {
		if strings.Contains(err.Error(), "request is too large") || strings.Contains(err.Error(), "exceeding the max size") {
			logrus.Warnf("Template %s size is too large. Skipping", template.Name)
			return nil
		}
		return errors.Wrapf(err, "failed to update template %s", template.Name)
	}
	if err := m.deleteTemplateVersions(*result); err != nil {
		return err
	}
	if err := m.createTemplateVersions(template.Spec.Versions, *result); err != nil {
		return err
	}
	return nil
}

func (m *Manager) createTemplateVersions(versionsSpec []v3.TemplateVersionSpec, template v3.Template) error {
	createdTemplates := []string{}
	rollback := false
	for _, spec := range versionsSpec {
		templateVersion := v3.TemplateVersion{}
		templateVersion.Spec = spec
		revision := spec.Version
		if spec.Revision != nil {
			revision = strconv.Itoa(*spec.Revision)
		}
		templateVersion.APIVersion = v3.TemplateVersionGroupVersionKind.Group + "/" + v3.TemplateVersionGroupVersionKind.Version
		templateVersion.Kind = v3.TemplateVersionGroupVersionKind.Kind
		templateVersion.Name = fmt.Sprintf("%s-%v", template.Name, revision)
		templateVersion.Labels = make(map[string]string)
		templateVersion.Labels[TemplateNameLabel] = template.Name
		templateVersion.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: template.APIVersion,
				Kind:       template.Kind,
				Name:       template.Name,
				UID:        template.UID,
			},
		}
		logrus.Debugf("Creating templateVersion %s", templateVersion.Name)
		_, err := m.templateVersionClient.Create(&templateVersion)
		if err != nil {
			logrus.Error(err)
			rollback = true
			break
		}
		createdTemplates = append(createdTemplates, templateVersion.Name)
	}
	if rollback {
		logrus.Debug("Rollback TemplateVersion")
		for _, name := range createdTemplates {
			logrus.Debugf("Deleting templateVersion %s", name)
			err := m.templateVersionClient.Delete(name, &metav1.DeleteOptions{})
			if err != nil && !kerrors.IsNotFound(err) {
				return err
			}
		}
		return nil
	}
	return nil
}

func (m *Manager) deleteTemplateVersions(template v3.Template) error {
	templateVersions, err := m.templateVersionClient.List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", TemplateNameLabel, template.Name),
	})
	if err != nil {
		return err
	}
	for _, version := range templateVersions.Items {
		logrus.Debugf("Deleting templateVersion %s", version.Name)
		if err := m.templateVersionClient.Delete(version.Name, &metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func showUpgradeLinks(version, upgradeVersion, upgradeFrom string) bool {
	if !utils.VersionGreaterThan(upgradeVersion, version) {
		return false
	}
	if upgradeFrom != "" {
		satisfiesRange, err := utils.VersionSatisfiesRange(version, upgradeFrom)
		if err != nil {
			return false
		}
		return satisfiesRange
	}
	return true
}
