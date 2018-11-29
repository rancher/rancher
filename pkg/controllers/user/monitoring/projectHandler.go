package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/controllers/user/helm/common"
	"github.com/rancher/rancher/pkg/monitoring"
	"github.com/rancher/rancher/pkg/settings"
	appsv1beta2 "github.com/rancher/types/apis/apps/v1beta2"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	projectv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	k8scorev1 "k8s.io/api/core/v1"
	k8srbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type projectHandler struct {
	ctx                  context.Context
	clusterName          string
	cattleClustersClient mgmtv3.ClusterInterface
	cattleProjectsClient mgmtv3.ProjectInterface
	app                  *appHandler
	agentWorkloadsClient appsv1beta2.Interface
}

func (ph *projectHandler) sync(key string, project *mgmtv3.Project) (runtime.Object, error) {
	if project == nil || project.DeletionTimestamp != nil ||
		project.Spec.ClusterName != ph.clusterName {
		return project, nil
	}

	_, err := ph.cattleClustersClient.Get(project.Spec.ClusterName, metav1.GetOptions{})
	if err != nil {
		return project, errors.Wrapf(err, "failed to find Cluster %s", project.Spec.ClusterName)
	}

	projectTag := getProjectTag(project)
	src := project
	cpy := src.DeepCopy()

	err = ph.doSync(projectTag, cpy)

	if !reflect.DeepEqual(cpy, src) {
		_, err := ph.cattleProjectsClient.Update(cpy)
		if err != nil {
			return project, errors.Wrapf(err, "failed to update Project %s in Cluster %s", projectTag, project.Spec.ClusterName)
		}
	}

	return cpy, err
}

func (ph *projectHandler) doSync(projectTag string, project *mgmtv3.Project) error {
	_, err := mgmtv3.ProjectConditionMetricExpressionDeployed.DoUntilTrue(project, func() (runtime.Object, error) {
		projectName := fmt.Sprintf("%s:%s", project.Spec.ClusterName, project.Name)

		tmpDate := templateData{ProjectName: projectName}
		expressions, err := generate(ProjectMetricExpression, tmpDate)
		if err != nil {
			return nil, err
		}

		return project, deployAddonWithKubectl(project.Name, expressions)
	})
	if err != nil {
		return fmt.Errorf("apply metric expression for project %s failed, %v", projectTag, err)
	}

	if project.Spec.EnableProjectMonitoring == nil {
		return nil
	}

	enableProjectMonitoring := *project.Spec.EnableProjectMonitoring
	appName, appTargetNamespace := monitoring.ProjectMonitoringInfo(project)

	if enableProjectMonitoring {
		if !mgmtv3.ProjectConditionMonitoringEnabled.IsTrue(project) {
			appProjectName, err := ph.app.ensureProjectMonitoringProjectName(appTargetNamespace, project)
			if err != nil {
				mgmtv3.ProjectConditionMonitoringEnabled.Unknown(project)
				mgmtv3.ProjectConditionMonitoringEnabled.Message(project, err.Error())
				return errors.Wrapf(err, "failed to ensure monitoring project name for Project %s", projectTag)
			}

			deployServiceAccountName, err := ph.app.grantProjectMonitoringRBAC(appName, appTargetNamespace, project)
			if err != nil {
				mgmtv3.ProjectConditionMonitoringEnabled.Unknown(project)
				mgmtv3.ProjectConditionMonitoringEnabled.Message(project, err.Error())
				return errors.Wrapf(err, "failed to grant prometheus RBAC into Project %s", projectTag)
			}

			if err := ph.app.deployProjectMonitoring(appName, appTargetNamespace, deployServiceAccountName, appProjectName, project); err != nil {
				mgmtv3.ProjectConditionMonitoringEnabled.Unknown(project)
				mgmtv3.ProjectConditionMonitoringEnabled.Message(project, err.Error())
				return errors.Wrapf(err, "failed to deploy monitoring into Project %s", projectTag)
			}

			if err := ph.detectMonitoringComponentsWhileInstall(appName, appTargetNamespace, project); err != nil {
				mgmtv3.ProjectConditionMonitoringEnabled.Unknown(project)
				mgmtv3.ProjectConditionMonitoringEnabled.Message(project, err.Error())
				return errors.Wrapf(err, "failed to detect the installation status of monitoring components in Project %s", projectTag)
			}

			mgmtv3.ProjectConditionMonitoringEnabled.True(project)
			mgmtv3.ProjectConditionMonitoringEnabled.Message(project, "")
		}
	} else {
		hasConditionMonitoringEnabled := false
		for _, cond := range project.Status.Conditions {
			if string(cond.Type) == string(mgmtv3.ProjectConditionMonitoringEnabled) {
				hasConditionMonitoringEnabled = true
			}
		}

		if hasConditionMonitoringEnabled && !mgmtv3.ProjectConditionMonitoringEnabled.IsFalse(project) {
			if err := ph.app.withdrawMonitoring(appName, appTargetNamespace); err != nil {
				mgmtv3.ProjectConditionMonitoringEnabled.Unknown(project)
				mgmtv3.ProjectConditionMonitoringEnabled.Message(project, err.Error())
				return errors.Wrapf(err, "failed to withdraw monitoring from Project %s", projectTag)
			}

			if err := ph.detectMonitoringComponentsWhileUninstall(appName, appTargetNamespace, project); err != nil {
				mgmtv3.ProjectConditionMonitoringEnabled.Unknown(project)
				mgmtv3.ProjectConditionMonitoringEnabled.Message(project, err.Error())
				return errors.Wrapf(err, "failed to detect the uninstallation status of monitoring components in Project %s", projectTag)
			}

			if err := ph.app.revokeProjectMonitoringRBAC(appName, appTargetNamespace, project); err != nil {
				mgmtv3.ProjectConditionMonitoringEnabled.Unknown(project)
				mgmtv3.ProjectConditionMonitoringEnabled.Message(project, err.Error())
				return errors.Wrapf(err, "failed to revoke prometheus RBAC from Project %s", projectTag)
			}

			mgmtv3.ProjectConditionMonitoringEnabled.False(project)
			mgmtv3.ProjectConditionMonitoringEnabled.Message(project, "")
		}
	}

	return nil
}

func (ph *projectHandler) detectMonitoringComponentsWhileInstall(appName, appTargetNamespace string, project *mgmtv3.Project) error {
	time.Sleep(5 * time.Second)

	if project.Status.MonitoringStatus == nil {
		project.Status.MonitoringStatus = &mgmtv3.MonitoringStatus{
			// in case of races
			Conditions: []mgmtv3.MonitoringCondition{
				{Type: mgmtv3.ClusterConditionType(ConditionGrafanaDeployed), Status: k8scorev1.ConditionFalse},
			},
		}
	}

	monitoringStatus := project.Status.MonitoringStatus

	return stream(
		func() error {
			return isGrafanaDeployed(ph.agentWorkloadsClient, appTargetNamespace, appName, monitoringStatus, project.Spec.ClusterName)
		},
	)
}

func (ph *projectHandler) detectMonitoringComponentsWhileUninstall(appName, appTargetNamespace string, project *mgmtv3.Project) error {
	if project.Status.MonitoringStatus == nil {
		return nil
	}

	time.Sleep(5 * time.Second)

	monitoringStatus := project.Status.MonitoringStatus

	return stream(
		func() error {
			return isGrafanaWithdrew(ph.agentWorkloadsClient, appTargetNamespace, appName, monitoringStatus)
		},
	)
}

func (ah *appHandler) ensureProjectMonitoringProjectName(appTargetNamespace string, project *mgmtv3.Project) (string, error) {
	// detect Namespace
	deployNamespace, err := ah.agentNamespacesClient.Get(appTargetNamespace, metav1.GetOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return "", errors.Wrapf(err, "failed to find %q Namespace", appTargetNamespace)
	}
	deployNamespace = deployNamespace.DeepCopy()

	if deployNamespace.Name == appTargetNamespace {
		if deployNamespace.DeletionTimestamp != nil {
			return "", errors.New(fmt.Sprintf("stale %q Namespace is still on terminating", appTargetNamespace))
		}
	} else {
		deployNamespace = &k8scorev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: appTargetNamespace,
			},
		}

		if deployNamespace, err = ah.agentNamespacesClient.Create(deployNamespace); err != nil && !k8serrors.IsAlreadyExists(err) {
			return "", errors.Wrapf(err, "failed to create %q Namespace", appTargetNamespace)
		}
	}

	expectedAppProjectName := fmt.Sprintf("%s:%s", project.Spec.ClusterName, project.Name)
	appProjectName := ""
	if projectName, ok := deployNamespace.Annotations[monitoring.CattleProjectIDAnnotationKey]; ok {
		appProjectName = projectName
	}
	if appProjectName != expectedAppProjectName {
		appProjectName = expectedAppProjectName
		if deployNamespace.Annotations == nil {
			deployNamespace.Annotations = make(map[string]string, 2)
		}
		if deployNamespace.Labels == nil {
			deployNamespace.Labels = make(map[string]string, 2)
		}

		deployNamespace.Annotations[monitoring.CattleProjectIDAnnotationKey] = appProjectName
		deployNamespace.Labels[monitoring.CattleMonitoringLabelKey] = "true"

		_, err := ah.agentNamespacesClient.Update(deployNamespace)
		if err != nil {
			return "", errors.Wrapf(err, "failed to move Namespace %s to Project %s", appTargetNamespace, project.Spec.DisplayName)
		}
	}

	// mark namespace as monitoring owned
	if deployNamespace.Labels == nil {
		deployNamespace.Labels = make(map[string]string, 2)
	}
	if deployNamespace.Labels[monitoring.CattleMonitoringLabelKey] != "true" {
		deployNamespace.Labels[monitoring.CattleMonitoringLabelKey] = "true"

		if _, err := ah.agentNamespacesClient.Update(deployNamespace); err != nil {
			return "", errors.Wrapf(err, "failed to mark Namespace %s as monitoring owned", appTargetNamespace)
		}
	}

	return appProjectName, nil
}

func (ah *appHandler) grantProjectMonitoringRBAC(appName, appTargetNamespace string, project *mgmtv3.Project) (string, error) {
	appServiceAccountName := appName
	appClusterRoleName := fmt.Sprintf("%s-%s", appName, project.Name)
	appClusterRoleBindingName := appClusterRoleName + "-binding"
	ownedLabels := monitoring.OwnedLabels(appName, appTargetNamespace, monitoring.ProjectLevel)

	err := stream(
		// detect ServiceAccount (the name as same as App)
		func() error {
			appServiceAccount, err := ah.agentServiceAccountGetter.ServiceAccounts(appTargetNamespace).Get(appServiceAccountName, metav1.GetOptions{})
			if err != nil && !k8serrors.IsNotFound(err) {
				return errors.Wrapf(err, "failed to query %q ServiceAccount in %q Namespace", appServiceAccountName, appTargetNamespace)
			}
			if appServiceAccount.Name == appServiceAccountName {
				if appServiceAccount.DeletionTimestamp != nil {
					return errors.New(fmt.Sprintf("stale %q ServiceAccount in %q Namespace is still on terminating", appServiceAccountName, appTargetNamespace))
				}
			} else {
				appServiceAccount = &k8scorev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      appServiceAccountName,
						Namespace: appTargetNamespace,
						Labels:    ownedLabels,
					},
				}

				if _, err := ah.agentServiceAccountGetter.ServiceAccounts(appTargetNamespace).Create(appServiceAccount); err != nil && !k8serrors.IsAlreadyExists(err) {
					return errors.Wrapf(err, "failed to create %q ServiceAccount in %q Namespace", appServiceAccountName, appTargetNamespace)
				}
			}

			return nil
		},

		// detect ClusterRoleBinding (the name is ${ServiceAccountName}-binding)
		func() error {
			appClusterRoleBinding, err := ah.agentRBACClient.ClusterRoleBindings(metav1.NamespaceAll).Get(appClusterRoleBindingName, metav1.GetOptions{})
			if err != nil && !k8serrors.IsNotFound(err) {
				return errors.Wrapf(err, "failed to query %q ClusterRoleBinding", appClusterRoleBindingName)
			}
			if appClusterRoleBinding.Name == appClusterRoleBindingName {
				if appClusterRoleBinding.DeletionTimestamp != nil {
					return errors.New(fmt.Sprintf("stale %q ClusterRoleBinding is still on terminating", appClusterRoleBindingName))
				}
			} else {
				appClusterRoleBinding = &k8srbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:   appClusterRoleBindingName,
						Labels: ownedLabels,
					},
					Subjects: []k8srbacv1.Subject{
						{
							Kind:      k8srbacv1.ServiceAccountKind,
							Namespace: appTargetNamespace,
							Name:      appServiceAccountName,
						},
					},
					RoleRef: k8srbacv1.RoleRef{
						APIGroup: k8srbacv1.GroupName,
						Kind:     "ClusterRole",
						Name:     project.Name + "-namespaces-edit",
					},
				}

				if _, err := ah.agentRBACClient.ClusterRoleBindings(metav1.NamespaceAll).Create(appClusterRoleBinding); err != nil && !k8serrors.IsAlreadyExists(err) {
					return errors.Wrapf(err, "failed to create %q ClusterRoleBinding", appClusterRoleBindingName)
				}
			}

			return nil
		},
	)
	if err != nil {
		return "", err
	}

	return appServiceAccountName, nil
}

func (ah *appHandler) deployProjectMonitoring(appName, appTargetNamespace string, appServiceAccountName, appProjectName string, project *mgmtv3.Project) error {
	projectID := project.Name
	projectCreatorID := project.Annotations[monitoring.CattleCreatorIDAnnotationKey]
	overwriteMonitoringAppAnswers := project.Annotations[monitoring.CattleOverwriteMonitoringAppAnswersAnnotationKey]

	// detect App "project-monitoring"
	app, err := ah.cattleAppsGetter.Apps(projectID).Get(appName, metav1.GetOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return errors.Wrapf(err, "failed to query %q App in %s", appName, projectID)
	}
	if app.Name == appName {
		if app.DeletionTimestamp != nil {
			return errors.New(fmt.Sprintf("stale %q App in %s is still on terminating", appName, projectID))
		}

		return nil
	}

	// detect TemplateVersion "rancher-monitoring"
	catalogID := settings.SystemMonitoringCatalogID.Get()
	templateVersionID, err := common.ParseExternalID(catalogID)
	if err != nil {
		return errors.Wrapf(err, "failed to parse catalog ID %q", catalogID)
	}
	if _, err := ah.cattleTemplateVersionClient.Get(templateVersionID, metav1.GetOptions{}); err != nil {
		return errors.Wrapf(err, "failed to find catalog by ID %q", catalogID)
	}

	clusterPrometheusSvcName, clusterPrometheusSvcNamespace, clusterPrometheusPort := monitoring.ClusterPrometheusEndpoint()

	appAnswers := map[string]string{
		"grafana.enabled":                 "true",
		"grafana.apiGroup":                monitoring.APIVersion.Group,
		"grafana.serviceAccountName":      appServiceAccountName,
		"grafana.persistence.enabled":     "false",
		"grafana.level":                   "project",
		"grafana.prometheusDatasourceURL": fmt.Sprintf("http://%s.%s:%s", clusterPrometheusSvcName, clusterPrometheusSvcNamespace, clusterPrometheusPort),
	}
	var appOverwriteAnswers mgmtv3.MonitoringInput
	if len(overwriteMonitoringAppAnswers) != 0 {
		if err := json.Unmarshal([]byte(overwriteMonitoringAppAnswers), &appOverwriteAnswers); err != nil {
			return errors.Wrap(err, "unable to unmarshal input error")
		}
	}
	appAnswers = monitoring.OverwriteAppAnswers(appAnswers, appOverwriteAnswers.Answers)

	app = &projectv3.App{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				monitoring.CattleCreatorIDAnnotationKey: projectCreatorID,
			},
			Labels:    monitoring.OwnedLabels(appName, appTargetNamespace, monitoring.ProjectLevel),
			Name:      appName,
			Namespace: projectID,
		},
		Spec: projectv3.AppSpec{
			Answers:         appAnswers,
			Description:     "Rancher Project Monitoring",
			ExternalID:      catalogID,
			ProjectName:     appProjectName,
			TargetNamespace: appTargetNamespace,
		},
	}

	if _, err := ah.cattleAppsGetter.Apps(projectID).Create(app); err != nil && !k8serrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create %q App", appName)
	}

	return nil
}

func (ah *appHandler) revokeProjectMonitoringRBAC(appName, appTargetNamespace string, project *mgmtv3.Project) error {
	appServiceAccountName := appName
	appClusterRoleName := fmt.Sprintf("%s-%s", appName, project.Name)
	appClusterRoleBindingName := appClusterRoleName + "-binding"

	go func() {
		// just delete ClusterRoleBinding
		ah.agentRBACClient.ClusterRoleBindings(metav1.NamespaceAll).Delete(appClusterRoleBindingName, &metav1.DeleteOptions{})

		// just delete ClusterRole
		ah.agentRBACClient.ClusterRoles(metav1.NamespaceAll).Delete(appClusterRoleName, &metav1.DeleteOptions{})

		// just delete ServiceAccount
		ah.agentServiceAccountGetter.ServiceAccounts(appTargetNamespace).Delete(appServiceAccountName, &metav1.DeleteOptions{})
	}()

	return nil
}

func getProjectTag(project *mgmtv3.Project) string {
	return fmt.Sprintf("%s(%s)", project.Spec.DisplayName, project.Name)
}
