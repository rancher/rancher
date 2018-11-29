package clusterdeploy

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/store/crd"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/user/helm/common"
	"github.com/rancher/rancher/pkg/image"
	"github.com/rancher/rancher/pkg/kubectl"
	"github.com/rancher/rancher/pkg/monitoring"
	"github.com/rancher/rancher/pkg/project"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/systemtemplate"
	corev1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	projectv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	rrbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	projectv3client "github.com/rancher/types/client/project/v3"
	"github.com/rancher/types/config"
	"github.com/rancher/types/user"
	"github.com/sirupsen/logrus"
	k8scorev1 "k8s.io/api/core/v1"
	k8srbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func Register(ctx context.Context, management *config.ManagementContext, clusterManager *clustermanager.Manager) {
	c := &clusterDeploy{
		ctx:                  ctx,
		systemAccountManager: systemaccount.NewManager(management),
		userManager:          management.UserManager,
		clusters:             management.Management.Clusters(""),
		nodeLister:           management.Management.Nodes("").Controller().Lister(),
		templateVersions:     management.Management.TemplateVersions(""),
		clusterManager:       clusterManager,
		projectsGetter:       management.Management.(v3.ProjectsGetter),
		appsGetter:           management.Project.(projectv3.AppsGetter),
		clusterroles:         management.RBAC.ClusterRoles(""),
	}

	management.Management.Clusters("").AddHandler(ctx, "cluster-deploy", c.sync)
}

type clusterDeploy struct {
	ctx                  context.Context
	systemAccountManager *systemaccount.Manager
	userManager          user.Manager
	clusters             v3.ClusterInterface
	templateVersions     v3.TemplateVersionInterface
	clusterManager       *clustermanager.Manager
	nodeLister           v3.NodeLister
	projectsGetter       v3.ProjectsGetter
	appsGetter           projectv3.AppsGetter
	clusterroles         rrbacv1.ClusterRoleInterface
}

func (cd *clusterDeploy) sync(key string, cluster *v3.Cluster) (runtime.Object, error) {
	var (
		err, updateErr error
	)

	if key == "" || cluster == nil {
		return nil, nil
	}

	original := cluster
	cluster = original.DeepCopy()

	err = cd.doSync(cluster)
	if cluster != nil && !reflect.DeepEqual(cluster, original) {
		_, updateErr = cd.clusters.Update(cluster)
	}

	if err != nil {
		return nil, err
	}
	return nil, updateErr
}

func (cd *clusterDeploy) doSync(cluster *v3.Cluster) error {
	if !v3.ClusterConditionProvisioned.IsTrue(cluster) {
		return nil
	}

	nodes, err := cd.nodeLister.List(cluster.Name, labels.Everything())
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return nil
	}

	_, err = v3.ClusterConditionSystemAccountCreated.DoUntilTrue(cluster, func() (runtime.Object, error) {
		return cluster, cd.systemAccountManager.CreateSystemAccount(cluster)
	})
	if err != nil {
		return err
	}
	err = cd.deployAgent(cluster)
	if err != nil {
		return err
	}
	_, err = cd.deployPrometheusOperator(cluster)
	if err != nil {
		return err
	}

	return cd.setNetworkPolicyAnn(cluster)
}

func (cd *clusterDeploy) deployAgent(cluster *v3.Cluster) error {
	desired := cluster.Spec.DesiredAgentImage
	if desired == "" || desired == "fixed" {
		desired = image.Resolve(settings.AgentImage.Get())
	}

	if cluster.Status.AgentImage == desired {
		return nil
	}

	kubeConfig, err := cd.getKubeConfig(cluster)
	if err != nil {
		return err
	}

	_, err = v3.ClusterConditionAgentDeployed.Do(cluster, func() (runtime.Object, error) {
		yaml, err := cd.getYAML(cluster, desired)
		if err != nil {
			return cluster, err
		}

		var output []byte
		for i := 0; i < 3; i++ {
			// This will fail almost always the first time because when we create the namespace in the file
			// it won't have privileges.  Just stupidly try 3 times
			output, err = kubectl.Apply(yaml, kubeConfig)
			if err == nil {
				break
			}
			time.Sleep(2 * time.Second)
		}
		if err != nil {
			return cluster, types.NewErrors(err, errors.New(string(output)))
		}
		v3.ClusterConditionAgentDeployed.Message(cluster, string(output))
		return cluster, nil
	})
	if err != nil {
		return err
	}

	if err == nil {
		cluster.Status.AgentImage = desired
		if cluster.Spec.DesiredAgentImage == "fixed" {
			cluster.Spec.DesiredAgentImage = desired
		}
	}

	return err
}

func (cd *clusterDeploy) setNetworkPolicyAnn(cluster *v3.Cluster) error {
	if cluster.Spec.EnableNetworkPolicy != nil {
		return nil
	}
	// set current state for upgraded canal clusters
	if cluster.Spec.RancherKubernetesEngineConfig != nil &&
		cluster.Spec.RancherKubernetesEngineConfig.Network.Plugin == "canal" {
		enableNetworkPolicy := true
		cluster.Spec.EnableNetworkPolicy = &enableNetworkPolicy
		cluster.Annotations["networking.management.cattle.io/enable-network-policy"] = "true"
	}
	return nil
}

func (cd *clusterDeploy) getKubeConfig(cluster *v3.Cluster) (*clientcmdapi.Config, error) {
	user, err := cd.systemAccountManager.GetSystemUser(cluster)
	if err != nil {
		return nil, err
	}

	token, err := cd.userManager.EnsureToken("agent-"+user.Name, "token for agent deployment", user.Name)
	if err != nil {
		return nil, err
	}

	return cd.clusterManager.KubeConfig(cluster.Name, token), nil
}

func (cd *clusterDeploy) getYAML(cluster *v3.Cluster, agentImage string) ([]byte, error) {
	token, err := cd.systemAccountManager.GetOrCreateSystemClusterToken(cluster.Name)
	if err != nil {
		return nil, err
	}

	url := settings.ServerURL.Get()
	if url == "" {
		return nil, fmt.Errorf("waiting for server-url setting to be set")
	}

	buf := &bytes.Buffer{}
	err = systemtemplate.SystemTemplate(buf, agentImage, token, url)

	return buf.Bytes(), err
}

func (cd *clusterDeploy) deployPrometheusOperator(cluster *v3.Cluster) (string, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		logrus.Warnf("cluster %s is deleted", cluster.Name)
		return "", nil
	}
	clusterTag := fmt.Sprintf("%s(%s)", cluster.Spec.DisplayName, cluster.Name)
	appName, appTargetNamespace := monitoring.SystemMonitoringInfo()

	kubeConfig, err := clustermanager.ToRESTConfig(cluster, cd.clusterManager.ScaledContext)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create RESTConfig for cluster %s", clusterTag)
	}

	var appProjectName string
	_, err = v3.ClusterConditionPrometheusOperatorDeployed.Do(cluster, func() (runtime.Object, error) {
		factory, err := crd.NewFactoryFromClient(*kubeConfig)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create CRD factory for cluster %s", clusterTag)
		}

		// create Prometheus Operator CRD
		factory.BatchCreateCRDs(cd.ctx, config.UserStorageContext, cd.clusterManager.ScaledContext.Schemas, &monitoring.APIVersion,
			projectv3client.PrometheusType,
			projectv3client.PrometheusRuleType,
			projectv3client.AlertmanagerType,
			projectv3client.ServiceMonitorType,
		)

		factory.BatchWait()

		// deploy Prometheus Operator
		agentContext, err := cd.clusterManager.UserContext(cluster.Name)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create UserContext for cluster %s", clusterTag)
		}

		appProjectName, err = ensureMonitoringProjectName(agentContext.Core.Namespaces(metav1.NamespaceAll), cd.projectsGetter.Projects(cluster.Name), cluster.Name, appTargetNamespace)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to ensure monitoring project name for Cluster %s", clusterTag)
		}

		appServiceAccountName, err := grantSystemMonitoringRBAC(agentContext.Core.(corev1.ServiceAccountsGetter), agentContext.RBAC, appName, appTargetNamespace)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to grant prometheus operator RBAC for Cluster %s", clusterTag)
		}

		err = deploySystemMonitoring(cd.appsGetter, cd.templateVersions, cluster.Annotations[monitoring.CattleCreatorIDAnnotationKey], appProjectName, appName, appTargetNamespace, appServiceAccountName)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to ensure prometheus operator app for Cluster %s", clusterTag)
		}

		return cluster, nil
	})
	if err != nil {
		return "", err
	}

	return appProjectName, nil
}

func ensureMonitoringProjectName(agentNamespacesClient corev1.NamespaceInterface, cattleProjectsClient v3.ProjectInterface, clusterName, appTargetNamespace string) (string, error) {
	// detect Namespace
	deployNamespace, err := agentNamespacesClient.Get(appTargetNamespace, metav1.GetOptions{})
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

		if deployNamespace, err = agentNamespacesClient.Create(deployNamespace); err != nil && !k8serrors.IsAlreadyExists(err) {
			return "", errors.Wrapf(err, "failed to create %q Namespace", appTargetNamespace)
		}
	}

	appProjectName := ""
	if projectName, ok := deployNamespace.Annotations[monitoring.CattleProjectIDAnnotationKey]; ok {
		appProjectName = projectName
	}
	if len(appProjectName) == 0 {
		// fetch all system Projects
		defaultSystemProjects, _ := cattleProjectsClient.List(metav1.ListOptions{
			LabelSelector: "authz.management.cattle.io/system-project=true",
		})

		// get "System" system Project
		var deployProject *v3.Project
		defaultSystemProjects = defaultSystemProjects.DeepCopy()
		for _, defaultProject := range defaultSystemProjects.Items {
			deployProject = &defaultProject

			if defaultProject.Spec.DisplayName == project.System {
				break
			}
		}
		if deployProject == nil {
			return "", errors.New(fmt.Sprintf("failed to find any cattle system project"))
		}

		// move Monitoring Namespace to System Project
		appProjectName = fmt.Sprintf("%s:%s", clusterName, deployProject.Name)
		if deployNamespace.Annotations == nil {
			deployNamespace.Annotations = make(map[string]string, 1)
		}
		deployNamespace.Annotations[monitoring.CattleProjectIDAnnotationKey] = appProjectName

		_, err := agentNamespacesClient.Update(deployNamespace)
		if err != nil {
			return "", errors.Wrapf(err, "failed to move Namespace %s to Project %s", appTargetNamespace, deployProject.Spec.DisplayName)
		}
	}

	return appProjectName, nil
}

func grantSystemMonitoringRBAC(agentServiceAccountGetter corev1.ServiceAccountsGetter, agentRBACClient rrbacv1.Interface, appName, appTargetNamespace string) (string, error) {
	appServiceAccountName := appName
	appClusterRoleName := appServiceAccountName
	appClusterRoleBindingName := appServiceAccountName + "-binding"
	ownedLabels := monitoring.OwnedLabels(appName, appTargetNamespace, monitoring.SystemLevel)

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
					APIGroups: []string{monitoring.APIVersion.Group},
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

func deploySystemMonitoring(cattleAppsGetter projectv3.AppsGetter, cattleTemplateVersionsClient v3.TemplateVersionInterface, clusterCreatorID, appProjectName string, appName, appTargetNamespace, appServiceAccountName string) error {
	_, projectID := ref.Parse(appProjectName)

	// detect App "system-monitoring"
	app, err := cattleAppsGetter.Apps(projectID).Get(appName, metav1.GetOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return errors.Wrapf(err, "failed to query %q App in %s Project", appName, projectID)
	}
	if app.Name == appName {
		if app.DeletionTimestamp != nil {
			return errors.New(fmt.Sprintf("stale %q App in %s Project is still on terminating", appName, projectID))
		}

		return nil
	}

	// detect TemplateVersion "rancher-monitoring"
	appCatalogID := settings.SystemMonitoringCatalogID.Get()
	templateVersionID, err := common.ParseExternalID(appCatalogID)
	if err != nil {
		return errors.Wrapf(err, "failed to parse catalog ID %q", appCatalogID)
	}
	for {
		if _, err := cattleTemplateVersionsClient.Get(templateVersionID, metav1.GetOptions{}); err != nil {
			time.Sleep(10 * time.Second)
			logrus.Warnf("failed to find catalog by ID %q, %v", appCatalogID, err)
		} else {
			break
		}
	}

	// create App "system-monitoring"
	app = &projectv3.App{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				monitoring.CattleCreatorIDAnnotationKey: clusterCreatorID,
			},
			Labels:    monitoring.OwnedLabels(appName, appTargetNamespace, monitoring.SystemLevel),
			Name:      appName,
			Namespace: projectID,
		},
		Spec: projectv3.AppSpec{
			Answers: map[string]string{
				"enabled":            "true",
				"apiGroup":           monitoring.APIVersion.Group,
				"nameOverride":       "prometheus-operator",
				"serviceAccountName": appServiceAccountName,
			},
			Description:     "Prometheus Operator for Rancher Monitoring",
			ExternalID:      appCatalogID,
			ProjectName:     appProjectName,
			TargetNamespace: appTargetNamespace,
		},
	}

	if _, err := cattleAppsGetter.Apps(projectID).Create(app); err != nil && !k8serrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create %q App", appName)
	}

	return nil
}
