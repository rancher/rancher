package manager

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/rancher/rancher/pkg/catalog/helm"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func traverseFiles(repoPath string, catalog *v3.Catalog) ([]v3.Template, []error, error) {
	index, err := helm.LoadIndex(repoPath)
	if err != nil {
		return nil, nil, err
	}
	newHelmVersionCommits := map[string]v3.VersionCommits{}

	var templates []v3.Template
	var errors []error
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
		for _, version := range metadata {
			newHelmVersionCommits[chart].Value[version.Version] = version.Digest
			if digest, ok := existingHelmVersionCommits[version.Version]; !ok || digest != version.Digest {
				hasChanged = true
			}
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
			errors = append(errors, err)
		}
		template.Spec.Icon = iconData
		template.Spec.IconFilename = iconFilename
		template.Spec.Base = HelmTemplateBaseType
		template.Spec.FolderName = chart
		template.Spec.DisplayName = chart
		var versions []v3.TemplateVersionSpec

		for _, version := range metadata {
			v := v3.TemplateVersionSpec{
				Version: version.Version,
			}
			for _, k := range version.Keywords {
				keywords[k] = struct{}{}
			}
			files, err := helm.FetchFiles(version, version.URLs)
			if err != nil {
				errors = append(errors, err)
				continue
			}
			var filesToAdd []v3.File
			for _, file := range files {
				if strings.EqualFold(fmt.Sprintf("%s/%s", chart, "readme.md"), file.Name) {
					contents, err := base64.StdEncoding.DecodeString(file.Contents)
					if err != nil {
						return nil, nil, err
					}
					v.Readme = string(contents)
					continue
				}
				filesToAdd = append(filesToAdd, file)
			}
			v.Files = filesToAdd
			v.UpgradeVersionLinks = map[string]string{}
			for _, versionSpec := range template.Spec.Versions {
				if showUpgradeLinks(v.Version, versionSpec.Version, versionSpec.UpgradeFrom) {
					revision := versionSpec.Version
					if v.Revision != nil {
						revision = strconv.Itoa(*versionSpec.Revision)
					}
					v.UpgradeVersionLinks[versionSpec.Version] = fmt.Sprintf("%s-%s", template.Name, revision)
				}
			}
			v.ExternalID = fmt.Sprintf("catalog://?catalog=%s&base=%s&template=%s&version=%s", catalog.Name, template.Spec.Base, template.Spec.FolderName, v.Version)
			versions = append(versions, v)
		}
		var categories []string
		for k := range keywords {
			categories = append(categories, k)
		}
		template.Spec.Categories = categories
		template.Spec.Versions = versions
		template.Spec.CatalogID = catalog.Name

		templates = append(templates, template)
	}
	catalog.Status.HelmVersionCommits = newHelmVersionCommits
	return templates, nil, nil
}
