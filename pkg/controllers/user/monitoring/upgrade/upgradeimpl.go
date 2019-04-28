package upgrade

import (
	"context"
	"reflect"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/controllers/user/systemimage"
	"github.com/rancher/rancher/pkg/monitoring"
	"github.com/rancher/rancher/pkg/project"
	"github.com/rancher/rancher/pkg/settings"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	projectv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/config"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
)

type monitoringSystemService struct {
	clusterName   string
	appLister     projectv3.AppLister
	clusterLister mgmtv3.ClusterLister
	projectLister mgmtv3.ProjectLister
	appClient     projectv3.AppInterface
}

func init() {
	systemimage.RegisterSystemService("monitoring", &monitoringSystemService{})
}

func (s *monitoringSystemService) Init(ctx context.Context, cluster *config.UserContext) {
	s.clusterName = cluster.ClusterName
	s.appLister = cluster.Management.Project.Apps("").Controller().Lister()
	s.clusterLister = cluster.Management.Management.Clusters("").Controller().Lister()
	s.projectLister = cluster.Management.Management.Projects("").Controller().Lister()
	s.appClient = cluster.Management.Project.Apps("")
}

func (s *monitoringSystemService) Upgrade(currentVersion string) (newVersion string, err error) {
	latestVersion, err := s.Version()
	if err != nil {
		return "", err
	}
	// Already implement cluster monitoring upgrade and operator upgrade logic
	// but not using them yet
	if s.upgradeProjectMonitorings(latestVersion); err != nil {
		return "", err
	}
	return latestVersion, nil
}

func (s *monitoringSystemService) Version() (string, error) {
	return settings.SystemMonitoringCatalogID.Get(), nil
}

func (s *monitoringSystemService) upgradeClusterMonitoring(version string) error {
	cluster, err := s.clusterLister.Get("", s.clusterName)
	if err != nil {
		return errors.Wrap(err, "failed to list cluster in monitoring upgrade service")
	}

	if !cluster.Spec.EnableClusterMonitoring {
		return nil
	}

	p, err := project.GetSystemProject(s.clusterName, s.projectLister)
	if err != nil {
		return errors.Wrap(err, "failed to list system in monitoring upgrade service")
	}

	appName, _ := monitoring.ClusterMonitoringInfo()
	app, err := s.appLister.Get(p.Name, appName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "failed to get app %s in monitoring upgrade service", app.Name)
	}

	return s.upgradeAppCatalogAndAnswers(app, version, nil)
}

func (s *monitoringSystemService) upgradeProjectMonitorings(version string) error {
	projects, err := s.projectLister.List(s.clusterName, labels.Everything())
	if err != nil {
		return errors.Wrapf(err, "failed to list projects of cluster %s in monitoring upgrade service", s.clusterName)
	}
	cluster, err := s.clusterLister.Get("", s.clusterName)
	appName, _ := monitoring.ProjectMonitoringInfo("")
	for _, p := range projects {
		if !p.Spec.EnableProjectMonitoring {
			continue
		}
		app, err := s.appLister.Get(p.Name, appName)
		if err != nil && !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get app %s in monitoring upgrade service", app.Name)
		}

		if apierrors.IsNotFound(err) {
			continue
		}

		answers := monitoring.GetProjectMonitoringAnswers(p, cluster.Spec.DisplayName)

		if err := s.upgradeAppCatalogAndAnswers(app, version, answers); err != nil {
			return errors.Wrapf(err, "failed to upgrade app %s/%s in monitoring upgrade service", app.Namespace, app.Name)
		}
	}
	return nil
}

func (s *monitoringSystemService) upgradePrometheusOperator(version string) error {
	cluster, err := s.clusterLister.Get("", s.clusterName)
	if err != nil {
		return errors.Wrap(err, "failed to list cluster in monitoring upgrade service")
	}
	clusterEnabled := cluster.Spec.EnableClusterAlerting || cluster.Spec.EnableClusterMonitoring
	projects, err := s.projectLister.List(s.clusterName, labels.Everything())
	if err != nil {
		return errors.Wrapf(err, "failed to list projects of cluster %s in monitoring upgrade service", s.clusterName)
	}
	projectEnabled := false
	for _, p := range projects {
		projectEnabled = projectEnabled || p.Spec.EnableProjectMonitoring
	}

	if !clusterEnabled && !projectEnabled {
		return nil
	}

	p, err := project.GetSystemProject(s.clusterName, s.projectLister)
	if err != nil {
		return errors.Wrap(err, "failed to list system in monitoring upgrade service")
	}

	operatorAppName, _ := monitoring.SystemMonitoringInfo()
	app, err := s.appLister.Get(p.Name, operatorAppName)
	if err != nil {
		return errors.Wrapf(err, "failed to get operator app %s in monitoring upgrade service", operatorAppName)
	}

	return s.upgradeAppCatalogAndAnswers(app, version, nil)
}

func (s *monitoringSystemService) upgradeAppCatalogAndAnswers(app *projectv3.App, catalogID string, answers map[string]string) error {
	if app.Spec.ExternalID == catalogID && reflect.DeepEqual(answers, app.Spec.Answers) {
		return nil
	}

	newApp := app.DeepCopy()
	newApp.Spec.ExternalID = catalogID
	if answers != nil {
		newApp.Spec.Answers = answers
	}
	_, err := s.appClient.Update(newApp)
	if err != nil {
		return errors.Wrapf(err, "failed to update app %s/%s in monitoring upgrade service", app.Namespace, app.Name)
	}
	return nil
}
