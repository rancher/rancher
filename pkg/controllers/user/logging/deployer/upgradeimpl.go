package deployer

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
	"github.com/rancher/rancher/pkg/controllers/user/systemimage"
	"github.com/rancher/rancher/pkg/project"
	appsv1beta2 "github.com/rancher/types/apis/apps/v1beta2"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/namespace"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	serviceName = "logging"
)

type loggingService struct {
	clusterName    string
	projectLister  v3.ProjectLister
	templateLister v3.CatalogTemplateLister
	daemonsets     appsv1beta2.DaemonSetInterface
	secrets        v1.SecretInterface
	appDeployer    *AppDeployer
}

func init() {
	systemimage.RegisterSystemService(serviceName, &loggingService{})
}

func (l *loggingService) Init(ctx context.Context, cluster *config.UserContext) {
	ad := &AppDeployer{
		AppsGetter: cluster.Management.Project,
		Namespaces: cluster.Core.Namespaces(metav1.NamespaceAll),
	}

	l.clusterName = cluster.ClusterName
	l.projectLister = cluster.Management.Management.Projects(metav1.NamespaceAll).Controller().Lister()
	l.templateLister = cluster.Management.Management.CatalogTemplates(metav1.NamespaceAll).Controller().Lister()
	l.daemonsets = cluster.Apps.DaemonSets(loggingconfig.LoggingNamespace)
	l.secrets = cluster.Core.Secrets(loggingconfig.LoggingNamespace)
	l.appDeployer = ad
}

func (l *loggingService) Version() (string, error) {
	return loggingconfig.AppInitVersion, nil
}

func (l *loggingService) Upgrade(currentVersion string) (string, error) {
	appName := loggingconfig.AppName
	templateID := loggingconfig.RancherLoggingTemplateID()
	template, err := l.templateLister.Get(namespace.GlobalNamespace, templateID)
	if err != nil {
		return "", errors.Wrapf(err, "get template %s failed", templateID)
	}

	newFullVersion := fmt.Sprintf("%s-%s", templateID, template.Spec.DefaultVersion)
	if currentVersion == newFullVersion {
		return currentVersion, nil
	}

	//clean old version
	if !strings.Contains(currentVersion, templateID) {
		if err = l.removeLegacy(); err != nil {
			return "", err
		}
	}

	//upgrade old app
	newVersion := template.Spec.DefaultVersion
	newCatalogID := loggingconfig.RancherLoggingCatalogID(newVersion)
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

	app, err := l.appDeployer.AppsGetter.Apps(metav1.NamespaceAll).GetNamespaced(systemProject.Name, appName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return newFullVersion, nil
		}
		return "", errors.Wrapf(err, "get app %s:%s failed", systemProject.Name, appName)
	}
	newApp := app.DeepCopy()
	newApp.Spec.ExternalID = newCatalogID

	if !reflect.DeepEqual(newApp, app) {
		if _, err = l.appDeployer.AppsGetter.Apps(metav1.NamespaceAll).Update(newApp); err != nil {
			return "", errors.Wrapf(err, "update app %s:%s failed", app.Namespace, app.Name)
		}
	}
	return newFullVersion, nil
}

func (l *loggingService) removeLegacy() error {
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
