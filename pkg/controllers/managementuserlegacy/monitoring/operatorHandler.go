package monitoring

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v33 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	app2 "github.com/rancher/rancher/pkg/app"
	"github.com/rancher/rancher/pkg/catalog/manager"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	projectv3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/monitoring"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

type operatorHandler struct {
	clusterName    string
	clusters       mgmtv3.ClusterInterface
	clusterLister  mgmtv3.ClusterLister
	catalogManager manager.CatalogManager
	app            *appHandler
}

func (h *operatorHandler) syncCluster(key string, obj *mgmtv3.Cluster) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil || obj.Name != h.clusterName {
		return obj, nil
	}

	if !obj.Spec.Internal && !v32.ClusterConditionAgentDeployed.IsTrue(obj) {
		return obj, nil
	}

	cpy := obj.DeepCopy()
	var returnErr, updateErr error
	var newObj runtime.Object
	//should deploy
	if obj.Spec.EnableClusterAlerting || obj.Spec.EnableClusterMonitoring {
		newObj, returnErr = v32.ClusterConditionPrometheusOperatorDeployed.Do(cpy, func() (runtime.Object, error) {
			return cpy, deploySystemMonitor(cpy, h.app, h.catalogManager, h.clusterName)
		})
		cpy = newObj.(*mgmtv3.Cluster)
	} else { // should withdraw
		returnErr = withdrawSystemMonitor(cpy, h.app)
	}

	if !reflect.DeepEqual(cpy, obj) {
		if cpy, updateErr = h.clusters.Update(cpy); updateErr != nil {
			returnErr = multierror.Append(returnErr, updateErr)
		}
	}

	return cpy, returnErr
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

	if !cluster.Spec.Internal && !v32.ClusterConditionAgentDeployed.IsTrue(cluster) {
		return project, nil
	}

	cpyCluster := cluster.DeepCopy()
	var returnErr error
	var newObj runtime.Object
	//should deploy
	if cluster.Spec.EnableClusterAlerting || project.Spec.EnableProjectMonitoring {
		newObj, returnErr = v32.ClusterConditionPrometheusOperatorDeployed.Do(cluster, func() (runtime.Object, error) {
			return cpyCluster, deploySystemMonitor(cpyCluster, h.app, h.catalogManager, cluster.Name)
		})
		cpyCluster = newObj.(*mgmtv3.Cluster)
	} else { // should withdraw
		returnErr = withdrawSystemMonitor(cpyCluster, h.app)
	}

	if !reflect.DeepEqual(cpyCluster, cluster) {
		if _, updateErr := h.clusters.Update(cpyCluster); updateErr != nil {
			returnErr = multierror.Append(returnErr, updateErr)
		}
	}

	return project, returnErr
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

func deploySystemMonitor(cluster *mgmtv3.Cluster, app *appHandler, catalogManager manager.CatalogManager, clusterName string) (backErr error) {
	appName, appTargetNamespace := monitoring.SystemMonitoringInfo()

	appDeployProjectID, err := app2.GetSystemProjectID(cluster.Name, app.projectLister)
	if err != nil {
		return errors.Wrap(err, "failed to get System Project ID")
	}

	creator, err := app.systemAccountManager.GetSystemUser(cluster.Name)
	if err != nil {
		return err
	}

	appProjectName, err := app2.EnsureAppProjectName(app.agentNamespaceClient, appDeployProjectID, cluster.Name, appTargetNamespace, creator.Name)
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
	monitoringInput := monitoring.GetMonitoringInput(cluster.Annotations)
	answers := monitoringInput.Answers
	answersSetString := monitoringInput.AnswersSetString
	version := monitoringInput.Version
	resolveOperatorPrefix(answers)
	resolveOperatorPrefix(answersSetString)

	// cannot overwrite mustAppAnswers
	for mustKey, mustVal := range mustAppAnswers {
		appAnswers[mustKey] = mustVal
		delete(answersSetString, mustKey)
	}

	annotations := map[string]string{
		"cluster.cattle.io/addon": appName,
		creatorIDAnno:             creator.Name,
	}

	appCatalogID, err := monitoring.GetMonitoringCatalogID(version, app.catalogTemplateLister, catalogManager, clusterName)
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
			Answers:          appAnswers,
			AnswersSetString: answersSetString,
			Description:      "Prometheus Operator for Rancher Monitoring",
			ExternalID:       appCatalogID,
			ProjectName:      appProjectName,
			TargetNamespace:  appTargetNamespace,
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

	_, err = app2.DeployApp(app.cattleAppClient, appDeployProjectID, targetApp, forceRedeploy)
	if err != nil {
		return errors.Wrap(err, "failed to ensure prometheus operator app")
	}

	return nil
}

func resolveOperatorPrefix(answers map[string]string) {
	for ansKey, ansVal := range answers {
		if strings.HasPrefix(ansKey, "operator.") || strings.HasPrefix(ansKey, "operator-init.") {
			answers[ansKey] = ansVal
		}
	}
}
