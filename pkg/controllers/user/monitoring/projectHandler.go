package monitoring

import (
	"context"
	"fmt"
	"reflect"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/monitoring"
	"github.com/rancher/rancher/pkg/settings"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	projectv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	k8scorev1 "k8s.io/api/core/v1"
	k8srbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type projectHandler struct {
	ctx                  context.Context
	clusterName          string
	cattleClustersClient mgmtv3.ClusterInterface
	cattleProjectsClient mgmtv3.ProjectInterface
	projectGraph         mgmtv3.ProjectMonitorGraphInterface
	app                  *appHandler
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

	err = ph.syncProjectMonitoring(cpy, ph.projectGraph)

	if !reflect.DeepEqual(cpy, src) {
		updated, updateErr := ph.cattleProjectsClient.Update(cpy)
		if updateErr != nil {
			return project, errors.Wrapf(updateErr, "failed to update Project %s in Cluster %s", projectTag, project.Spec.ClusterName)
		}

		cpy = updated
	}

	if err != nil {
		err = errors.Wrapf(err, "unable to sync Project %s in Cluster %s", projectTag, project.Spec.ClusterName)
	}

	return cpy, err
}

func (ph *projectHandler) syncProjectMonitoring(project *mgmtv3.Project, projectGraphClient mgmtv3.ProjectMonitorGraphInterface) error {
	if !mgmtv3.NamespaceBackedResource.IsTrue(project) && !mgmtv3.ProjectConditionInitialRolesPopulated.IsTrue(project) {
		return nil
	}
	_, err := mgmtv3.ProjectConditionMetricExpressionDeployed.DoUntilTrue(project, func() (runtime.Object, error) {
		projectName := fmt.Sprintf("%s:%s", project.Spec.ClusterName, project.Name)

		for _, graph := range preDefinedProjectGraph {
			newObj := graph.DeepCopy()
			newObj.Namespace = project.Name
			newObj.Spec.ProjectName = projectName
			if _, err := projectGraphClient.Create(newObj); err != nil && !apierrors.IsAlreadyExists(err) {
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
		appProjectName, err := ph.app.ensureProjectMonitoringProjectName(appTargetNamespace, project)
		if err != nil {
			mgmtv3.ProjectConditionMonitoringEnabled.Unknown(project)
			mgmtv3.ProjectConditionMonitoringEnabled.Message(project, err.Error())
			return errors.Wrap(err, "failed to ensure monitoring project name")
		}

		deployServiceAccountName, err := ph.app.grantProjectMonitoringPermissions(appName, appTargetNamespace, project)
		if err != nil {
			mgmtv3.ProjectConditionMonitoringEnabled.Unknown(project)
			mgmtv3.ProjectConditionMonitoringEnabled.Message(project, err.Error())
			return errors.Wrap(err, "failed to grant monitoring permissions")
		}

		if err := ph.app.deployProjectMonitoringApp(appName, appTargetNamespace, deployServiceAccountName, appProjectName, project); err != nil {
			mgmtv3.ProjectConditionMonitoringEnabled.Unknown(project)
			mgmtv3.ProjectConditionMonitoringEnabled.Message(project, err.Error())
			return errors.Wrap(err, "failed to deploy monitoring")
		}

		if err := ph.detectMonitoringComponentsWhileInstall(appName, appTargetNamespace, project); err != nil {
			mgmtv3.ProjectConditionMonitoringEnabled.Unknown(project)
			mgmtv3.ProjectConditionMonitoringEnabled.Message(project, err.Error())
			return errors.Wrap(err, "failed to detect the installation status of monitoring components")
		}

		mgmtv3.ProjectConditionMonitoringEnabled.True(project)
		mgmtv3.ProjectConditionMonitoringEnabled.Message(project, "")
	} else if project.Status.MonitoringStatus != nil {
		if err := ph.app.withdrawApp(appName, appTargetNamespace); err != nil {
			mgmtv3.ProjectConditionMonitoringEnabled.Unknown(project)
			mgmtv3.ProjectConditionMonitoringEnabled.Message(project, err.Error())
			return errors.Wrap(err, "failed to withdraw monitoring")
		}

		if err := ph.detectMonitoringComponentsWhileUninstall(appName, appTargetNamespace, project); err != nil {
			mgmtv3.ProjectConditionMonitoringEnabled.Unknown(project)
			mgmtv3.ProjectConditionMonitoringEnabled.Message(project, err.Error())
			return errors.Wrap(err, "failed to detect the uninstallation status of monitoring components")
		}

		appServiceAccountName := appName
		appClusterRoleName := fmt.Sprintf("%s-%s", appName, project.Name)
		appClusterRoleBindingName := appClusterRoleName + "-binding"
		if err := ph.app.revokePermissions(appServiceAccountName, appClusterRoleName, appClusterRoleBindingName, appTargetNamespace); err != nil {
			mgmtv3.ProjectConditionMonitoringEnabled.Unknown(project)
			mgmtv3.ProjectConditionMonitoringEnabled.Message(project, err.Error())
			return errors.Wrap(err, "failed to revoke monitoring permissions")
		}

		mgmtv3.ProjectConditionMonitoringEnabled.False(project)
		mgmtv3.ProjectConditionMonitoringEnabled.Message(project, "")
	}

	return nil
}

func (ph *projectHandler) detectMonitoringComponentsWhileInstall(appName, appTargetNamespace string, project *mgmtv3.Project) error {
	if project.Status.MonitoringStatus == nil {
		project.Status.MonitoringStatus = &mgmtv3.MonitoringStatus{
			Conditions: []mgmtv3.MonitoringCondition{
				{Type: mgmtv3.ClusterConditionType(ConditionGrafanaDeployed), Status: k8scorev1.ConditionFalse},
			},
		}
	}

	return isGrafanaDeployed(ph.app.agentWorkloadsClient, appTargetNamespace, appName, project.Status.MonitoringStatus, project.Spec.ClusterName)
}

func (ph *projectHandler) detectMonitoringComponentsWhileUninstall(appName, appTargetNamespace string, project *mgmtv3.Project) error {
	if project.Status.MonitoringStatus == nil {
		return nil
	}

	return isGrafanaWithdrew(ph.app.agentWorkloadsClient, appTargetNamespace, appName, project.Status.MonitoringStatus)
}

func (ah *appHandler) ensureProjectMonitoringProjectName(appTargetNamespace string, project *mgmtv3.Project) (string, error) {
	agentNamespacesClient := ah.agentCoreClient.Namespaces(metav1.NamespaceAll)
	appProjectName, err := monitoring.EnsureAppProjectName(agentNamespacesClient, project.Name, project.Spec.ClusterName, appTargetNamespace)
	if err != nil {
		return "", err
	}

	return appProjectName, nil
}

func (ah *appHandler) grantProjectMonitoringPermissions(appName, appTargetNamespace string, project *mgmtv3.Project) (string, error) {
	appServiceAccountName := appName
	appClusterRoleName := fmt.Sprintf("%s-%s", appName, project.Name)
	appClusterRoleBindingName := appClusterRoleName + "-binding"

	err := stream(
		// detect ServiceAccount (the name as same as App)
		func() error {
			appServiceAccount, err := ah.agentCoreClient.ServiceAccounts(appTargetNamespace).Get(appServiceAccountName, metav1.GetOptions{})
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
						Labels:    monitoring.OwnedLabels(appName, appTargetNamespace, monitoring.ProjectLevel),
					},
				}

				if _, err := ah.agentCoreClient.ServiceAccounts(appTargetNamespace).Create(appServiceAccount); err != nil && !k8serrors.IsAlreadyExists(err) {
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
						Labels: monitoring.OwnedLabels(appName, appTargetNamespace, monitoring.ProjectLevel),
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

func (ah *appHandler) deployProjectMonitoringApp(appName, appTargetNamespace string, appServiceAccountName, appProjectName string, project *mgmtv3.Project) error {
	clusterPrometheusSvcName, clusterPrometheusSvcNamespace, clusterPrometheusPort := monitoring.ClusterPrometheusEndpoint()
	appAnswers := map[string]string{
		"grafana.enabled":                 "true",
		"grafana.apiGroup":                monitoring.APIVersion.Group,
		"grafana.serviceAccountName":      appServiceAccountName,
		"grafana.persistence.enabled":     "false",
		"grafana.level":                   "project",
		"grafana.prometheusDatasourceURL": fmt.Sprintf("http://%s.%s:%s", clusterPrometheusSvcName, clusterPrometheusSvcNamespace, clusterPrometheusPort),
	}
	appAnswers = monitoring.OverwriteAppAnswers(appAnswers, project.Annotations)

	appCatalogID := settings.SystemMonitoringCatalogID.Get()
	appDeployProjectID := project.Name
	app := &projectv3.App{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: monitoring.CopyCreatorID(nil, project.Annotations),
			Labels:      monitoring.OwnedLabels(appName, appTargetNamespace, monitoring.ProjectLevel),
			Name:        appName,
			Namespace:   appDeployProjectID,
		},
		Spec: projectv3.AppSpec{
			Answers:         appAnswers,
			Description:     "Rancher Project Monitoring",
			ExternalID:      appCatalogID,
			ProjectName:     appProjectName,
			TargetNamespace: appTargetNamespace,
		},
	}

	err := monitoring.DeployApp(ah.cattleAppsGetter, appDeployProjectID, app)
	if err != nil {
		return err
	}

	return nil
}

func getProjectTag(project *mgmtv3.Project) string {
	return fmt.Sprintf("%s(%s)", project.Spec.DisplayName, project.Name)
}
