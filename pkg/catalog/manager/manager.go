package manager

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/catalog/utils"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/helm/common"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	projectv3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	helmlib "github.com/rancher/rancher/pkg/helm"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/sirupsen/logrus"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IncompatibleTemplateVersionErr is returned when there's no valid template version for the current Kubernetes cluster
type IncompatibleTemplateVersionErr error

type Manager struct {
	catalogClient         v3.CatalogInterface
	CatalogLister         v3.CatalogLister
	ClusterLister         v3.ClusterLister
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
	ConfigMap             corev1.ConfigMapInterface
	ConfigMapLister       corev1.ConfigMapLister
	SecretLister          corev1.SecretLister
}

type CatalogManager interface {
	ValidateChartCompatibility(template *v3.CatalogTemplateVersion, clusterName, currentAppVersion string) error
	ValidateKubeVersion(template *v3.CatalogTemplateVersion, clusterName string) error
	ValidateRancherVersion(template *v3.CatalogTemplateVersion, currentAppVersion string) error
	LatestAvailableTemplateVersion(template *v3.CatalogTemplate, clusterName string) (*v32.TemplateVersionSpec, error)
	GetSystemAppCatalogID(templateVersionID, clusterName string) (string, error)
}

func New(management v3.Interface, project projectv3.Interface, core corev1.Interface) *Manager {
	var bundledMode bool
	if strings.ToLower(settings.SystemCatalog.Get()) == "bundled" {
		bundledMode = true
	}
	return &Manager{
		catalogClient:         management.Catalogs(""),
		CatalogLister:         management.Catalogs("").Controller().Lister(),
		ClusterLister:         management.Clusters("").Controller().Lister(),
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
		ConfigMap:             core.ConfigMaps(""),
		ConfigMapLister:       core.ConfigMaps("").Controller().Lister(),
		SecretLister:          core.Secrets("").Controller().Lister(),
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
	// Orphaned catalog templates and template versions may exist, remove anywhere the catalog does not exist
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

func (m *Manager) ValidateChartCompatibility(template *v3.CatalogTemplateVersion, clusterName, currentAppVersion string) error {
	if err := m.ValidateRancherVersion(template, currentAppVersion); err != nil {
		return err
	}
	return m.ValidateKubeVersion(template, clusterName)
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

	cluster, err := m.ClusterLister.Get("", clusterName)
	if err != nil {
		return err
	}

	if cluster.Status.Version == nil {
		return fmt.Errorf("cluster [%s] status version is not available yet. Cannot validate kube version for template [%s]", clusterName, template.Name)
	}

	k8sVersion, err := semver.Parse(strings.TrimPrefix(cluster.Status.Version.String(), "v"))
	if err != nil {
		return err
	}
	if !constraint(k8sVersion) {
		return IncompatibleTemplateVersionErr(fmt.Errorf("incompatible kubernetes version [%s] for template [%s]", k8sVersion.String(), template.Name))
	}
	return nil
}

func (m *Manager) ValidateRancherVersion(template *v3.CatalogTemplateVersion, currentAppVersion string) error {
	if currentAppVersion != "" && currentAppVersion == template.Spec.Version {
		// if current app version is provided and the version in the update is equal to it then the
		// version is deemed okay as it is already installed. This ensures the app can continue to
		// be edited as long as it is not being upgraded/rollbacked to another incompatible version.
		return nil
	}

	serverVersion := settings.ServerVersion.Get()

	// don't compare if we are running as dev or in the build env
	if !utils.ReleaseServerVersion(serverVersion) {
		return nil
	}

	serverVersion = strings.TrimPrefix(serverVersion, "v")

	versionRange := ""
	if template.Spec.RancherMinVersion != "" {
		versionRange += " >=" + strings.TrimPrefix(template.Spec.RancherMinVersion, "v")
	}
	if template.Spec.RancherMaxVersion != "" {
		versionRange += " <=" + strings.TrimPrefix(template.Spec.RancherMaxVersion, "v")
	}
	if versionRange == "" {
		return nil
	}
	constraint, err := semver.ParseRange(versionRange)
	if err != nil {
		logrus.Errorf("failed to parse constraint for rancher version %s: %v", versionRange, err)
		return nil
	}

	rancherVersion, err := semver.Parse(serverVersion)
	if err != nil {
		return err
	}
	if !constraint(rancherVersion) {
		return IncompatibleTemplateVersionErr(fmt.Errorf("incompatible rancher version [%s] for template [%s]", serverVersion, template.Name))
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
			Spec: templateVersion,
		}

		if err := m.ValidateChartCompatibility(catalogTemplateVersion, clusterName, ""); err == nil {
			return &templateVersion, nil
		}
	}

	return nil, IncompatibleTemplateVersionErr(errors.Errorf("template %s incompatible with rancher version or cluster's [%s] kubernetes version", template.Name, clusterName))
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
