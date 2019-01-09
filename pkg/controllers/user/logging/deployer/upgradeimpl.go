package deployer

import (
	"context"
	"fmt"
	"reflect"

	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
	"github.com/rancher/rancher/pkg/controllers/user/systemimage"
	"github.com/rancher/rancher/pkg/project"
	appsv1beta2 "github.com/rancher/types/apis/apps/v1beta2"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"

	"github.com/pkg/errors"
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
	templateLister v3.TemplateLister
	daemonsets     appsv1beta2.DaemonSetInterface
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
	l.templateLister = cluster.Management.Management.Templates(metav1.NamespaceAll).Controller().Lister()
	l.daemonsets = cluster.Apps.DaemonSets(metav1.NamespaceAll)
	l.appDeployer = ad
}

func (l *loggingService) Version() (string, error) {
	return loggingconfig.AppInitVersion, nil
}

func (l *loggingService) Upgrade(currentVersion string) (string, error) {
	appName := loggingconfig.AppName
	templateID := loggingconfig.RancherLoggingTemplateID()
	template, err := l.templateLister.Get(metav1.NamespaceAll, templateID)
	if err != nil {
		return "", errors.Wrapf(err, "get template %s failed", templateID)
	}

	newFullVersion := fmt.Sprintf("%s-%s", templateID, template.Spec.DefaultVersion)
	if currentVersion == newFullVersion {
		return currentVersion, nil
	}

	newVersion := template.Spec.DefaultVersion

	//clean old version
	if err = l.daemonsets.Delete(loggingconfig.FluentdName, &metav1.DeleteOptions{}); !apierrors.IsNotFound(err) {
		return "", nil
	}

	if err = l.daemonsets.Delete(loggingconfig.LogAggregatorName, &metav1.DeleteOptions{}); !apierrors.IsNotFound(err) {
		return "", nil
	}

	//upgrade old app
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
			return newVersion, nil
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
	return newVersion, nil
}
