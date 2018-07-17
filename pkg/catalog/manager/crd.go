package manager

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"crypto/sha256"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/catalog/utils"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

const (
	CatalogNameLabel  = "catalog.cattle.io/name"
	TemplateNameLabel = "catalog.cattle.io/template_name"
)

func (m *Manager) createTemplate(template v3.Template, catalog *v3.Catalog, tagMap map[string]struct{}) error {
	template.Labels = labels.Merge(template.Labels, map[string]string{
		CatalogNameLabel: catalog.Name,
	})
	versionFiles := make([]v3.TemplateVersionSpec, len(template.Spec.Versions))
	copy(versionFiles, template.Spec.Versions)
	for i := range template.Spec.Versions {
		template.Spec.Versions[i].Files = nil
		template.Spec.Versions[i].Readme = ""
		template.Spec.Versions[i].AppReadme = ""
	}
	if err := m.convertTemplateIcon(&template, tagMap); err != nil {
		return err
	}
	logrus.Infof("Creating template %s", template.Name)
	createdTemplate, err := m.templateClient.Create(&template)
	if err != nil {
		return errors.Wrapf(err, "failed to create template %s", template.Name)
	}
	return m.createTemplateVersions(versionFiles, createdTemplate, tagMap)
}

func (m *Manager) getTemplateMap(catalogName string) (map[string]*v3.Template, error) {
	r, _ := labels.NewRequirement(CatalogNameLabel, selection.Equals, []string{catalogName})
	templateList, err := m.templateLister.List("", labels.NewSelector().Add(*r))
	if err != nil {
		return nil, err
	}
	templateMap := map[string]*v3.Template{}
	for _, t := range templateList {
		templateMap[t.Name] = t
	}
	return templateMap, nil
}

func (m *Manager) convertTemplateIcon(template *v3.Template, tagMap map[string]struct{}) error {
	tag, content, err := zipAndHash(template.Spec.Icon)
	if err != nil {
		return err
	}
	if _, ok := tagMap[tag]; !ok {
		templateContent := &v3.TemplateContent{}
		templateContent.Name = tag
		templateContent.Data = content
		if _, err := m.templateContentClient.Create(templateContent); err != nil {
			return err
		}
		tagMap[tag] = struct{}{}
	}
	template.Spec.Icon = tag
	return nil
}

func (m *Manager) updateTemplate(template *v3.Template, toUpdate v3.Template, tagMap map[string]struct{}) error {
	r, _ := labels.NewRequirement(TemplateNameLabel, selection.Equals, []string{template.Name})
	templateVersions, err := m.templateVersionLister.List("", labels.NewSelector().Add(*r))
	if err != nil {
		return errors.Wrapf(err, "failed to list templateVersions")
	}
	tvByVersion := map[string]*v3.TemplateVersion{}
	for _, ver := range templateVersions {
		tvByVersion[ver.Spec.Version] = ver
	}
	/*
		for each templateVersion in toUpdate, calculate each hash value and if doesn't match, do update.
		For version that doesn't exist, create a new one
	*/
	for _, toUpdateVer := range toUpdate.Spec.Versions {
		// gzip each file to store the hash value into etcd. Next time if it already exists in etcd then use the existing tag
		templateVersion := &v3.TemplateVersion{}
		templateVersion.Spec = toUpdateVer
		if err := m.convertFile(templateVersion, toUpdateVer, tagMap); err != nil {
			return err
		}
		if tv, ok := tvByVersion[toUpdateVer.Version]; ok {
			if tv.Spec.Digest != toUpdateVer.Digest {
				logrus.Infof("Updating templateVersion %v", tv.Name)
				newObject := tv.DeepCopy()
				newObject.Spec = templateVersion.Spec
				if _, err := m.templateVersionClient.Update(newObject); err != nil {
					return err
				}
			}
		} else {
			toCreate := &v3.TemplateVersion{}
			toCreate.Name = fmt.Sprintf("%s-%v", template.Name, toUpdateVer.Version)
			toCreate.Labels = map[string]string{
				TemplateNameLabel: template.Name,
			}
			toCreate.Spec = templateVersion.Spec
			logrus.Infof("Creating templateVersion %v", toCreate.Name)
			if _, err := m.templateVersionClient.Create(toCreate); err != nil {
				return err
			}
		}
	}

	// find existing templateVersion that is not in toUpdate.Versions
	toUpdateTvs := map[string]struct{}{}
	for _, toUpdateVer := range toUpdate.Spec.Versions {
		toUpdateTvs[toUpdateVer.Version] = struct{}{}
	}
	for v, tv := range tvByVersion {
		if _, ok := toUpdateTvs[v]; !ok {
			logrus.Infof("Deleting templateVersion %s", tv.Name)
			if err := m.templateVersionClient.Delete(tv.Name, &metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
				return err
			}
		}
	}

	for i := range toUpdate.Spec.Versions {
		toUpdate.Spec.Versions[i].Files = nil
		toUpdate.Spec.Versions[i].Readme = ""
		toUpdate.Spec.Versions[i].AppReadme = ""
	}
	newObj := template.DeepCopy()
	newObj.Spec = toUpdate.Spec
	newObj.Labels = mergeLabels(template.Labels, toUpdate.Labels)
	if err := m.convertTemplateIcon(newObj, tagMap); err != nil {
		return err
	}
	if _, err := m.templateClient.Update(newObj); err != nil {
		return err
	}
	return nil
}

// merge any label from set2 into set1 and delete label
func mergeLabels(set1, set2 map[string]string) map[string]string {
	if set1 == nil {
		set1 = map[string]string{}
	}
	for k, v := range set2 {
		set1[k] = v
	}
	for k := range set1 {
		if set2 != nil {
			if _, ok := set2[k]; !ok && k != CatalogNameLabel {
				delete(set1, k)
			}
		} else {
			if k != CatalogNameLabel {
				delete(set1, k)
			}
		}

	}
	return set1
}

func (m *Manager) getTemplateVersion(templateName string) (map[string]struct{}, error) {
	//because templates is a cluster resource now so we set namespace to "" when listing it.
	r, _ := labels.NewRequirement(TemplateNameLabel, selection.Equals, []string{templateName})
	templateVersions, err := m.templateVersionLister.List("", labels.NewSelector().Add(*r))
	if err != nil {
		return nil, err
	}
	tVersion := map[string]struct{}{}
	for _, ver := range templateVersions {
		tVersion[ver.Name] = struct{}{}
	}
	return tVersion, nil
}

func (m *Manager) createTemplateVersions(versionsSpec []v3.TemplateVersionSpec, template *v3.Template, tagMap map[string]struct{}) error {
	for _, spec := range versionsSpec {
		templateVersion := &v3.TemplateVersion{}
		templateVersion.Spec = spec
		templateVersion.Name = fmt.Sprintf("%s-%v", template.Name, spec.Version)
		templateVersion.Labels = map[string]string{
			TemplateNameLabel: template.Name,
		}
		// gzip each file to store the hash value into etcd. Next time if it already exists in etcd then use the existing tag
		if err := m.convertFile(templateVersion, spec, tagMap); err != nil {
			return err
		}

		logrus.Debugf("Creating templateVersion %s", templateVersion.Name)
		if _, err := m.templateVersionClient.Create(templateVersion); err != nil && !kerrors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func (m *Manager) convertFile(templateVersion *v3.TemplateVersion, spec v3.TemplateVersionSpec, tagMap map[string]struct{}) error {
	for name, file := range spec.Files {
		tag, content, err := zipAndHash(file)
		if err != nil {
			return err
		}
		if _, ok := tagMap[tag]; !ok {
			templateContent := &v3.TemplateContent{}
			templateContent.Name = tag
			templateContent.Data = content
			if _, err := m.templateContentClient.Create(templateContent); err != nil {
				return err
			}
			tagMap[tag] = struct{}{}
		}
		templateVersion.Spec.Files[name] = tag
		if file == spec.Readme {
			templateVersion.Spec.Readme = tag
		}
		if file == spec.AppReadme {
			templateVersion.Spec.AppReadme = tag
		}
	}
	return nil
}

func zipAndHash(content string) (string, string, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(content)); err != nil {
		return "", "", err
	}
	zw.Close()
	digest := sha256.New()
	compressedData := buf.Bytes()
	digest.Write(compressedData)
	tag := hex.EncodeToString(digest.Sum(nil))
	return tag, base64.StdEncoding.EncodeToString(compressedData), nil
}

func showUpgradeLinks(version, upgradeVersion string) bool {
	if !utils.VersionGreaterThan(upgradeVersion, version) {
		return false
	}
	return true
}
