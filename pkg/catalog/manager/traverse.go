package manager

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/catalog/helm"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (m *Manager) traverseAndUpdate(repoPath, commit string, catalog *v3.Catalog) error {
	index, err := helm.LoadIndex(repoPath)
	if err != nil {
		return err
	}

	// list all existing templates
	templateMap, err := m.getTemplateMap(catalog.Name)
	if err != nil {
		return err
	}
	// list all templateContent tag
	templateContentList, err := m.templateContentLister.List("", labels.NewSelector())
	if err != nil {
		return err
	}
	templateContentMap := map[string]struct{}{}
	for _, t := range templateContentList {
		templateContentMap[t.Name] = struct{}{}
	}

	newHelmVersionCommits := map[string]v3.VersionCommits{}
	var errs []error
	var terrors []error
	createdTemplates, updatedTemplates, deletedTemplates := 0, 0, 0
	for chart, metadata := range index.IndexFile.Entries {
		newHelmVersionCommits[chart] = v3.VersionCommits{
			Value: map[string]string{},
		}
		existingHelmVersionCommits := map[string]string{}
		if catalog.Status.HelmVersionCommits[chart].Value != nil {
			existingHelmVersionCommits = catalog.Status.HelmVersionCommits[chart].Value
		}
		keywords := map[string]struct{}{}
		// comparing version commit with the previous commit to detect if a template has been changed.
		hasChanged := false
		versionNumber := 0
		for _, version := range metadata {
			newHelmVersionCommits[chart].Value[version.Version] = version.Digest
			digest, ok := existingHelmVersionCommits[version.Version]
			if !ok || digest != version.Digest {
				hasChanged = true
			}
			if ok {
				versionNumber++
			}
		}
		// if there is a version getting deleted then also set hasChanged to true
		if versionNumber != len(existingHelmVersionCommits) {
			hasChanged = true
		}
		if !hasChanged {
			logrus.Debugf("chart %s has not been changed. Skipping generating templates for it", chart)
			continue
		}

		template := v3.Template{
			ObjectMeta: metav1.ObjectMeta{
				Name: chart,
			},
		}
		template.Spec.Description = metadata[0].Description
		template.Spec.DefaultVersion = metadata[0].Version
		if len(metadata[0].Sources) > 0 {
			template.Spec.ProjectURL = metadata[0].Sources[0]
		}
		iconData, iconFilename, err := helm.Icon(metadata)
		if err != nil {
			errs = append(errs, err)
		}
		template.Spec.Icon = iconData
		template.Spec.IconFilename = iconFilename
		template.Spec.FolderName = chart
		template.Spec.DisplayName = chart
		label := map[string]string{}
		var versions []v3.TemplateVersionSpec
		for _, version := range metadata {
			v := v3.TemplateVersionSpec{
				Version: version.Version,
			}
			files, err := helm.FetchFiles(version, version.URLs)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			filesToAdd := make(map[string]string)

			for _, file := range files {
				if strings.EqualFold(fmt.Sprintf("%s/%s", chart, "readme.md"), file.Name) {
					v.Readme = file.Contents
				}
				for _, f := range supportedFiles {
					if strings.EqualFold(fmt.Sprintf("%s/%s", chart, f), file.Name) {
						var value catalogYml
						if err := yaml.Unmarshal([]byte(file.Contents), &value); err != nil {
							return err
						}
						v.Questions = value.Questions
						v.RancherVersion = value.RancherVersion
						v.RequiredNamespace = value.Namespace
						label = labels.Merge(label, value.Labels)
						for _, category := range value.Categories {
							keywords[category] = struct{}{}
						}
						break
					}
				}
				if strings.EqualFold(fmt.Sprintf("%s/%s", chart, "app-readme.md"), file.Name) {
					v.AppReadme = file.Contents
				}
				filesToAdd[file.Name] = file.Contents
			}
			v.Files = filesToAdd
			v.KubeVersion = version.KubeVersion
			v.Digest = version.Digest
			v.UpgradeVersionLinks = map[string]string{}
			for _, versionSpec := range template.Spec.Versions {
				if showUpgradeLinks(v.Version, versionSpec.Version) {
					version := versionSpec.Version
					v.UpgradeVersionLinks[versionSpec.Version] = fmt.Sprintf("%s-%s", template.Name, version)
				}
			}

			v.ExternalID = fmt.Sprintf("catalog://?catalog=%s&template=%s&version=%s", catalog.Name, template.Spec.FolderName, v.Version)
			versions = append(versions, v)
		}
		var categories []string
		for k := range keywords {
			categories = append(categories, k)
		}
		// merge all labels from templateVersion to template
		template.Labels = label
		template.Spec.Categories = categories
		template.Spec.Versions = versions
		template.Spec.CatalogID = catalog.Name
		template.Name = fmt.Sprintf("%s-%s", catalog.Name, template.Spec.FolderName)

		v3.CatalogConditionRefreshed.Unknown(catalog)
		v3.CatalogConditionRefreshed.Message(catalog, fmt.Sprintf("syncing template %v", template.Name))
		if newCatalog, err := m.catalogClient.Update(catalog); err == nil {
			catalog = newCatalog
		} else {
			catalog, _ = m.catalogClient.Get(catalog.Name, metav1.GetOptions{})
		}
		var temErr error
		// look template by name, if not found then create it, otherwise do update
		if existing, ok := templateMap[template.Name]; ok {
			if err := m.updateTemplate(existing, template, templateContentMap); err != nil {
				temErr = err
			}
			updatedTemplates++
		} else {
			if err := m.createTemplate(template, catalog, templateContentMap); err != nil {
				temErr = err
			}
			createdTemplates++
		}
		if temErr != nil {
			delete(newHelmVersionCommits, template.Spec.DisplayName)
			terrors = append(terrors, temErr)
		}
	}

	toDeleteChart := []string{}
	for chart := range catalog.Status.HelmVersionCommits {
		if _, ok := index.IndexFile.Entries[chart]; !ok {
			toDeleteChart = append(toDeleteChart, fmt.Sprintf("%s-%s", catalog.Name, chart))
		}
	}
	// delete non-existing templates
	for _, toDelete := range toDeleteChart {
		logrus.Debugf("Deleting template %s and its associated templateVersion", toDelete)
		if err := m.deleteChart(toDelete); err != nil {
			return err
		}
		deletedTemplates++
	}
	logrus.Infof("Catalog sync done. %v templates created, %v templates updated, %v templates deleted", createdTemplates, updatedTemplates, deletedTemplates)

	catalog.Status.HelmVersionCommits = newHelmVersionCommits
	if len(terrors) > 0 {
		if _, err := m.catalogClient.Update(catalog); err != nil {
			return err
		}
		return errors.Errorf("failed to update templates. Multiple error occurred: %v", terrors)
	}
	var finalError error
	if len(errs) > 0 {
		finalError = errors.Errorf("failed to sync templates. Resetting commit. Multiple error occurred: %v", errs)
		commit = ""
	}

	v3.CatalogConditionRefreshed.True(catalog)
	v3.CatalogConditionRefreshed.Message(catalog, "")
	catalog.Status.Commit = commit
	if _, err := m.catalogClient.Update(catalog); err != nil {
		return err
	}
	return finalError
}

var supportedFiles = []string{"catalog.yml", "catalog.yaml", "questions.yml", "questions.yaml"}

type catalogYml struct {
	RancherVersion string            `yaml:"rancher_version,omitempty"`
	Categories     []string          `yaml:"categories,omitempty"`
	Questions      []v3.Question     `yaml:"questions,omitempty"`
	Namespace      string            `yaml:"namespace,omitempty"`
	Labels         map[string]string `yaml:"labels,omitempty"`
}
