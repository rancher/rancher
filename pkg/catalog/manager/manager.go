package manager

import (
	"fmt"
	"strings"
	"time"

	helmlib "github.com/rancher/rancher/pkg/catalog/helm"
	"github.com/rancher/rancher/pkg/controllers/managementuser/helm/common"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	projectv3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/types/namespace"
	"github.com/sirupsen/logrus"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Manager struct {
	catalogClient         v3.CatalogInterface
	CatalogLister         v3.CatalogLister
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

func New(management *config.ManagementContext) *Manager {
	var bundledMode bool
	if strings.ToLower(settings.SystemCatalog.Get()) == "bundled" {
		bundledMode = true
	}
	return &Manager{
		catalogClient:         management.Management.Catalogs(""),
		CatalogLister:         management.Management.Catalogs("").Controller().Lister(),
		templateClient:        management.Management.CatalogTemplates(""),
		templateContentClient: management.Management.TemplateContents(""),
		templateVersionClient: management.Management.CatalogTemplateVersions(""),
		templateLister:        management.Management.CatalogTemplates("").Controller().Lister(),
		templateVersionLister: management.Management.CatalogTemplateVersions("").Controller().Lister(),
		projectCatalogClient:  management.Management.ProjectCatalogs(""),
		ProjectCatalogLister:  management.Management.ProjectCatalogs("").Controller().Lister(),
		clusterCatalogClient:  management.Management.ClusterCatalogs(""),
		ClusterCatalogLister:  management.Management.ClusterCatalogs("").Controller().Lister(),
		appRevisionClient:     management.Project.AppRevisions(""),
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
