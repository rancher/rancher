package deployer

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v33 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"

	"github.com/rancher/norman/controller"
	"github.com/rancher/rancher/pkg/catalog/manager"
	alertutil "github.com/rancher/rancher/pkg/controllers/managementuserlegacy/alert/common"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/helm/common"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	projectv3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	monitorutil "github.com/rancher/rancher/pkg/monitoring"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/types/config"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	initVersion = "initializing"
)

var (
	ServiceName             = "alerting"
	waitCatalogSyncInterval = 60 * time.Second
)

const (
	defaultGroupIntervalSeconds = 180
)

type AlertService struct {
	clusterName           string
	clusterLister         v3.ClusterLister
	catalogLister         v3.CatalogLister
	catalogManager        manager.CatalogManager
	apps                  projectv3.AppInterface
	appLister             projectv3.AppLister
	oldClusterAlerts      v3.ClusterAlertInterface
	oldProjectAlerts      v3.ProjectAlertInterface
	oldProjectAlertLister v3.ProjectAlertLister
	clusterAlertGroups    v3.ClusterAlertGroupInterface
	projectAlertGroups    v3.ProjectAlertGroupInterface
	clusterAlertRules     v3.ClusterAlertRuleInterface
	projectAlertRules     v3.ProjectAlertRuleInterface
	projectLister         v3.ProjectLister
	namespaces            v1.NamespaceInterface
	templateLister        v3.CatalogTemplateLister
}

func NewService() *AlertService {
	return &AlertService{}
}

func (l *AlertService) Init(cluster *config.UserContext) {
	l.clusterName = cluster.ClusterName
	l.clusterLister = cluster.Management.Management.Clusters("").Controller().Lister()
	l.catalogLister = cluster.Management.Management.Catalogs(metav1.NamespaceAll).Controller().Lister()
	l.oldClusterAlerts = cluster.Management.Management.ClusterAlerts(cluster.ClusterName)
	l.oldProjectAlerts = cluster.Management.Management.ProjectAlerts(metav1.NamespaceAll)
	l.oldProjectAlertLister = cluster.Management.Management.ProjectAlerts("").Controller().Lister()
	l.clusterAlertGroups = cluster.Management.Management.ClusterAlertGroups(cluster.ClusterName)
	l.projectAlertGroups = cluster.Management.Management.ProjectAlertGroups(metav1.NamespaceAll)
	l.clusterAlertRules = cluster.Management.Management.ClusterAlertRules(cluster.ClusterName)
	l.projectAlertRules = cluster.Management.Management.ProjectAlertRules(metav1.NamespaceAll)
	l.projectLister = cluster.Management.Management.Projects(cluster.ClusterName).Controller().Lister()
	l.apps = cluster.Management.Project.Apps(metav1.NamespaceAll)
	l.appLister = cluster.Management.Project.Apps("").Controller().Lister()
	l.namespaces = cluster.Core.Namespaces(metav1.NamespaceAll)
	l.templateLister = cluster.Management.Management.CatalogTemplates(metav1.NamespaceAll).Controller().Lister()
	l.catalogManager = cluster.Management.CatalogManager
}

func (l *AlertService) Version() (string, error) {
	return fmt.Sprintf("%s-%s", monitorutil.RancherMonitoringTemplateName, initVersion), nil
}

func (l *AlertService) Upgrade(currentVersion string) (string, error) {
	template, err := l.templateLister.Get(namespace.GlobalNamespace, monitorutil.RancherMonitoringTemplateName)
	if err != nil {
		return "", fmt.Errorf("get template %s:%s failed, %v", namespace.GlobalNamespace, monitorutil.RancherMonitoringTemplateName, err)
	}

	templateVersion, err := l.catalogManager.LatestAvailableTemplateVersion(template, l.clusterName)
	if err != nil {
		return "", err
	}

	systemCatalogName := template.Spec.CatalogID
	newExternalID := templateVersion.ExternalID

	newVersion, _, err := common.ParseExternalID(newExternalID)
	if err != nil {
		return "", err
	}

	appName, _ := monitorutil.ClusterAlertManagerInfo()
	//migrate legacy
	if !strings.Contains(currentVersion, monitorutil.RancherMonitoringTemplateName) {
		if err := l.migrateLegacyClusterAlert(); err != nil {
			return "", err
		}

		if err := l.migrateLegacyProjectAlert(); err != nil {
			return "", err
		}

		if err := l.removeLegacyAlerting(); err != nil {
			return "", err
		}
	}

	//remove finalizer from legacy ProjectAlert
	if err := l.removeFinalizerFromLegacyAlerting(); err != nil {
		return "", err
	}

	//upgrade old app
	defaultSystemProjects, err := l.projectLister.List(metav1.NamespaceAll, labels.Set(systemProjectLabel).AsSelector())
	if err != nil {
		return "", fmt.Errorf("list system project failed, %v", err)
	}

	if len(defaultSystemProjects) == 0 {
		return "", fmt.Errorf("get system project failed")
	}

	systemProject := defaultSystemProjects[0]
	if systemProject == nil {
		return "", fmt.Errorf("get system project failed")
	}
	app, err := l.appLister.Get(systemProject.Name, appName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return newVersion, nil
		}
		return "", fmt.Errorf("get app %s:%s failed, %v", systemProject.Name, appName, err)
	}
	newApp := app.DeepCopy()
	newApp.Spec.ExternalID = newExternalID
	if newApp.Spec.Answers == nil {
		newApp.Spec.Answers = make(map[string]string)
	}
	newApp.Spec.Answers["operator.enabled"] = "false"

	if !reflect.DeepEqual(newApp, app) {
		// check cluster ready before upgrade, because helm will not retry if got cluster not ready error
		cluster, err := l.clusterLister.Get(metav1.NamespaceAll, l.clusterName)
		if err != nil {
			return "", fmt.Errorf("get cluster %s failed, %v", l.clusterName, err)
		}
		if !v32.ClusterConditionReady.IsTrue(cluster) {
			return "", fmt.Errorf("cluster %v not ready", l.clusterName)
		}

		systemCatalog, err := l.catalogLister.Get(metav1.NamespaceAll, systemCatalogName)
		if err != nil {
			return "", fmt.Errorf("get catalog %s failed, %v", systemCatalogName, err)
		}

		if !v32.CatalogConditionUpgraded.IsTrue(systemCatalog) || !v32.CatalogConditionRefreshed.IsTrue(systemCatalog) || !v32.CatalogConditionDiskCached.IsTrue(systemCatalog) {
			return "", fmt.Errorf("catalog %v not ready", systemCatalogName)
		}

		// add force upgrade to handle chart compatibility in different version
		v33.AppConditionForceUpgrade.Unknown(newApp)

		if _, err = l.apps.Update(newApp); err != nil {
			return "", fmt.Errorf("update app %s:%s failed, %v", app.Namespace, app.Name, err)
		}
	}
	return newVersion, nil
}

func (l *AlertService) migrateLegacyClusterAlert() error {
	oldClusterAlert, err := l.oldClusterAlerts.List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("get old cluster alert failed, %s", err)
	}
	for _, v := range oldClusterAlert.Items {
		migrationGroupName := fmt.Sprintf("migrate-group-%s", v.Name)
		groupID := alertutil.GetGroupID(l.clusterName, migrationGroupName)

		name := fmt.Sprintf("migrate-%s", v.Name)
		newClusterRule := &v3.ClusterAlertRule{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: l.clusterName,
			},
			Spec: v32.ClusterAlertRuleSpec{
				ClusterName: l.clusterName,
				GroupName:   groupID,
				CommonRuleField: v32.CommonRuleField{
					DisplayName: v.Spec.DisplayName,
					Severity:    v.Spec.Severity,
					TimingField: v32.TimingField{
						GroupWaitSeconds:      v.Spec.InitialWaitSeconds,
						GroupIntervalSeconds:  defaultGroupIntervalSeconds,
						RepeatIntervalSeconds: v.Spec.RepeatIntervalSeconds,
					},
				},
			},
		}

		if v.Spec.TargetNode != nil {
			newClusterRule.Spec.NodeRule = &v32.NodeRule{
				NodeName:     v.Spec.TargetNode.NodeName,
				Selector:     v.Spec.TargetNode.Selector,
				Condition:    v.Spec.TargetNode.Condition,
				MemThreshold: v.Spec.TargetNode.MemThreshold,
				CPUThreshold: v.Spec.TargetNode.CPUThreshold,
			}
		}

		if v.Spec.TargetEvent != nil {
			newClusterRule.Spec.EventRule = &v32.EventRule{
				EventType:    v.Spec.TargetEvent.EventType,
				ResourceKind: v.Spec.TargetEvent.ResourceKind,
			}
		}

		if v.Spec.TargetSystemService != nil {
			newClusterRule.Spec.SystemServiceRule = &v32.SystemServiceRule{
				Condition: v.Spec.TargetSystemService.Condition,
			}
		}

		oldClusterRule, err := l.clusterAlertRules.Get(newClusterRule.Name, metav1.GetOptions{})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("migrate %s:%s failed, get alert rule failed, %v", v.Namespace, v.Name, err)
			}

			if _, err = l.clusterAlertRules.Create(newClusterRule); err != nil && !apierrors.IsAlreadyExists(err) {
				return fmt.Errorf("migrate %s:%s failed, create alert rule failed, %v", v.Namespace, v.Name, err)
			}
		} else {
			updatedClusterRule := oldClusterRule.DeepCopy()
			updatedClusterRule.Spec = newClusterRule.Spec
			if _, err := l.clusterAlertRules.Update(updatedClusterRule); err != nil {
				return fmt.Errorf("migrate %s:%s failed, update alert rule failed, %v", v.Namespace, v.Name, err)
			}
		}
		legacyGroup := &v32.ClusterAlertGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      migrationGroupName,
				Namespace: l.clusterName,
			},
			Spec: v32.ClusterGroupSpec{
				ClusterName: l.clusterName,
				CommonGroupField: v32.CommonGroupField{
					DisplayName: "Migrate group",
					Description: "Migrate alert from last version",
					TimingField: v32.TimingField{
						GroupWaitSeconds:      v.Spec.InitialWaitSeconds,
						GroupIntervalSeconds:  defaultGroupIntervalSeconds,
						RepeatIntervalSeconds: v.Spec.RepeatIntervalSeconds,
					},
				},
				Recipients: v.Spec.Recipients,
			},
		}

		_, err = l.clusterAlertGroups.Create(legacyGroup)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("migrate failed, create alert group %s:%s failed, %v", l.clusterName, migrationGroupName, err)
		}
	}
	return nil
}

func (l *AlertService) migrateLegacyProjectAlert() error {
	oldProjectAlert, err := l.oldProjectAlertLister.List("", labels.NewSelector())
	if err != nil {
		return fmt.Errorf("get old project alert failed, %s", err)
	}

	oldProjectAlertGroup := make(map[string][]*v3.ProjectAlert)
	for _, v := range oldProjectAlert {
		if controller.ObjectInCluster(l.clusterName, v) {
			oldProjectAlertGroup[v.Spec.ProjectName] = append(oldProjectAlertGroup[v.Spec.ProjectName], v)
		}
	}

	for projectID, oldAlerts := range oldProjectAlertGroup {
		_, projectName := ref.Parse(projectID)

		for _, v := range oldAlerts {
			migrationGroupName := fmt.Sprintf("migrate-group-%s", v.Name)
			groupID := alertutil.GetGroupID(projectName, migrationGroupName)

			migrationRuleName := fmt.Sprintf("migrate-rule-%s", v.Name)
			newProjectRule := &v3.ProjectAlertRule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      migrationRuleName,
					Namespace: projectName,
				},
				Spec: v32.ProjectAlertRuleSpec{
					ProjectName: projectID,
					GroupName:   groupID,
					CommonRuleField: v32.CommonRuleField{
						DisplayName: v.Spec.DisplayName,
						Severity:    v.Spec.Severity,
						TimingField: v32.TimingField{
							GroupWaitSeconds:      v.Spec.InitialWaitSeconds,
							GroupIntervalSeconds:  defaultGroupIntervalSeconds,
							RepeatIntervalSeconds: v.Spec.RepeatIntervalSeconds,
						},
					},
				},
			}

			if v.Spec.TargetPod != nil {
				newProjectRule.Spec.PodRule = &v32.PodRule{
					PodName:                v.Spec.TargetPod.PodName,
					Condition:              v.Spec.TargetPod.Condition,
					RestartTimes:           v.Spec.TargetPod.RestartTimes,
					RestartIntervalSeconds: v.Spec.TargetPod.RestartIntervalSeconds,
				}
			}

			if v.Spec.TargetWorkload != nil {
				newProjectRule.Spec.WorkloadRule = &v32.WorkloadRule{
					WorkloadID:          v.Spec.TargetWorkload.WorkloadID,
					Selector:            v.Spec.TargetWorkload.Selector,
					AvailablePercentage: v.Spec.TargetWorkload.AvailablePercentage,
				}
			}

			oldProjectRule, err := l.projectAlertRules.GetNamespaced(projectName, newProjectRule.Name, metav1.GetOptions{})
			if err != nil {
				if !apierrors.IsNotFound(err) {
					return fmt.Errorf("migrate %s:%s failed, get alert rule failed, %v", v.Namespace, v.Name, err)
				}

				if _, err = l.projectAlertRules.Create(newProjectRule); err != nil && !apierrors.IsAlreadyExists(err) {
					return fmt.Errorf("migrate %s:%s failed, create alert rule failed, %v", v.Namespace, v.Name, err)
				}
			} else {
				updatedProjectRule := oldProjectRule.DeepCopy()
				updatedProjectRule.Spec = newProjectRule.Spec
				if _, err := l.projectAlertRules.Update(updatedProjectRule); err != nil {
					return fmt.Errorf("migrate %s:%s failed, update alert rule failed, %v", v.Namespace, v.Name, err)
				}
			}

			legacyGroup := &v3.ProjectAlertGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      migrationGroupName,
					Namespace: projectName,
				},
				Spec: v32.ProjectGroupSpec{
					ProjectName: projectID,
					CommonGroupField: v32.CommonGroupField{
						DisplayName: "Migrate group",
						Description: "Migrate alert from last version",
						TimingField: v32.TimingField{
							GroupWaitSeconds:      v.Spec.InitialWaitSeconds,
							GroupIntervalSeconds:  defaultGroupIntervalSeconds,
							RepeatIntervalSeconds: v.Spec.RepeatIntervalSeconds,
						},
					},
					Recipients: v.Spec.Recipients,
				},
			}

			legacyGroup, err = l.projectAlertGroups.Create(legacyGroup)
			if err != nil && !apierrors.IsAlreadyExists(err) {
				return fmt.Errorf("create migrate alert group %s:%s failed, %v", legacyGroup.Namespace, legacyGroup.Name, err)
			}
		}
	}
	return nil
}

func (l *AlertService) removeLegacyAlerting() error {
	legacyAlertmanagerNamespace := "cattle-alerting"

	if err := l.namespaces.Delete(legacyAlertmanagerNamespace, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrap(err, "failed to remove legacy alerting namespace when upgrade")
	}
	return nil
}

func (l *AlertService) removeFinalizerFromLegacyAlerting() error {
	oldProjectAlert, err := l.oldProjectAlertLister.List("", labels.NewSelector())
	if err != nil {
		return errors.Wrap(err, "list legacy projectAlerts failed")
	}

	for _, v := range oldProjectAlert {
		if len(v.Finalizers) == 0 {
			continue
		}
		newObj := v.DeepCopy()
		newObj.SetFinalizers([]string{})
		if _, err = l.oldProjectAlerts.Update(newObj); err != nil {
			return errors.Wrapf(err, "remove finalizer from legacy projectAlert %s:%s failed", newObj.Namespace, newObj.Name)
		}
	}

	return nil
}
