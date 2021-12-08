package manager

import (
	"fmt"
	"reflect"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/catalog/utils"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
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

func (m *Manager) createTemplate(template v3.CatalogTemplate, catalog *v3.Catalog) error {
	template.Labels = labels.Merge(template.Labels, map[string]string{
		CatalogNameLabel: catalog.Name,
	})
	versionFiles := make([]v32.TemplateVersionSpec, len(template.Spec.Versions))
	copy(versionFiles, template.Spec.Versions)
	for i := range template.Spec.Versions {
		template.Spec.Versions[i].Files = nil
		template.Spec.Versions[i].Readme = ""
		template.Spec.Versions[i].AppReadme = ""
	}
	logrus.Debugf("Creating template %s", template.Name)
	createdTemplate, err := m.templateClient.Create(&template)
	if err != nil {
		return errors.Wrapf(err, "failed to create template %s", template.Name)
	}
	return m.createTemplateVersions(catalog.Name, versionFiles, createdTemplate)
}

func (m *Manager) getTemplateMap(catalogName string, namespace string) (map[string]*v3.CatalogTemplate, error) {
	r, _ := labels.NewRequirement(CatalogNameLabel, selection.Equals, []string{catalogName})
	templateList, err := m.templateLister.List(namespace, labels.NewSelector().Add(*r))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list templates for %v", catalogName)
	}
	templateMap := make(map[string]*v3.CatalogTemplate)
	for _, t := range templateList {
		templateMap[t.Name] = t
	}
	return templateMap, nil
}

func (m *Manager) updateTemplate(template *v3.CatalogTemplate, toUpdate v3.CatalogTemplate) error {
	r, err := labels.NewRequirement(TemplateNameLabel, selection.Equals, []string{template.Name})
	if err != nil {
		return errors.Wrapf(err, "failed to find template version with label %v for %v", TemplateNameLabel, template.Name)
	}
	templateVersions, err := m.templateVersionLister.List(template.Namespace, labels.NewSelector().Add(*r))
	if err != nil {
		return errors.Wrapf(err, "failed to list templateVersions")
	}
	tvByVersion := map[string]*v3.CatalogTemplateVersion{}
	for _, ver := range templateVersions {
		tvByVersion[ver.Spec.Version] = ver
	}
	/*
		For each templateVersion in toUpdate, if spec doesn't match, do update
		For version that doesn't exist, create a new one
	*/
	for _, toUpdateVer := range toUpdate.Spec.Versions {
		templateVersion := &v3.CatalogTemplateVersion{}
		templateVersion.Spec = toUpdateVer
		if tv, ok := tvByVersion[toUpdateVer.Version]; ok {
			if !reflect.DeepEqual(tv.Spec, toUpdateVer) {
				logrus.Debugf("Updating templateVersion %v", tv.Name)
				newObject := tv.DeepCopy()
				newObject.Spec = templateVersion.Spec
				if _, err := m.templateVersionClient.Update(newObject); err != nil {
					return err
				}
			}
		} else {
			toCreate := &v3.CatalogTemplateVersion{}
			toCreate.Name = fmt.Sprintf("%s-%v", template.Name, toUpdateVer.Version)
			toCreate.Namespace = template.Namespace
			toCreate.Labels = map[string]string{
				TemplateNameLabel: template.Name,
			}
			toCreate.Spec = templateVersion.Spec
			toCreate.Status = v32.TemplateVersionStatus{HelmVersion: template.Status.HelmVersion}
			logrus.Debugf("Creating templateVersion %v", toCreate.Name)
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
			if err := m.templateVersionClient.DeleteNamespaced(template.Namespace, tv.Name, &metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
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

func (m *Manager) getTemplateVersion(templateName string, namespace string) (map[string]struct{}, error) {
	//because templates is a cluster resource now so we set namespace to "" when listing it.
	r, err := labels.NewRequirement(TemplateNameLabel, selection.Equals, []string{templateName})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find template version with label %v for %v", TemplateNameLabel, templateName)
	}
	templateVersions, err := m.templateVersionLister.List(namespace, labels.NewSelector().Add(*r))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list template version(s) for %v: ", templateName)
	}
	tVersion := map[string]struct{}{}
	for _, ver := range templateVersions {
		tVersion[ver.Name] = struct{}{}
	}
	return tVersion, nil
}

func (m *Manager) createTemplateVersions(catalogName string, versionsSpec []v32.TemplateVersionSpec, template *v3.CatalogTemplate) error {
	for _, spec := range versionsSpec {
		templateVersion := &v3.CatalogTemplateVersion{}
		templateVersion.Spec = spec
		templateVersion.Status = v32.TemplateVersionStatus{HelmVersion: template.Status.HelmVersion}
		templateVersion.Name = getValidTemplateNameWithVersion(template.Name, spec.Version)
		templateVersion.Namespace = template.Namespace
		templateVersion.Labels = map[string]string{
			TemplateNameLabel: template.Name,
		}
		//help with garbage collection on delete
		ownerRef := []metav1.OwnerReference{{
			Name:       template.Name,
			APIVersion: "management.cattle.io/v3",
			UID:        template.UID,
			Kind:       template.Kind,
		}}
		templateVersion.OwnerReferences = ownerRef

		logrus.Debugf("Creating templateVersion %s", templateVersion.Name)
		if _, err := m.templateVersionClient.Create(templateVersion); err != nil && !kerrors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func showUpgradeLinks(version, upgradeVersion string) bool {
	if !utils.VersionGreaterThan(upgradeVersion, version) {
		return false
	}
	return true
}
