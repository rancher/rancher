package monitoring

import (
	"fmt"
	"reflect"
	"strings"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v33 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"

	"github.com/rancher/rancher/pkg/app/utils"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/pkg/errors"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	projectv3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/monitoring"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type operatorHandler struct {
	clusterName   string
	clusters      mgmtv3.ClusterInterface
	clusterLister mgmtv3.ClusterLister
	app           *appHandler
}

func (h *operatorHandler) syncCluster(key string, obj *mgmtv3.Cluster) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil || obj.Name != h.clusterName {
		return obj, nil
	}

	if !v32.ClusterConditionAgentDeployed.IsTrue(obj) {
		return obj, nil
	}

	var newCluster *mgmtv3.Cluster
	var err error
	//should deploy
	if obj.Spec.EnableClusterAlerting || obj.Spec.EnableClusterMonitoring {
		newObj, err := v32.ClusterConditionPrometheusOperatorDeployed.Do(obj, func() (runtime.Object, error) {
			cpy := obj.DeepCopy()
			return cpy, deploySystemMonitor(cpy, h.app)
		})
		if err != nil {
			logrus.Warnf("deploy prometheus operator error, %v", err)
		}
		newCluster = newObj.(*mgmtv3.Cluster)
	} else { // should withdraw
		newCluster = obj.DeepCopy()
		if err = withdrawSystemMonitor(newCluster, h.app); err != nil {
			logrus.Warnf("withdraw prometheus operator error, %v", err)
		}
	}

	if newCluster != nil && !reflect.DeepEqual(newCluster, obj) {
		if newCluster, err = h.clusters.Update(newCluster); err != nil {
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
	cluster, err := h.clusterLister.Get("", clusterID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find Cluster %s", clusterID)
	}

	if !v32.ClusterConditionAgentDeployed.IsTrue(cluster) {
		return project, nil
	}

	var newCluster *mgmtv3.Cluster
	//should deploy
	if cluster.Spec.EnableClusterAlerting || project.Spec.EnableProjectMonitoring {
		newObj, err := v32.ClusterConditionPrometheusOperatorDeployed.Do(cluster, func() (runtime.Object, error) {
			cpy := cluster.DeepCopy()
			return cpy, deploySystemMonitor(cpy, h.app)
		})
		if err != nil {
			logrus.Warnf("deploy prometheus operator error, %v", err)
		}
		newCluster = newObj.(*mgmtv3.Cluster)
	} else { // should withdraw
		newCluster = cluster.DeepCopy()
		if err = withdrawSystemMonitor(newCluster, h.app); err != nil {
			logrus.Warnf("withdraw prometheus operator error, %v", err)
		}
	}

	if newCluster != nil && !reflect.DeepEqual(newCluster, cluster) {
		if _, err = h.clusters.Update(newCluster); err != nil {
			return nil, err
		}
	}

	return project, nil
}

func withdrawSystemMonitor(cluster *v32.Cluster, app *appHandler) error {
	isAlertingDisabling := v32.ClusterConditionAlertingEnabled.IsFalse(cluster) ||
		v32.ClusterConditionAlertingEnabled.GetStatus(cluster) == ""
	isClusterMonitoringDisabling := v32.ClusterConditionMonitoringEnabled.IsFalse(cluster) ||
		v32.ClusterConditionMonitoringEnabled.GetStatus(cluster) == ""
	//status false and empty should withdraw. when status unknown, it means the deployment has error while deploying apps
	isOperatorDeploying := !v32.ClusterConditionPrometheusOperatorDeployed.IsFalse(cluster)
	areAllOwnedProjectMonitoringDisabling, err := allOwnedProjectsMonitoringDisabling(app.projectLister)
	if err != nil {
		v32.ClusterConditionPrometheusOperatorDeployed.Unknown(cluster)
		v32.ClusterConditionPrometheusOperatorDeployed.ReasonAndMessageFromError(cluster, errors.Wrap(err, "failed to list owned projects of cluster"))
		return err
	}

	if areAllOwnedProjectMonitoringDisabling && isAlertingDisabling && isClusterMonitoringDisabling && isOperatorDeploying {
		appName, appTargetNamespace := monitoring.SystemMonitoringInfo()

		if err := monitoring.WithdrawApp(app.cattleAppClient, monitoring.OwnedAppListOptions(cluster.Name, appName, appTargetNamespace)); err != nil {
			v32.ClusterConditionPrometheusOperatorDeployed.Unknown(cluster)
			v32.ClusterConditionPrometheusOperatorDeployed.ReasonAndMessageFromError(cluster, errors.Wrap(err, "failed to withdraw prometheus operator app"))
			return err
		}

		v32.ClusterConditionPrometheusOperatorDeployed.False(cluster)
		v32.ClusterConditionPrometheusOperatorDeployed.Reason(cluster, "")
		v32.ClusterConditionPrometheusOperatorDeployed.Message(cluster, "")
	}

	return nil
}

func allOwnedProjectsMonitoringDisabling(projectClient mgmtv3.ProjectLister) (bool, error) {
	ownedProjectList, err := projectClient.List("", labels.NewSelector())
	if err != nil {
		return false, err
	}

	for _, ownedProject := range ownedProjectList {
		if ownedProject.Spec.EnableProjectMonitoring {
			return false, nil
		}
	}

	return true, nil
}

func deploySystemMonitor(cluster *mgmtv3.Cluster, app *appHandler) (backErr error) {
	appName, appTargetNamespace := monitoring.SystemMonitoringInfo()

	appDeployProjectID, err := utils.GetSystemProjectID(cluster.Name, app.projectLister)
	if err != nil {
		return errors.Wrap(err, "failed to get System Project ID")
	}

	creator, err := app.systemAccountManager.GetSystemUser(cluster.Name)
	if err != nil {
		return err
	}

	appProjectName, err := utils.EnsureAppProjectName(app.agentNamespaceClient, appDeployProjectID, cluster.Name, appTargetNamespace, creator.Name)
	if err != nil {
		return errors.Wrap(err, "failed to ensure monitoring project name")
	}

	appAnswers := map[string]string{
		"enabled":      "true",
		"apiGroup":     monitoring.APIVersion.Group,
		"nameOverride": "prometheus-operator",
	}

	mustAppAnswers := map[string]string{
		"operator.apiGroup":     monitoring.APIVersion.Group,
		"operator.nameOverride": "prometheus-operator",
		"operator-init.enabled": "true",
	}

	// take operator answers from overwrite answers
	answers, version := monitoring.GetOverwroteAppAnswersAndVersion(cluster.Annotations)
	for ansKey, ansVal := range answers {
		if strings.HasPrefix(ansKey, "operator.") {
			appAnswers[ansKey] = ansVal
		}
	}

	// cannot overwrite mustAppAnswers
	for mustKey, mustVal := range mustAppAnswers {
		appAnswers[mustKey] = mustVal
	}

	annotations := map[string]string{
		"cluster.cattle.io/addon": appName,
		creatorIDAnno:             creator.Name,
	}

	appCatalogID, err := monitoring.GetMonitoringCatalogID(version, app.catalogTemplateLister)
	if err != nil {
		return err
	}

	targetApp := &projectv3.App{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: annotations,
			Labels:      monitoring.OwnedLabels(appName, appTargetNamespace, appProjectName, monitoring.SystemLevel),
			Name:        appName,
			Namespace:   appDeployProjectID,
		},
		Spec: v33.AppSpec{
			Answers:         appAnswers,
			Description:     "Prometheus Operator for Rancher Monitoring",
			ExternalID:      appCatalogID,
			ProjectName:     appProjectName,
			TargetNamespace: appTargetNamespace,
		},
	}

	// redeploy operator App forcibly if cannot find the workload
	var forceRedeploy bool
	appWorkload, err := app.agentDeploymentClient.GetNamespaced(appTargetNamespace, fmt.Sprintf("prometheus-operator-%s", appName), metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrapf(err, "failed to get deployment %s/prometheus-operator-%s", appTargetNamespace, appName)
	}
	if appWorkload == nil || appWorkload.Name == "" || appWorkload.DeletionTimestamp != nil {
		forceRedeploy = true
	}

	_, err = utils.DeployApp(app.cattleAppClient, appDeployProjectID, targetApp, forceRedeploy)
	if err != nil {
		return errors.Wrap(err, "failed to ensure prometheus operator app")
	}

	return nil
}
