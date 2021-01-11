package manager

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/blang/semver"
	mVersion "github.com/mcuadros/go-version"
	"github.com/pkg/errors"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	helmlib "github.com/rancher/rancher/pkg/catalog/helm"
	"github.com/rancher/rancher/pkg/catalog/utils"
	"github.com/rancher/rancher/pkg/controllers/managementuser/helm/common"
	managementv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	projectv3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/sirupsen/logrus"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Manager struct {
	catalogClient         v3.CatalogInterface
	CatalogLister         v3.CatalogLister
	clusterLister         v3.ClusterLister
	templateClient        v3.CatalogTemplateInterface
	templateContentClient v3.TemplateContentInterface
	templateVersionClient v3.CatalogTemplateVersionInterface
	templateLister        v3.CatalogTemplateLister
	templateVersionLister v3.CatalogTemplateVersionLister
	projectCatalogClient  v3.ProjectCatalogInterface
	ProjectCatalogLister  v3.ProjectCatalogLister
	clusterCatalogClient  v3.ClusterCatalogInterface
	ClusterCatalogLister  v3.ClusterCatalogLister
	appRevisionClient     projectv3.AppRevisionInterface
	lastUpdateTime        time.Time
	bundledMode           bool
}

type CatalogManager interface {
	ValidateChartCompatibility(template *v3.CatalogTemplateVersion, clusterName string) error
	ValidateKubeVersion(template *v3.CatalogTemplateVersion, clusterName string) error
	ValidateRancherVersion(template *v3.CatalogTemplateVersion) error
	LatestAvailableTemplateVersion(template *v3.CatalogTemplate, clusterName string) (*v32.TemplateVersionSpec, error)
	GetSystemAppCatalogID(templateVersionID, clusterName string) (string, error)
}

func New(management managementv3.Interface, project projectv3.Interface) *Manager {
	var bundledMode bool
	if strings.ToLower(settings.SystemCatalog.Get()) == "bundled" {
		bundledMode = true
	}
	return &Manager{
		catalogClient:         management.Catalogs(""),
		CatalogLister:         management.Catalogs("").Controller().Lister(),
		clusterLister:         management.Clusters("").Controller().Lister(),
		templateClient:        management.CatalogTemplates(""),
		templateContentClient: management.TemplateContents(""),
		templateVersionClient: management.CatalogTemplateVersions(""),
		templateLister:        management.CatalogTemplates("").Controller().Lister(),
		templateVersionLister: management.CatalogTemplateVersions("").Controller().Lister(),
		projectCatalogClient:  management.ProjectCatalogs(""),
		ProjectCatalogLister:  management.ProjectCatalogs("").Controller().Lister(),
		clusterCatalogClient:  management.ClusterCatalogs(""),
		ClusterCatalogLister:  management.ClusterCatalogs("").Controller().Lister(),
		appRevisionClient:     project.AppRevisions(""),
		bundledMode:           bundledMode,
	}
}

func (m *Manager) deleteChart(toDelete string, namespace string) error {
	toDeleteTvs, err := m.getTemplateVersion(toDelete, namespace)
	if err != nil {
		return err
	}
	for tv := range toDeleteTvs {
		if err := m.templateVersionClient.DeleteNamespaced(namespace, tv, &metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
			return err
		}
	}
	if err := m.templateClient.DeleteNamespaced(namespace, toDelete, &metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
		return err
	}
	return nil
}

func getKey(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}

func (m *Manager) DeleteOldTemplateContent() bool {
	// Template content is not used, remove old contents
	if err := m.templateContentClient.DeleteCollection(&metav1.DeleteOptions{}, metav1.ListOptions{}); err != nil {
		logrus.Warnf("Catalog-manager error deleting old template content: %s", err)
		return false
	}
	return true
}

func (m *Manager) DeleteBadCatalogTemplates() bool {
	// Orphaned catalog templates and template versions may exist, remove any where the catalog does not exist
	errs := m.deleteBadCatalogTemplates()
	if len(errs) == 0 {
		return true
	}
	for _, err := range errs {
		logrus.Errorf("Catalog-manager error deleting bad catalog templates: %s", err)
	}
	return false
}

func (m *Manager) deleteBadCatalogTemplates() []error {
	templates, err := m.templateClient.List(metav1.ListOptions{})
	if err != nil {
		return []error{err}
	}

	templateVersions, err := m.templateVersionClient.List(metav1.ListOptions{})
	if err != nil {
		return []error{err}
	}

	var hasCatalog = map[string]bool{}

	catalogs, err := m.catalogClient.List(metav1.ListOptions{})
	if err != nil {
		return []error{err}
	}
	for _, catalog := range catalogs.Items {
		hasCatalog[getKey(namespace.GlobalNamespace, catalog.Name)] = true
	}

	clusterCatalogs, err := m.clusterCatalogClient.List(metav1.ListOptions{})
	if err != nil {
		return []error{err}
	}
	for _, clusterCatalog := range clusterCatalogs.Items {
		hasCatalog[getKey(clusterCatalog.Namespace, clusterCatalog.Name)] = true
	}

	projectCatalogs, err := m.projectCatalogClient.List(metav1.ListOptions{})
	if err != nil {
		return []error{err}
	}
	for _, projectCatalog := range projectCatalogs.Items {
		hasCatalog[getKey(projectCatalog.Namespace, projectCatalog.Name)] = true
	}

	var (
		deleteCount int
		errs        []error
	)

	for _, template := range templates.Items {
		var catalogName string
		if template.Spec.CatalogID != "" {
			catalogName = template.Spec.CatalogID
		} else if template.Spec.ClusterCatalogID != "" {
			catalogName = template.Spec.ClusterCatalogID
		} else if template.Spec.ProjectCatalogID != "" {
			catalogName = template.Spec.ProjectCatalogID
		}

		ns, name := helmlib.SplitNamespaceAndName(catalogName)
		if ns == "" {
			ns = namespace.GlobalNamespace
		}
		if !hasCatalog[getKey(ns, name)] {
			if err := m.templateClient.DeleteNamespaced(template.Namespace, template.Name, &metav1.DeleteOptions{}); err != nil {
				errs = append(errs, err)
			}
			deleteCount++
		}
	}

	for _, templateVersion := range templateVersions.Items {
		ns, name, _, _, _, err := common.SplitExternalID(templateVersion.Spec.ExternalID)
		if err != nil {
			logrus.Errorf("Catalog-manager error extracting namespace/name from template version: %s", err)
			continue
		}
		if ns == "" {
			ns = namespace.GlobalNamespace
		}
		if !hasCatalog[getKey(ns, name)] {
			if err := m.templateVersionClient.DeleteNamespaced(templateVersion.Namespace, templateVersion.Name, &metav1.DeleteOptions{}); err != nil {
				errs = append(errs, err)
			}
			deleteCount++
		}
	}

	if deleteCount > 0 {
		logrus.Infof("Catalog-manager deleted %d orphaned catalog template entries", deleteCount)
	}

	return errs
}

func (m *Manager) ValidateChartCompatibility(template *v3.CatalogTemplateVersion, clusterName string) error {
	if err := m.ValidateRancherVersion(template); err != nil {
		return err
	}
	if err := m.ValidateKubeVersion(template, clusterName); err != nil {
		return err
	}
	return nil
}

func (m *Manager) ValidateKubeVersion(template *v3.CatalogTemplateVersion, clusterName string) error {
	if template.Spec.KubeVersion == "" {
		return nil
	}
	constraint, err := semver.ParseRange(template.Spec.KubeVersion)
	if err != nil {
		logrus.Errorf("failed to parse constraint for kubeversion %s: %v", template.Spec.KubeVersion, err)
		return nil
	}

	cluster, err := m.clusterLister.Get("", clusterName)
	if err != nil {
		return err
	}

	k8sVersion, err := semver.Parse(strings.TrimPrefix(cluster.Status.Version.String(), "v"))
	if err != nil {
		return err
	}
	if !constraint(k8sVersion) {
		return fmt.Errorf("incompatible kubernetes version [%s] for template [%s]", k8sVersion.String(), template.Name)
	}
	return nil
}

func (m *Manager) ValidateRancherVersion(template *v3.CatalogTemplateVersion) error {
	rancherMin := template.Spec.RancherMinVersion
	rancherMax := template.Spec.RancherMaxVersion

	serverVersion := settings.ServerVersion.Get()

	// don't compare if we are running as dev or in the build env
	if !utils.ReleaseServerVersion(serverVersion) {
		return nil
	}

	if rancherMin != "" && !mVersion.Compare(serverVersion, rancherMin, ">=") {
		return fmt.Errorf("rancher min version not met")
	}

	if rancherMax != "" && !mVersion.Compare(serverVersion, rancherMax, "<=") {
		return fmt.Errorf("rancher max version exceeded")
	}

	return nil
}

func (m *Manager) LatestAvailableTemplateVersion(template *v3.CatalogTemplate, clusterName string) (*v32.TemplateVersionSpec, error) {
	versions := template.DeepCopy().Spec.Versions
	if len(versions) == 0 {
		return nil, errors.New("empty catalog template version list")
	}

	sort.Slice(versions, func(i, j int) bool {
		val1, err := semver.ParseTolerant(versions[i].Version)
		if err != nil {
			return false
		}

		val2, err := semver.ParseTolerant(versions[j].Version)
		if err != nil {
			return false
		}

		return val2.LT(val1)
	})

	for _, templateVersion := range versions {
		catalogTemplateVersion := &v3.CatalogTemplateVersion{
			TemplateVersion: v3.TemplateVersion{
				Spec: templateVersion,
			},
		}

		if err := m.ValidateChartCompatibility(catalogTemplateVersion, clusterName); err == nil {
			return &templateVersion, nil
		}
	}

	return nil, errors.Errorf("template %s incompatible with rancher version or cluster's [%s] kubernetes version", template.Name, clusterName)
}

func (m *Manager) GetSystemAppCatalogID(templateVersionID, clusterName string) (string, error) {
	template, err := m.templateLister.Get(namespace.GlobalNamespace, templateVersionID)
	if err != nil {
		return "", errors.Wrapf(err, "failed to find template by ID %s", templateVersionID)
	}

	templateVersion, err := m.LatestAvailableTemplateVersion(template, clusterName)
	if err != nil {
		return "", err
	}
	return templateVersion.ExternalID, nil
}
