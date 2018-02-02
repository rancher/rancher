package manager

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/blang/semver"
	"github.com/rancher/rancher/pkg/catalog/helm"
	"github.com/rancher/rancher/pkg/catalog/parse"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func traverseFiles(repoPath string, catalog *v3.Catalog, catalogType CatalogType) ([]v3.Template, []error, error) {
	if catalog.Spec.CatalogKind == "" || catalog.Spec.CatalogKind == RancherTemplateType {
		return traverseGitFiles(repoPath, catalog.Name)
	}
	if catalog.Spec.CatalogKind == HelmTemplateType {
		if catalogType == CatalogTypeHelmGitRepo {
			return traverseHelmGitFiles(repoPath, catalog.Name)
		}
		return traverseHelmFiles(repoPath, catalog)
	}
	return nil, nil, fmt.Errorf("Unknown kind %s", catalog.Spec.CatalogKind)
}

func traverseHelmGitFiles(repoPath, catalogName string) ([]v3.Template, []error, error) {
	fullpath := path.Join(repoPath, "stable")

	templates := []v3.Template{}
	var template *v3.Template
	errors := []error{}
	err := filepath.Walk(fullpath, func(path string, info os.FileInfo, err error) error {
		if len(path) == len(fullpath) {
			return nil
		}
		relPath := path[len(fullpath)+1:]
		components := strings.Split(relPath, "/")
		if len(components) == 1 {
			if template != nil {
				templates = append(templates, *template)
			}
			template = new(v3.Template)
			template.Spec.Versions = make([]v3.TemplateVersionSpec, 0)
			template.Spec.Versions = append(template.Spec.Versions, v3.TemplateVersionSpec{
				Files: make([]v3.File, 0),
			})
			template.Spec.Base = HelmTemplateBaseType
		}
		if info.IsDir() {
			return nil
		}

		if strings.HasSuffix(info.Name(), "Chart.yaml") {
			metadata, err := helm.LoadMetadata(path)
			if err != nil {
				return err
			}
			template.Spec.Description = metadata.Description
			template.Spec.DefaultVersion = metadata.Version
			if len(metadata.Sources) > 0 {
				template.Spec.ProjectURL = metadata.Sources[0]
			}
			iconData, iconFilename, err := parse.Icon(metadata.Icon)
			if err != nil {
				errors = append(errors, err)
			}
			rev := 0
			template.Spec.Icon = iconData
			template.Spec.IconFilename = iconFilename
			template.Spec.FolderName = components[0]
			template.Spec.CatalogID = catalogName
			template.Name = components[0]
			template.Spec.Versions[0].Revision = &rev
			template.Spec.Versions[0].Version = metadata.Version
		}
		file, err := helm.LoadFile(path)
		if err != nil {
			return err
		}

		file.Name = relPath

		if strings.HasSuffix(info.Name(), "README.md") {
			template.Spec.Versions[0].Readme = file.Contents
			return nil
		}

		template.Spec.Versions[0].Files = append(template.Spec.Versions[0].Files, *file)

		return nil
	})
	return templates, errors, err
}

func traverseHelmFiles(repoPath string, catalog *v3.Catalog) ([]v3.Template, []error, error) {
	index, err := helm.LoadIndex(repoPath)
	if err != nil {
		return nil, nil, err
	}
	newHelmVersionCommits := make(map[string]v3.VersionCommits)

	templates := []v3.Template{}
	var errors []error
	for chart, metadata := range index.IndexFile.Entries {
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
		iconData, iconFilename, err := parse.Icon(metadata[0].Icon)
		if err != nil {
			errors = append(errors, err)
		}
		template.Spec.Icon = iconData
		template.Spec.IconFilename = iconFilename
		template.Spec.Base = HelmTemplateBaseType
		template.Spec.FolderName = chart
		versions := make([]v3.TemplateVersionSpec, 0)

		newHelmVersionCommits[chart] = v3.VersionCommits{
			Value: map[string]string{},
		}
		existingHelmVersionCommits := map[string]string{}
		if catalog.Status.HelmVersionCommits[chart].Value != nil {
			existingHelmVersionCommits = catalog.Status.HelmVersionCommits[chart].Value
		}
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
		for _, version := range metadata {
			v := v3.TemplateVersionSpec{
				Version: version.Version,
			}
			files, err := helm.FetchFiles(version.URLs)
			if err != nil {
				fmt.Println(err)
				errors = append(errors, err)
				continue
			}
			filesToAdd := []v3.File{}
			for _, file := range files {
				if strings.EqualFold(fmt.Sprintf("%s/%s", chart, "readme.md"), file.Name) {
					v.Readme = file.Contents
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
		template.Spec.Versions = versions
		template.Spec.CatalogID = catalog.Name

		templates = append(templates, template)
	}
	catalog.Status.HelmVersionCommits = newHelmVersionCommits
	return templates, nil, nil
}

func traverseGitFiles(repoPath, catalogName string) ([]v3.Template, []error, error) {
	templateIndex := map[string]*v3.Template{}
	var errors []error

	if err := filepath.Walk(repoPath, func(fullPath string, f os.FileInfo, err error) error {
		if f == nil || !f.Mode().IsRegular() {
			return nil
		}

		relativePath, err := filepath.Rel(repoPath, fullPath)
		if err != nil {
			return err
		}

		_, _, parsedCorrectly := parse.TemplatePath(relativePath)
		if !parsedCorrectly {
			return nil
		}

		_, filename := path.Split(relativePath)

		if err = handleFile(templateIndex, fullPath, relativePath, filename); err != nil {
			errors = append(errors, fmt.Errorf("%s: %v", fullPath, err))
		}

		return nil
	}); err != nil {
		return nil, nil, err
	}

	templates := []v3.Template{}
	for _, template := range templateIndex {
		for i, version := range template.Spec.Versions {
			var readme string
			for _, file := range version.Files {
				if strings.ToLower(file.Name) == "readme.md" {
					readme = file.Contents
				}
			}

			var compose string
			var rancherCompose string
			var templateVersion string
			for _, file := range version.Files {
				switch file.Name {
				case "template-version.yml":
					templateVersion = file.Contents
				case "compose.yml":
					compose = file.Contents
				case "rancher-compose.yml":
					rancherCompose = file.Contents
				}
			}
			newVersion := version
			if templateVersion != "" || compose != "" || rancherCompose != "" {
				var err error
				if templateVersion != "" {
					newVersion, err = parse.CatalogInfoFromTemplateVersion([]byte(templateVersion))
				}
				if compose != "" {
					newVersion, err = parse.CatalogInfoFromCompose([]byte(compose))
				}
				if rancherCompose != "" {
					newVersion, err = parse.CatalogInfoFromRancherCompose([]byte(rancherCompose))
				}

				if err != nil {
					var id string
					if template.Spec.Base == "" {
						id = fmt.Sprintf("%s:%d", template.Spec.FolderName, i)
					} else {
						id = fmt.Sprintf("%s*%s:%d", template.Spec.Base, template.Spec.FolderName, i)
					}
					errors = append(errors, fmt.Errorf("Failed to parse rancher-compose.yml for %s: %v", id, err))
					continue
				}
				newVersion.Revision = version.Revision
				// If rancher-compose.yml contains version, use this instead of folder version
				if newVersion.Version == "" {
					newVersion.Version = version.Version
				}
				newVersion.Files = version.Files
			}
			newVersion.Readme = readme

			template.Spec.Versions[i] = newVersion
		}
		var filteredVersions []v3.TemplateVersionSpec
		for _, version := range template.Spec.Versions {
			if version.Version != "" {
				filteredVersions = append(filteredVersions, version)
			}
		}
		template.Spec.Versions = filteredVersions
		template.Spec.CatalogID = catalogName
		for _, versionSpec := range template.Spec.Versions {
			if versionSpec.Version == template.Spec.DefaultVersion {
				templateName := ""
				if template.Spec.Base == "" && template.Spec.FolderName != "" {
					templateName = fmt.Sprintf("%s-%s", catalogName, template.Spec.FolderName)
				} else {
					templateName = fmt.Sprintf("%s-%s-%s", catalogName, template.Spec.Base, template.Spec.FolderName)
				}
				revision := versionSpec.Version
				if versionSpec.Revision != nil {
					revision = strconv.Itoa(*versionSpec.Revision)
				}
				template.Spec.DefaultTemplateVersionID = fmt.Sprintf("%s-%s", templateName, revision)
			}
		}
		templates = append(templates, *template)
	}

	return templates, errors, nil
}

func handleFile(templateIndex map[string]*v3.Template, fullPath, relativePath, filename string) error {
	switch {
	case filename == "config.yml" || filename == "template.yml":
		base, templateName, parsedCorrectly := parse.TemplatePath(relativePath)
		if !parsedCorrectly {
			return nil
		}
		contents, err := ioutil.ReadFile(fullPath)
		if err != nil {
			return err
		}

		var template v3.Template
		if template, err = parse.TemplateInfo(contents); err != nil {
			return err
		}

		template.Spec.Base = base
		template.Spec.FolderName = templateName
		template.Spec.Categories = []string{template.Spec.Category}

		key := base + templateName

		if existingTemplate, ok := templateIndex[key]; ok {
			template.Spec.Icon = existingTemplate.Spec.Icon
			template.Spec.IconFilename = existingTemplate.Spec.IconFilename
			template.Spec.Readme = existingTemplate.Spec.Readme
			template.Spec.Versions = existingTemplate.Spec.Versions
		}
		templateIndex[key] = &template
	case strings.HasPrefix(filename, "catalogIcon") || strings.HasPrefix(filename, "icon"):
		base, templateName, parsedCorrectly := parse.TemplatePath(relativePath)
		if !parsedCorrectly {
			return nil
		}

		contents, err := ioutil.ReadFile(fullPath)
		if err != nil {
			return err
		}

		key := base + templateName

		if _, ok := templateIndex[key]; !ok {
			templateIndex[key] = &v3.Template{}
		}
		templateIndex[key].Spec.Icon = base64.StdEncoding.EncodeToString([]byte(contents))
		templateIndex[key].Spec.IconFilename = filename
	case strings.HasPrefix(strings.ToLower(filename), "readme.md"):
		base, templateName, parsedCorrectly := parse.TemplatePath(relativePath)
		if !parsedCorrectly {
			return nil
		}

		_, _, _, parsedCorrectly = parse.VersionPath(relativePath)
		if parsedCorrectly {
			return handleVersionFile(templateIndex, fullPath, relativePath, filename)
		}

		contents, err := ioutil.ReadFile(fullPath)
		if err != nil {
			return err
		}

		key := base + templateName

		if _, ok := templateIndex[key]; !ok {
			templateIndex[key] = &v3.Template{}
		}
		templateIndex[key].Spec.Readme = string(contents)
	default:
		return handleVersionFile(templateIndex, fullPath, relativePath, filename)
	}

	return nil
}

func handleVersionFile(templateIndex map[string]*v3.Template, fullPath, relativePath, filename string) error {
	base, templateName, folderName, parsedCorrectly := parse.VersionPath(relativePath)
	if !parsedCorrectly {
		return nil
	}

	contents, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return err
	}

	key := base + templateName
	file := v3.File{
		Name:     filename,
		Contents: string(contents),
	}

	if _, ok := templateIndex[key]; !ok {
		templateIndex[key] = &v3.Template{}
	}

	// Handle case where folder name is a revision (just a number)
	revision, err := strconv.Atoi(folderName)
	if err == nil {
		for i, version := range templateIndex[key].Spec.Versions {
			if version.Revision != nil && *version.Revision == revision {
				templateIndex[key].Spec.Versions[i].Files = append(version.Files, file)
				return nil
			}
		}
		templateIndex[key].Spec.Versions = append(templateIndex[key].Spec.Versions, v3.TemplateVersionSpec{
			Revision: &revision,
			Files:    []v3.File{file},
		})
		return nil
	}

	// Handle case where folder name is version (must be in semver format)
	_, err = semver.Parse(strings.Trim(folderName, "v"))
	if err == nil {
		for i, version := range templateIndex[key].Spec.Versions {
			if version.Version == folderName {
				templateIndex[key].Spec.Versions[i].Files = append(version.Files, file)
				return nil
			}
		}
		templateIndex[key].Spec.Versions = append(templateIndex[key].Spec.Versions, v3.TemplateVersionSpec{
			Version: folderName,
			Files:   []v3.File{file},
		})
		return nil
	}

	return nil
}
