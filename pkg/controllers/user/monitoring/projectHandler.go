package monitoring

import (
	"fmt"
	"reflect"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/controllers/user/nslabels"
	"github.com/rancher/rancher/pkg/monitoring"
	"github.com/rancher/rancher/pkg/settings"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	k8scorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type projectHandler struct {
	clusterName         string
	cattleClusterClient mgmtv3.ClusterInterface
	cattleProjectClient mgmtv3.ProjectInterface
	app                 *appHandler
}

func (ph *projectHandler) sync(key string, project *mgmtv3.Project) (runtime.Object, error) {
	if project == nil || project.DeletionTimestamp != nil ||
		project.Spec.ClusterName != ph.clusterName {
		return project, nil
	}

	clusterID := project.Spec.ClusterName
	cluster, err := ph.cattleClusterClient.Get(clusterID, metav1.GetOptions{})
	if err != nil {
		return project, errors.Wrapf(err, "failed to find Cluster %s", clusterID)
	}

	clusterName := cluster.Spec.DisplayName
	projectTag := getProjectTag(project, clusterName)
	src := project
	cpy := src.DeepCopy()

	err = ph.doSync(cpy, clusterName)
	if !reflect.DeepEqual(cpy, src) {
		updated, updateErr := ph.cattleProjectClient.Update(cpy)
		if updateErr != nil {
			return project, errors.Wrapf(updateErr, "failed to update Project %s", projectTag)
		}

		cpy = updated
	}

	if err != nil {
		err = errors.Wrapf(err, "unable to sync Project %s", projectTag)
	}

	return cpy, err
}

func (ph *projectHandler) doSync(project *mgmtv3.Project, clusterName string) error {
	if !mgmtv3.NamespaceBackedResource.IsTrue(project) && !mgmtv3.ProjectConditionInitialRolesPopulated.IsTrue(project) {
		return nil
	}
	_, err := mgmtv3.ProjectConditionMetricExpressionDeployed.DoUntilTrue(project, func() (runtime.Object, error) {
		projectName := fmt.Sprintf("%s:%s", project.Spec.ClusterName, project.Name)

		for _, graph := range preDefinedProjectGraph {
			newObj := graph.DeepCopy()
			newObj.Namespace = project.Name
			newObj.Spec.ProjectName = projectName
			if _, err := ph.app.cattleProjectGraphClient.Create(newObj); err != nil && !apierrors.IsAlreadyExists(err) {
				return project, err
			}
		}

		return project, nil
	})
	if err != nil {
		return errors.Wrap(err, "failed to apply metric expression")
	}

	appName, appTargetNamespace := monitoring.ProjectMonitoringInfo(project.Name)

	if project.Spec.EnableProjectMonitoring {
		appProjectName, err := ph.ensureAppProjectName(appTargetNamespace, project)
		if err != nil {
			mgmtv3.ProjectConditionMonitoringEnabled.Unknown(project)
			mgmtv3.ProjectConditionMonitoringEnabled.Message(project, err.Error())
			return errors.Wrap(err, "failed to ensure monitoring project name")
		}

		if err := ph.deployApp(appName, appTargetNamespace, appProjectName, project, clusterName); err != nil {
			mgmtv3.ProjectConditionMonitoringEnabled.Unknown(project)
			mgmtv3.ProjectConditionMonitoringEnabled.Message(project, err.Error())
			return errors.Wrap(err, "failed to deploy monitoring")
		}

		if err := ph.detectAppComponentsWhileInstall(appName, appTargetNamespace, project); err != nil {
			mgmtv3.ProjectConditionMonitoringEnabled.Unknown(project)
			mgmtv3.ProjectConditionMonitoringEnabled.Message(project, err.Error())
			return errors.Wrap(err, "failed to detect the installation status of monitoring components")
		}

		mgmtv3.ProjectConditionMonitoringEnabled.True(project)
		mgmtv3.ProjectConditionMonitoringEnabled.Message(project, "")
	} else if project.Status.MonitoringStatus != nil {
		if err := ph.app.withdrawApp(project.Spec.ClusterName, appName, appTargetNamespace); err != nil {
			mgmtv3.ProjectConditionMonitoringEnabled.Unknown(project)
			mgmtv3.ProjectConditionMonitoringEnabled.Message(project, err.Error())
			return errors.Wrap(err, "failed to withdraw monitoring")
		}

		if err := ph.detectAppComponentsWhileUninstall(appName, appTargetNamespace, project); err != nil {
			mgmtv3.ProjectConditionMonitoringEnabled.Unknown(project)
			mgmtv3.ProjectConditionMonitoringEnabled.Message(project, err.Error())
			return errors.Wrap(err, "failed to detect the uninstallation status of monitoring components")
		}

		mgmtv3.ProjectConditionMonitoringEnabled.False(project)
		mgmtv3.ProjectConditionMonitoringEnabled.Message(project, "")
	}

	return nil
}

func (ph *projectHandler) ensureAppProjectName(appTargetNamespace string, project *mgmtv3.Project) (string, error) {
	appProjectName, err := monitoring.EnsureAppProjectName(ph.app.agentNamespaceClient, project.Name, project.Spec.ClusterName, appTargetNamespace)
	if err != nil {
		return "", err
	}

	return appProjectName, nil
}

func (ph *projectHandler) deployApp(appName, appTargetNamespace string, appProjectName string, project *mgmtv3.Project, clusterName string) error {
	appDeployProjectID := project.Name
	clusterPrometheusSvcName, clusterPrometheusSvcNamespaces, clusterPrometheusPort := monitoring.ClusterPrometheusEndpoint()
	clusterAlertManagerSvcName, clusterAlertManagerSvcNamespaces, clusterAlertManagerPort := monitoring.ClusterAlertManagerEndpoint()

	optionalAppAnswers := map[string]string{
		"grafana.persistence.enabled": "false",

		"prometheus.persistence.enabled": "false",

		"prometheus.sync.mode": "federate",
	}

	mustAppAnswers := map[string]string{
		"enabled": "false",

		"exporter-coredns.enabled": "false",

		"exporter-kube-controller-manager.enabled": "false",

		"exporter-kube-dns.enabled": "false",

		"exporter-kube-scheduler.enabled": "false",

		"exporter-kube-state.enabled": "false",

		"exporter-kubelets.enabled": "false",

		"exporter-kubernetes.enabled": "false",

		"exporter-node.enabled": "false",

		"exporter-fluentd.enabled": "false",

		"grafana.enabled":            "true",
		"grafana.level":              "project",
		"grafana.apiGroup":           monitoring.APIVersion.Group,
		"grafana.serviceAccountName": appName,

		"prometheus.enabled":                          "true",
		"prometheus.externalLabels.prometheus_from":   clusterName,
		"prometheus.level":                            "project",
		"prometheus.apiGroup":                         monitoring.APIVersion.Group,
		"prometheus.serviceAccountNameOverride":       appName,
		"prometheus.additionalBindingClusterRoles[0]": fmt.Sprintf("%s-namespaces-readonly", appDeployProjectID),
		"prometheus.additionalAlertManagerConfigs[0].static_configs[0].targets[0]":          fmt.Sprintf("%s.%s:%s", clusterAlertManagerSvcName, clusterAlertManagerSvcNamespaces, clusterAlertManagerPort),
		"prometheus.additionalAlertManagerConfigs[0].static_configs[0].labels.level":        "project",
		"prometheus.additionalAlertManagerConfigs[0].static_configs[0].labels.project_id":   project.Name,
		"prometheus.additionalAlertManagerConfigs[0].static_configs[0].labels.project_name": project.Spec.DisplayName,
		"prometheus.additionalAlertManagerConfigs[0].static_configs[0].labels.cluster_id":   project.Spec.ClusterName,
		"prometheus.additionalAlertManagerConfigs[0].static_configs[0].labels.cluster_name": clusterName,
		"prometheus.serviceMonitorNamespaceSelector.matchExpressions[0].key":                nslabels.ProjectIDFieldLabel,
		"prometheus.serviceMonitorNamespaceSelector.matchExpressions[0].operator":           "In",
		"prometheus.serviceMonitorNamespaceSelector.matchExpressions[0].values[0]":          appDeployProjectID,
		"prometheus.ruleNamespaceSelector.matchExpressions[0].key":                          nslabels.ProjectIDFieldLabel,
		"prometheus.ruleNamespaceSelector.matchExpressions[0].operator":                     "In",
		"prometheus.ruleNamespaceSelector.matchExpressions[0].values[0]":                    appDeployProjectID,
		"prometheus.ruleSelector.matchExpressions[0].key":                                   monitoring.CattlePrometheusRuleLabelKey,
		"prometheus.ruleSelector.matchExpressions[0].operator":                              "In",
		"prometheus.ruleSelector.matchExpressions[0].values[0]":                             monitoring.CattleAlertingPrometheusRuleLabelValue,
	}

	appAnswers := monitoring.OverwriteAppAnswers(optionalAppAnswers, project.Annotations)

	// cannot overwrite mustAppAnswers
	for mustKey, mustVal := range mustAppAnswers {
		appAnswers[mustKey] = mustVal
	}

	// complete sync target & path
	if appAnswers["prometheus.sync.mode"] == "federate" {
		appAnswers["prometheus.sync.target"] = fmt.Sprintf("%s.%s:%s", clusterPrometheusSvcName, clusterPrometheusSvcNamespaces, clusterPrometheusPort)
		appAnswers["prometheus.sync.path"] = "/federate"
	} else {
		appAnswers["prometheus.sync.target"] = fmt.Sprintf("http://%s.%s:%s", clusterPrometheusSvcName, clusterPrometheusSvcNamespaces, clusterPrometheusPort)
		appAnswers["prometheus.sync.path"] = "/api/v1/read"
	}

	appCatalogID := settings.SystemMonitoringCatalogID.Get()
	app := &v3.App{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: monitoring.CopyCreatorID(nil, project.Annotations),
			Labels:      monitoring.OwnedLabels(appName, appTargetNamespace, appProjectName, monitoring.ProjectLevel),
			Name:        appName,
			Namespace:   appDeployProjectID,
		},
		Spec: v3.AppSpec{
			Answers:         appAnswers,
			Description:     "Rancher Project Monitoring",
			ExternalID:      appCatalogID,
			ProjectName:     appProjectName,
			TargetNamespace: appTargetNamespace,
		},
	}

	err := monitoring.DeployApp(ph.app.cattleAppClient, appDeployProjectID, app)
	if err != nil {
		return err
	}

	return nil
}

func (ph *projectHandler) detectAppComponentsWhileInstall(appName, appTargetNamespace string, project *mgmtv3.Project) error {
	if project.Status.MonitoringStatus == nil {
		project.Status.MonitoringStatus = &mgmtv3.MonitoringStatus{
			Conditions: []mgmtv3.MonitoringCondition{
				{Type: mgmtv3.ClusterConditionType(ConditionPrometheusDeployed), Status: k8scorev1.ConditionFalse},
				{Type: mgmtv3.ClusterConditionType(ConditionGrafanaDeployed), Status: k8scorev1.ConditionFalse},
			},
		}
	}
	monitoringStatus := project.Status.MonitoringStatus

	checkers := make([]func() error, 0, len(monitoringStatus.Conditions))
	if !ConditionGrafanaDeployed.IsTrue(monitoringStatus) {
		checkers = append(checkers, func() error {
			return isGrafanaDeployed(ph.app.agentDeploymentClient, appTargetNamespace, appName, monitoringStatus, project.Spec.ClusterName)
		})
	}
	if !ConditionPrometheusDeployed.IsTrue(monitoringStatus) {
		checkers = append(checkers, func() error {
			return isPrometheusDeployed(ph.app.agentStatefulSetClient, appTargetNamespace, appName, monitoringStatus)
		})
	}

	if len(checkers) == 0 {
		return nil
	}

	err := stream(checkers...)
	if err != nil {
		time.Sleep(5 * time.Second)
	}

	return err
}

func (ph *projectHandler) detectAppComponentsWhileUninstall(appName, appTargetNamespace string, project *mgmtv3.Project) error {
	if project.Status.MonitoringStatus == nil {
		return nil
	}
	monitoringStatus := project.Status.MonitoringStatus

	checkers := make([]func() error, 0, len(monitoringStatus.Conditions))
	if !ConditionGrafanaDeployed.IsFalse(monitoringStatus) {
		checkers = append(checkers, func() error {
			return isGrafanaWithdrew(ph.app.agentDeploymentClient, appTargetNamespace, appName, monitoringStatus)
		})
	}
	if !ConditionPrometheusDeployed.IsFalse(monitoringStatus) {
		checkers = append(checkers, func() error {
			return isPrometheusWithdrew(ph.app.agentStatefulSetClient, appTargetNamespace, appName, monitoringStatus)
		})
	}

	if len(checkers) == 0 {
		return nil
	}

	err := stream(checkers...)
	if err != nil {
		time.Sleep(5 * time.Second)
	}

	return err
}

func getProjectTag(project *mgmtv3.Project, clusterName string) string {
	return fmt.Sprintf("%s(%s) of Cluster %s(%s)", project.Name, project.Spec.DisplayName, project.Spec.ClusterName, clusterName)
}
