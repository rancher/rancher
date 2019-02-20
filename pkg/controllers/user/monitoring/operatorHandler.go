package monitoring

import (
	"reflect"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/monitoring"
	"github.com/rancher/rancher/pkg/settings"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	projectv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type operatorHandler struct {
	clusterName         string
	cattleClusterClient mgmtv3.ClusterInterface
	app                 *appHandler
}

func (h *operatorHandler) syncCluster(key string, obj *mgmtv3.Cluster) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil || obj.Name != h.clusterName {
		return obj, nil
	}
	var newCluster *mgmtv3.Cluster
	var err error
	//should deploy
	if obj.Spec.EnableClusterAlerting || obj.Spec.EnableClusterMonitoring {
		newObj, err := mgmtv3.ClusterConditionPrometheusOperatorDeployed.DoUntilTrue(obj, func() (runtime.Object, error) {
			cpy := obj.DeepCopy()
			return cpy, deploySystemMonitor(cpy, h.app)
		})
		if err != nil {
			logrus.WithError(err).Info("deploy prometheus operator error")
		}
		newCluster = newObj.(*mgmtv3.Cluster)
	} else { // should withdraw
		newCluster = obj.DeepCopy()
		if err = withdrawSystemMonitor(newCluster, h.app); err != nil {
			logrus.WithError(err).Info("withdraw prometheus operator error")
		}
	}

	if newCluster != nil && !reflect.DeepEqual(newCluster, obj) {
		if newCluster, err = h.cattleClusterClient.Update(newCluster); err != nil {
			return nil, err
		}
		return newCluster, nil
	}
	return obj, nil
}

func (h *operatorHandler) syncProject(key string, project *mgmtv3.Project) (runtime.Object, error) {
	if project == nil || project.DeletionTimestamp != nil || project.Spec.ClusterName != h.clusterName {
		return project, nil
	}

	clusterID := project.Spec.ClusterName
	cluster, err := h.cattleClusterClient.Get(clusterID, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find Cluster %s", clusterID)
	}

	var newCluster *mgmtv3.Cluster
	//should deploy
	if project.Spec.EnableProjectMonitoring {
		newObj, err := mgmtv3.ClusterConditionPrometheusOperatorDeployed.DoUntilTrue(cluster, func() (runtime.Object, error) {
			cpy := cluster.DeepCopy()
			return cpy, deploySystemMonitor(cpy, h.app)
		})
		if err != nil {
			logrus.WithError(err).Info("deploy prometheus operator error")
		}
		newCluster = newObj.(*mgmtv3.Cluster)
	} else { // should withdraw
		newCluster = cluster.DeepCopy()
		if err = withdrawSystemMonitor(newCluster, h.app); err != nil {
			logrus.WithError(err).Info("withdraw prometheus operator error")
		}
	}

	if newCluster != nil && !reflect.DeepEqual(newCluster, cluster) {
		if _, err = h.cattleClusterClient.Update(newCluster); err != nil {
			return nil, err
		}
	}

	return project, nil
}

func withdrawSystemMonitor(cluster *mgmtv3.Cluster, app *appHandler) error {
	isAlertingDisabling := mgmtv3.MonitoringConditionAlertmaanagerDeployed.IsFalse(cluster) ||
		mgmtv3.MonitoringConditionAlertmaanagerDeployed.GetStatus(cluster) == ""
	isClusterMonitoringDisabling := mgmtv3.ClusterConditionMonitoringEnabled.IsFalse(cluster) ||
		mgmtv3.ClusterConditionMonitoringEnabled.GetStatus(cluster) == ""
	//status false and empty should withdraw. when status unknown, it means the deployment has error while deploying apps
	isOperatorDeploying := !mgmtv3.ClusterConditionPrometheusOperatorDeployed.IsFalse(cluster)
	areAllOwnedProjectMonitoringDisabling, err := allOwnedProjectsMonitoringDisabling(app.cattleProjectClient)
	if err != nil {
		mgmtv3.ClusterConditionPrometheusOperatorDeployed.Unknown(cluster)
		mgmtv3.ClusterConditionPrometheusOperatorDeployed.ReasonAndMessageFromError(cluster, errors.Wrap(err, "failed to list owned projects of cluster"))
		return err
	}

	if areAllOwnedProjectMonitoringDisabling && isAlertingDisabling && isClusterMonitoringDisabling && isOperatorDeploying {
		appName, appTargetNamespace := monitoring.SystemMonitoringInfo()

		if err := monitoring.WithdrawApp(app.cattleAppClient, monitoring.OwnedAppListOptions(cluster.Name, appName, appTargetNamespace)); err != nil {
			mgmtv3.ClusterConditionPrometheusOperatorDeployed.Unknown(cluster)
			mgmtv3.ClusterConditionPrometheusOperatorDeployed.ReasonAndMessageFromError(cluster, errors.Wrap(err, "failed to withdraw prometheus operator app"))
			return err
		}

		mgmtv3.ClusterConditionPrometheusOperatorDeployed.False(cluster)
	}

	return nil
}

func allOwnedProjectsMonitoringDisabling(projectClient mgmtv3.ProjectInterface) (bool, error) {
	ownedProjectList, err := projectClient.List(metav1.ListOptions{})
	if err != nil {
		return false, errors.Wrap(err, "failed to list all projects")
	}

	for _, ownedProject := range ownedProjectList.Items {
		if ownedProject.Spec.EnableProjectMonitoring {
			return false, nil
		}
	}

	return true, nil
}

func deploySystemMonitor(cluster *mgmtv3.Cluster, app *appHandler) (backErr error) {
	appName, appTargetNamespace := monitoring.SystemMonitoringInfo()

	appCatalogID := settings.SystemMonitoringCatalogID.Get()
	err := monitoring.DetectAppCatalogExistence(appCatalogID, app.cattleTemplateVersionClient)
	if err != nil {
		return errors.Wrapf(err, "failed to ensure catalog %q", appCatalogID)
	}

	appDeployProjectID, err := monitoring.GetSystemProjectID(app.cattleProjectClient)
	if err != nil {
		return errors.Wrap(err, "failed to get System Project ID")
	}

	appProjectName, err := monitoring.EnsureAppProjectName(app.agentNamespaceClient, appDeployProjectID, cluster.Name, appTargetNamespace)
	if err != nil {
		return errors.Wrap(err, "failed to ensure monitoring project name")
	}

	appAnswers := map[string]string{
		"enabled":      "true",
		"apiGroup":     monitoring.APIVersion.Group,
		"nameOverride": "prometheus-operator",
	}
	annotations := monitoring.CopyCreatorID(nil, cluster.Annotations)
	annotations["cluster.cattle.io/addon"] = appName
	targetApp := &projectv3.App{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: annotations,
			Labels:      monitoring.OwnedLabels(appName, appTargetNamespace, appProjectName, monitoring.SystemLevel),
			Name:        appName,
			Namespace:   appDeployProjectID,
		},
		Spec: projectv3.AppSpec{
			Answers:         appAnswers,
			Description:     "Prometheus Operator for Rancher Monitoring",
			ExternalID:      appCatalogID,
			ProjectName:     appProjectName,
			TargetNamespace: appTargetNamespace,
		},
	}

	err = monitoring.DeployApp(app.cattleAppClient, appDeployProjectID, targetApp)
	if err != nil {
		return errors.Wrap(err, "failed to ensure prometheus operator app")
	}

	return nil
}
