package monitoring

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/settings"
	corev1 "github.com/rancher/types/apis/core/v1"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	projectv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	rbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/sirupsen/logrus"
	k8scorev1 "k8s.io/api/core/v1"
	k8srbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func SyncServiceMonitor(cluster *mgmtv3.Cluster, agentCoreClient corev1.Interface, agentRBACClient rbacv1.Interface, cattleAppsGetter projectv3.AppsGetter, cattleProjectsGetter mgmtv3.ProjectsGetter, cattleTemplateVersionsClient mgmtv3.TemplateVersionInterface) error {
	if cluster.Spec.EnableClusterMonitoring || cluster.Spec.EnableClusterAlerting {
		err := DeploySystemMonitor(cluster, agentCoreClient, agentRBACClient, cattleAppsGetter, cattleProjectsGetter, cattleTemplateVersionsClient)
		if err != nil {
			return errors.Wrap(err, "failed to deploy system monitoring")
		}
	} else if !cluster.Spec.EnableClusterMonitoring && !cluster.Spec.EnableClusterAlerting {
		err := WithdrawSystemMonitor(cluster, agentCoreClient, agentRBACClient, cattleAppsGetter)
		if err != nil {
			return errors.Wrap(err, "failed to withdraw system monitoring")
		}
	}
	return nil
}

func DeploySystemMonitor(cluster *mgmtv3.Cluster, agentCoreClient corev1.Interface, agentRBACClient rbacv1.Interface, cattleAppsGetter projectv3.AppsGetter, cattleProjectsGetter mgmtv3.ProjectsGetter, cattleTemplateVersionsClient mgmtv3.TemplateVersionInterface) (backErr error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		logrus.Warnf("cluster %s is deleted", cluster.Name)
		return nil
	}
	appName, appTargetNamespace := SystemMonitoringInfo()

	_, backErr = mgmtv3.ClusterConditionPrometheusOperatorDeployed.Do(cluster, func() (runtime.Object, error) {
		appCatalogID := settings.SystemMonitoringCatalogID.Get()
		err := DetectAppCatalogExistence(appCatalogID, cattleTemplateVersionsClient)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to ensure catalog %q", appCatalogID)
		}

		appDeployProjectID, err := GetSystemProjectID(cattleProjectsGetter.Projects(cluster.Name))
		if err != nil {
			return nil, errors.Wrap(err, "failed to get System Project ID")
		}

		appProjectName, err := EnsureAppProjectName(agentCoreClient.Namespaces(metav1.NamespaceAll), appDeployProjectID, cluster.Name, appTargetNamespace)
		if err != nil {
			return nil, errors.Wrap(err, "failed to ensure monitoring project name")
		}

		appServiceAccountName, err := grantSystemMonitorRBAC(agentCoreClient.(corev1.ServiceAccountsGetter), agentRBACClient, appName, appTargetNamespace)
		if err != nil {
			return nil, errors.Wrap(err, "failed to grant monitoring permissions")
		}

		appAnswers := map[string]string{
			"enabled":            "true",
			"apiGroup":           APIVersion.Group,
			"nameOverride":       "prometheus-operator",
			"serviceAccountName": appServiceAccountName,
		}
		app := &projectv3.App{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: CopyCreatorID(nil, cluster.Annotations),
				Labels:      OwnedLabels(appName, appTargetNamespace, SystemLevel),
				Name:        appName,
			},
			Spec: projectv3.AppSpec{
				Answers:         appAnswers,
				Description:     "Prometheus Operator for Rancher Monitoring",
				ExternalID:      appCatalogID,
				ProjectName:     appProjectName,
				TargetNamespace: appTargetNamespace,
			},
		}

		err = DeployApp(cattleAppsGetter, appDeployProjectID, app)
		if err != nil {
			return nil, errors.Wrap(err, "failed to ensure prometheus operator app")
		}

		return cluster, nil
	})
	if backErr != nil {
		return backErr
	}

	return nil
}

func grantSystemMonitorRBAC(agentServiceAccountGetter corev1.ServiceAccountsGetter, agentRBACClient rbacv1.Interface, appName, appTargetNamespace string) (string, error) {
	appServiceAccountName := appName
	appClusterRoleName := appServiceAccountName
	appClusterRoleBindingName := appServiceAccountName + "-binding"
	ownedLabels := OwnedLabels(appName, appTargetNamespace, SystemLevel)

	err := utilerrors.AggregateGoroutines(
		// detect ServiceAccount (the name as same as App)
		func() error {
			appServiceAccount, err := agentServiceAccountGetter.ServiceAccounts(appTargetNamespace).Get(appServiceAccountName, metav1.GetOptions{})
			if err != nil && !k8serrors.IsNotFound(err) {
				return errors.Wrapf(err, "failed to query %q ServiceAccount", appServiceAccountName)
			}
			if appServiceAccount.Name == appServiceAccountName {
				if appServiceAccount.DeletionTimestamp != nil {
					return errors.New(fmt.Sprintf("stale %q ServiceAccount is still on terminating", appServiceAccountName))
				}
			} else {
				appServiceAccount = &k8scorev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      appServiceAccountName,
						Namespace: appTargetNamespace,
						Labels:    ownedLabels,
					},
				}

				if _, err := agentServiceAccountGetter.ServiceAccounts(appTargetNamespace).Create(appServiceAccount); err != nil && !k8serrors.IsAlreadyExists(err) {
					return errors.Wrapf(err, "failed to create %q ServiceAccount", appServiceAccountName)
				}
			}

			return nil
		},

		// detect ClusterRole (the name as same as App)
		func() error {
			appClusterRole, err := agentRBACClient.ClusterRoles(metav1.NamespaceAll).Get(appClusterRoleName, metav1.GetOptions{})
			if err != nil && !k8serrors.IsNotFound(err) {
				return errors.Wrapf(err, "failed to query %q ClusterRole", appClusterRoleName)
			}

			rules := []k8srbacv1.PolicyRule{
				{
					APIGroups: []string{APIVersion.Group},
					Resources: []string{"alertmanager", "alertmanagers", "prometheus", "prometheuses", "service-monitor", "servicemonitors", "prometheusrules", "prometheuses/finalizers", "alertmanagers/finalizers"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{"apps"},
					Resources: []string{"statefulsets"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"configmaps", "secrets"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"pods"},
					Verbs:     []string{"list", "delete"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"services", "endpoints"},
					Verbs:     []string{"get", "create", "update"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"nodes", "namespaces"},
					Verbs:     []string{"list", "watch"},
				},
			}

			if appClusterRole.Name == appClusterRoleName {
				if appClusterRole.DeletionTimestamp != nil {
					return errors.New(fmt.Sprintf("stale %q ClusterRole is still on terminating", appClusterRoleName))
				}

				// ensure
				appClusterRole = appClusterRole.DeepCopy()
				appClusterRole.Rules = rules
				if _, err := agentRBACClient.ClusterRoles(metav1.NamespaceAll).Update(appClusterRole); err != nil {
					return errors.Wrapf(err, "failed to update %q ClusterRole", appClusterRoleName)
				}
			} else {
				appClusterRole = &k8srbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name:   appClusterRoleName,
						Labels: ownedLabels,
					},
					Rules: rules,
				}

				if _, err := agentRBACClient.ClusterRoles(metav1.NamespaceAll).Create(appClusterRole); err != nil && !k8serrors.IsAlreadyExists(err) {
					return errors.Wrapf(err, "failed to create %q ClusterRole", appClusterRoleName)
				}
			}

			return nil
		},

		// detect ClusterRoleBinding (the name is ${ServiceAccountName}-binding)
		func() error {
			appClusterRoleBinding, err := agentRBACClient.ClusterRoleBindings(metav1.NamespaceAll).Get(appClusterRoleBindingName, metav1.GetOptions{})
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
						Name:     appClusterRoleName,
					},
				}

				if _, err := agentRBACClient.ClusterRoleBindings(metav1.NamespaceAll).Create(appClusterRoleBinding); err != nil && !k8serrors.IsAlreadyExists(err) {
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

func WithdrawSystemMonitor(cluster *mgmtv3.Cluster, agentCoreClient corev1.Interface, agentRBACClient rbacv1.Interface, cattleAppsGetter projectv3.AppsGetter) error {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		logrus.Warnf("cluster %s is deleted", cluster.Name)
		return nil
	}
	appName, appTargetNamespace := SystemMonitoringInfo()

	if !mgmtv3.ClusterConditionPrometheusOperatorDeployed.IsFalse(cluster) {
		if err := WithdrawApp(cattleAppsGetter, OwnedAppListOptions(appName, appTargetNamespace)); err != nil {
			mgmtv3.ClusterConditionPrometheusOperatorDeployed.Unknown(cluster)
			mgmtv3.ClusterConditionPrometheusOperatorDeployed.Message(cluster, err.Error())
			return errors.Wrap(err, "failed to withdraw prometheus operator app")
		}

		appServiceAccountName := appName
		appClusterRoleName := appName
		appClusterRoleBindingName := appClusterRoleName + "-binding"
		if err := RevokePermissions(agentRBACClient, agentCoreClient, appServiceAccountName, appClusterRoleName, appClusterRoleBindingName, appTargetNamespace); err != nil {
			mgmtv3.ClusterConditionPrometheusOperatorDeployed.Unknown(cluster)
			mgmtv3.ClusterConditionPrometheusOperatorDeployed.Message(cluster, err.Error())
			return errors.Wrap(err, "failed to revoke monitoring permissions")
		}
		mgmtv3.ClusterConditionPrometheusOperatorDeployed.False(cluster)
		mgmtv3.ClusterConditionPrometheusOperatorDeployed.Message(cluster, "")
	}

	return nil
}
