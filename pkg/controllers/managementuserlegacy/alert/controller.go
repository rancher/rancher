package alert

import (
	"context"
	"fmt"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/ref"

	"github.com/rancher/norman/controller"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/alert/configsyncer"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/alert/deployer"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/alert/manager"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/alert/statesyncer"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/alert/watcher"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	monitorutil "github.com/rancher/rancher/pkg/monitoring"
	"github.com/rancher/rancher/pkg/types/config"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	SeverityInfo       = "info"
	SeverityCritical   = "critical"
	SeverityWarning    = "warning"
	defaultTimingField = v32.TimingField{
		GroupWaitSeconds:      180,
		GroupIntervalSeconds:  180,
		RepeatIntervalSeconds: 3600,
	}
)

func Register(ctx context.Context, mgmt *config.ScaledContext, cluster *config.UserContext) {
	starter := cluster.DeferredStart(ctx, func(ctx context.Context) error {
		registerDeferred(ctx, mgmt, cluster)
		return nil
	})

	AddStarter(ctx, cluster, starter)
}

func AddStarter(ctx context.Context, cluster *config.UserContext, starter func() error) {
	cluster.Management.Management.Features("").AddHandler(ctx, "alerts-deferred", func(key string, obj *v32.Feature) (runtime.Object, error) {
		if features.Legacy.Enabled() {
			return obj, starter()
		}
		return obj, nil
	})

	alerts := cluster.Management.Management.ClusterAlerts("")
	alerts.AddClusterScopedHandler(ctx, "alerts-deferred", cluster.ClusterName, func(key string, obj *v32.ClusterAlert) (runtime.Object, error) {
		return obj, starter()
	})

	notifiers := cluster.Management.Management.Notifiers("")
	notifiers.AddClusterScopedHandler(ctx, "alerts-deferred", cluster.ClusterName, func(key string, obj *v32.Notifier) (runtime.Object, error) {
		return obj, starter()
	})
}

func registerDeferred(ctx context.Context, mgmt *config.ScaledContext, cluster *config.UserContext) {
	alertmanager := manager.NewAlertManager(cluster)

	prometheusCRDManager := manager.NewPrometheusCRDManager(ctx, cluster)

	clusterAlertRules := cluster.Management.Management.ClusterAlertRules(cluster.ClusterName)
	projectAlertRules := cluster.Management.Management.ProjectAlertRules("")

	clusterAlertGroups := cluster.Management.Management.ClusterAlertGroups(cluster.ClusterName)
	projectAlertGroups := cluster.Management.Management.ProjectAlertGroups("")

	notifiers := cluster.Management.Management.Notifiers(cluster.ClusterName)

	deploy := deployer.NewDeployer(cluster, alertmanager)
	clusterAlertGroups.AddClusterScopedHandler(ctx, "cluster-alert-group-deployer", cluster.ClusterName, deploy.ClusterGroupSync)
	projectAlertGroups.AddClusterScopedHandler(ctx, "project-alert-group-deployer", cluster.ClusterName, deploy.ProjectGroupSync)

	clusterAlertRules.AddClusterScopedHandler(ctx, "cluster-alert-rule-deployer", cluster.ClusterName, deploy.ClusterRuleSync)
	projectAlertRules.AddClusterScopedHandler(ctx, "project-alert-rule-deployer", cluster.ClusterName, deploy.ProjectRuleSync)

	configSyncer := configsyncer.NewConfigSyncer(ctx, mgmt, cluster, alertmanager, prometheusCRDManager)
	clusterAlertGroups.AddClusterScopedHandler(ctx, "cluster-alert-group-controller", cluster.ClusterName, configSyncer.ClusterGroupSync)
	projectAlertGroups.AddClusterScopedHandler(ctx, "project-alert-group-controller", cluster.ClusterName, configSyncer.ProjectGroupSync)

	clusterAlertRules.AddClusterScopedHandler(ctx, "cluster-alert-rule-controller", cluster.ClusterName, configSyncer.ClusterRuleSync)
	projectAlertRules.AddClusterScopedHandler(ctx, "project-alert-rule-controller", cluster.ClusterName, configSyncer.ProjectRuleSync)
	notifiers.AddClusterScopedHandler(ctx, "notifier-config-syncer", cluster.ClusterName, configSyncer.NotifierSync)

	cleaner := &alertGroupCleaner{
		clusterName:        cluster.ClusterName,
		operatorCRDManager: prometheusCRDManager,
		clusterAlertRules:  clusterAlertRules,
		projectAlertRules:  projectAlertRules,
		clusterAlertGroups: clusterAlertGroups,
		projectAlertGroups: projectAlertGroups,
	}

	cl := &clusterAlertGroupLifecycle{cleaner: cleaner}
	pl := &projectAlertGroupLifecycle{cleaner: cleaner}
	clusterAlertGroups.AddClusterScopedLifecycle(ctx, "cluster-alert-group-lifecycle", cluster.ClusterName, cl)
	projectAlertGroups.AddClusterScopedLifecycle(ctx, "project-alert-group-lifecycle", cluster.ClusterName, pl)

	projectLifecycle := &ProjectLifecycle{
		projectAlertRules:  projectAlertRules,
		projectAlertGroups: projectAlertGroups,
		clusterName:        cluster.ClusterName,
	}
	projects := cluster.Management.Management.Projects(cluster.ClusterName)
	projects.AddClusterScopedLifecycle(ctx, "project-precan-alert-controller", cluster.ClusterName, projectLifecycle)

	statesyncer.StartStateSyncer(ctx, cluster, alertmanager)

	i := &initClusterAlerts{
		clusterAlertGroups:      clusterAlertGroups,
		clusterAlertGroupLister: cluster.Management.Management.ClusterAlertGroups("").Controller().Lister(),
		clusterAlertRules:       clusterAlertRules,
		clusterAlertRuleLister:  cluster.Management.Management.ClusterAlertRules("").Controller().Lister(),
	}
	initClusterPreCanAlerts(i, cluster.ClusterName)

	nodeSyncer := &windowsNodeSyner{
		clusterName:       cluster.ClusterName,
		clusterAlertRules: clusterAlertRules,
		nodeLister:        cluster.Core.Nodes(metav1.NamespaceAll).Controller().Lister(),
	}
	nodes := cluster.Core.Nodes(metav1.NamespaceAll)
	nodes.AddHandler(ctx, "init-windows-alert", nodeSyncer.Sync)

	watcher.StartEventWatcher(ctx, cluster, alertmanager)
	watcher.StartSysComponentWatcher(ctx, cluster, alertmanager)
	watcher.StartPodWatcher(ctx, cluster, alertmanager)
	watcher.StartWorkloadWatcher(ctx, cluster, alertmanager)
	watcher.StartNodeWatcher(ctx, cluster, alertmanager)
}

type clusterAlertGroupLifecycle struct {
	cleaner *alertGroupCleaner
}

type projectAlertGroupLifecycle struct {
	cleaner *alertGroupCleaner
}

type alertGroupCleaner struct {
	clusterName        string
	operatorCRDManager *manager.PromOperatorCRDManager
	clusterAlertRules  v3.ClusterAlertRuleInterface
	projectAlertRules  v3.ProjectAlertRuleInterface
	clusterAlertGroups v3.ClusterAlertGroupInterface
	projectAlertGroups v3.ProjectAlertGroupInterface
}

func (l *projectAlertGroupLifecycle) Create(obj *v3.ProjectAlertGroup) (runtime.Object, error) {
	return obj, nil
}

func (l *projectAlertGroupLifecycle) Updated(obj *v3.ProjectAlertGroup) (runtime.Object, error) {
	return obj, nil
}

func (l *projectAlertGroupLifecycle) Remove(obj *v3.ProjectAlertGroup) (runtime.Object, error) {
	err := l.cleaner.Clean(nil, obj)
	return obj, err
}

func (l *clusterAlertGroupLifecycle) Create(obj *v3.ClusterAlertGroup) (runtime.Object, error) {
	return obj, nil
}

func (l *clusterAlertGroupLifecycle) Updated(obj *v3.ClusterAlertGroup) (runtime.Object, error) {
	return obj, nil
}

func (l *clusterAlertGroupLifecycle) Remove(obj *v3.ClusterAlertGroup) (runtime.Object, error) {
	err := l.cleaner.Clean(obj, nil)
	return obj, err
}

func (l *alertGroupCleaner) Clean(clusterGroup *v3.ClusterAlertGroup, projectGroup *v3.ProjectAlertGroup) error {
	if clusterGroup != nil {
		groupName := fmt.Sprintf("%s:%s", clusterGroup.Namespace, clusterGroup.Name)
		list, err := l.clusterAlertRules.List(metav1.ListOptions{})
		if err != nil {
			return err
		}

		for _, v := range list.Items {
			if v.Spec.GroupName == groupName {
				if err := l.clusterAlertRules.DeleteNamespaced(v.Namespace, v.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
					return err
				}
			}
		}

		selector := fields.OneTermNotEqualSelector("metadata.name", clusterGroup.Name)
		groups, err := l.clusterAlertGroups.List(metav1.ListOptions{FieldSelector: selector.String()})
		if err != nil {
			return fmt.Errorf("list cluster alert group failed while clean, %v", err)
		}

		if len(groups.Items) == 0 {
			_, namespace := monitorutil.ClusterMonitoringInfo()
			if err := l.operatorCRDManager.DeletePrometheusRule(namespace, l.clusterName); err != nil {
				return err
			}
		}
		return nil
	}

	if projectGroup != nil {
		groupName := fmt.Sprintf("%s:%s", projectGroup.Namespace, projectGroup.Name)

		list, err := l.projectAlertRules.List(metav1.ListOptions{})
		if err != nil {
			return err
		}

		for _, v := range list.Items {
			if controller.ObjectInCluster(l.clusterName, &v) {
				if v.Spec.GroupName == groupName {
					if err := l.projectAlertRules.DeleteNamespaced(v.Namespace, v.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
						return err
					}
				}
			}
		}
		_, projectName := ref.Parse(projectGroup.Spec.ProjectName)

		s1 := fields.OneTermEqualSelector("metadata.namespace", projectName)
		s2 := fields.OneTermNotEqualSelector("metadata.name", projectGroup.Name)
		selector := fields.AndSelectors(s2, s1)
		groups, err := l.projectAlertGroups.List(metav1.ListOptions{FieldSelector: selector.String()})
		if err != nil {
			return fmt.Errorf("list project alert group failed while clean, %v", err)
		}

		if len(groups.Items) == 0 {
			_, namespace := monitorutil.ProjectMonitoringInfo(projectName)
			if err := l.operatorCRDManager.DeletePrometheusRule(namespace, projectName); err != nil {
				return err
			}
		}

	}
	return nil
}
