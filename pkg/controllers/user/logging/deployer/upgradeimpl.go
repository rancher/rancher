package deployer

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	versionutil "github.com/rancher/rancher/pkg/catalog/utils"
	"github.com/rancher/rancher/pkg/controllers/user/helm/common"
	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
	"github.com/rancher/rancher/pkg/project"
	appsv1 "github.com/rancher/types/apis/apps/v1"
	v1 "github.com/rancher/types/apis/core/v1"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	projectv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/config"

	"github.com/pkg/errors"
	"github.com/rancher/types/namespace"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	ServiceName             = "logging"
	waitCatalogSyncInterval = 60 * time.Second
)

type LoggingService struct {
	clusterName    string
	clusterLister  v3.ClusterLister
	catalogLister  v3.CatalogLister
	projectLister  v3.ProjectLister
	templateLister v3.CatalogTemplateLister
	daemonsets     appsv1.DaemonSetInterface
	secrets        v1.SecretInterface
	appDeployer    *AppDeployer
}

func NewService() *LoggingService {
	return &LoggingService{}
}

func (l *LoggingService) Init(cluster *config.UserContext) {
	ad := &AppDeployer{
		AppsGetter: cluster.Management.Project,
		AppsLister: cluster.Management.Project.Apps("").Controller().Lister(),
		Namespaces: cluster.Core.Namespaces(metav1.NamespaceAll),
	}

	l.clusterName = cluster.ClusterName
	l.clusterLister = cluster.Management.Management.Clusters("").Controller().Lister()
	l.catalogLister = cluster.Management.Management.Catalogs(metav1.NamespaceAll).Controller().Lister()
	l.projectLister = cluster.Management.Management.Projects(cluster.ClusterName).Controller().Lister()
	l.templateLister = cluster.Management.Management.CatalogTemplates(metav1.NamespaceAll).Controller().Lister()
	l.daemonsets = cluster.Apps.DaemonSets(loggingconfig.LoggingNamespace)
	l.secrets = cluster.Core.Secrets(loggingconfig.LoggingNamespace)
	l.appDeployer = ad
}

func (l *LoggingService) Version() (string, error) {
	return loggingconfig.RancherLoggingInitVersion(), nil
}

func (l *LoggingService) Upgrade(currentVersion string) (string, error) {
	appName := loggingconfig.AppName
	templateID := loggingconfig.RancherLoggingTemplateID()
	template, err := l.templateLister.Get(namespace.GlobalNamespace, templateID)
	if err != nil {
		return "", errors.Wrapf(err, "get template %s failed", templateID)
	}

	templateVersion, err := versionutil.LatestAvailableTemplateVersion(template)
	if err != nil {
		return "", err
	}

	newFullVersion := fmt.Sprintf("%s-%s", templateID, templateVersion.Version)
	if currentVersion == newFullVersion {
		return currentVersion, nil
	}

	// check cluster ready before upgrade, because helm will not retry if got cluster not ready error
	cluster, err := l.clusterLister.Get(metav1.NamespaceAll, l.clusterName)
	if err != nil {
		return "", fmt.Errorf("get cluster %s failed, %v", l.clusterName, err)
	}
	if !v3.ClusterConditionReady.IsTrue(cluster) {
		return "", fmt.Errorf("cluster %v not ready", l.clusterName)
	}

	//clean old version
	if !strings.Contains(currentVersion, templateID) {
		if err = l.removeLegacy(); err != nil {
			return "", err
		}
	}

	//upgrade old app
	defaultSystemProjects, err := l.projectLister.List(metav1.NamespaceAll, labels.Set(project.SystemProjectLabel).AsSelector())
	if err != nil {
		return "", errors.Wrap(err, "list system project failed")
	}

	if len(defaultSystemProjects) == 0 {
		return "", errors.New("get system project failed")
	}

	systemProject := defaultSystemProjects[0]
	if systemProject == nil {
		return "", errors.New("get system project failed")
	}

	app, err := l.appDeployer.AppsLister.Get(systemProject.Name, appName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return newFullVersion, nil
		}
		return "", errors.Wrapf(err, "get app %s:%s failed", systemProject.Name, appName)
	}

	_, systemCatalogName, _, _, _, err := common.SplitExternalID(templateVersion.ExternalID)
	if err != nil {
		return "", err
	}

	systemCatalog, err := l.catalogLister.Get(metav1.NamespaceAll, systemCatalogName)
	if err != nil {
		return "", fmt.Errorf("get catalog %s failed, %v", systemCatalogName, err)
	}

	if !v3.CatalogConditionUpgraded.IsTrue(systemCatalog) || !v3.CatalogConditionRefreshed.IsTrue(systemCatalog) || !v3.CatalogConditionDiskCached.IsTrue(systemCatalog) {
		return "", fmt.Errorf("catalog %v not ready", systemCatalogName)
	}

	newApp := app.DeepCopy()
	newApp.Spec.ExternalID = templateVersion.ExternalID

	if !reflect.DeepEqual(newApp, app) {
		// add force upgrade to handle chart compatibility in different version
		projectv3.AppConditionForceUpgrade.Unknown(newApp)

		if _, err = l.appDeployer.AppsGetter.Apps(metav1.NamespaceAll).Update(newApp); err != nil {
			return "", errors.Wrapf(err, "update app %s:%s failed", app.Namespace, app.Name)
		}
	}
	return newFullVersion, nil
}

func (l *LoggingService) removeLegacy() error {
	op := metav1.DeletePropagationBackground
	errMsg := "failed to remove legacy logging %s %s:%s when upgrade"

	if err := l.daemonsets.Delete(loggingconfig.FluentdName, &metav1.DeleteOptions{PropagationPolicy: &op}); err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrapf(err, errMsg, loggingconfig.LoggingNamespace, "daemonset", loggingconfig.FluentdName)
	}

	if err := l.daemonsets.Delete(loggingconfig.LogAggregatorName, &metav1.DeleteOptions{PropagationPolicy: &op}); err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrapf(err, errMsg, loggingconfig.LoggingNamespace, "daemonset", loggingconfig.LogAggregatorName)
	}

	legacySSlConfigName := "sslconfig"
	legacyClusterConfigName := "cluster-logging"
	legacyProjectConfigName := "project-logging"

	if err := l.secrets.Delete(legacySSlConfigName, &metav1.DeleteOptions{PropagationPolicy: &op}); err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrapf(err, errMsg, "serect", loggingconfig.LoggingNamespace, legacySSlConfigName)
	}

	if err := l.secrets.Delete(legacyClusterConfigName, &metav1.DeleteOptions{PropagationPolicy: &op}); err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrapf(err, errMsg, "serect", loggingconfig.LoggingNamespace, legacyClusterConfigName)
	}

	if err := l.secrets.Delete(legacyProjectConfigName, &metav1.DeleteOptions{PropagationPolicy: &op}); err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrapf(err, errMsg, "serect", loggingconfig.LoggingNamespace, legacyProjectConfigName)
	}
	return nil
}
