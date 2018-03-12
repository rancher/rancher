package alert

import (
	"context"

	"github.com/rancher/rancher/pkg/controllers/user/alert/configsyncer"
	"github.com/rancher/rancher/pkg/controllers/user/alert/deploy"
	"github.com/rancher/rancher/pkg/controllers/user/alert/manager"
	"github.com/rancher/rancher/pkg/controllers/user/alert/statesyncer"
	"github.com/rancher/rancher/pkg/controllers/user/alert/watcher"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Register(ctx context.Context, cluster *config.UserContext) {
	alertmanager := manager.NewManager(cluster)

	clusterAlerts := cluster.Management.Management.ClusterAlerts(cluster.ClusterName)
	projectAlerts := cluster.Management.Management.ProjectAlerts("")
	notifiers := cluster.Management.Management.Notifiers("")
	projects := cluster.Management.Management.Projects("")

	projectLifecycle := &ProjectLifecycle{
		projectAlerts: projectAlerts,
		clusterName:   cluster.ClusterName,
	}
	projects.AddClusterScopedLifecycle("project-precan-alert-controller", cluster.ClusterName, projectLifecycle)

	deployer := deploy.NewDeployer(cluster, alertmanager)
	clusterAlerts.AddClusterScopedHandler("cluster-alert-deployer", cluster.ClusterName, deployer.ClusterSync)
	projectAlerts.AddClusterScopedHandler("project-alert-deployer", cluster.ClusterName, deployer.ProjectSync)

	configSyncer := configsyner.NewConfigSyncer(ctx, cluster, alertmanager)
	clusterAlerts.AddClusterScopedHandler("cluster-config-syncer", cluster.ClusterName, configSyncer.ClusterSync)
	projectAlerts.AddClusterScopedHandler("project-config-syncer", cluster.ClusterName, configSyncer.ProjectSync)
	notifiers.AddClusterScopedHandler("notifier-config-syncer", cluster.ClusterName, configSyncer.NotifierSync)

	statesyncer.StartStateSyncer(ctx, cluster, alertmanager)

	watcher.StartSysComponentWatcher(ctx, cluster, alertmanager)
	watcher.StartPodWatcher(ctx, cluster, alertmanager)
	watcher.StartNodeWatcher(ctx, cluster, alertmanager)
	watcher.StartWorkloadWatcher(ctx, cluster, alertmanager)
	watcher.StartEventWatcher(cluster, alertmanager)

	initClusterPreCanAlerts(clusterAlerts, cluster.ClusterName)

}

func initClusterPreCanAlerts(clusterAlerts v3.ClusterAlertInterface, clusterName string) {
	etcdRule := &v3.ClusterAlert{
		ObjectMeta: metav1.ObjectMeta{
			Name: "clusteralert-etcd",
		},
		Spec: v3.ClusterAlertSpec{
			ClusterName: clusterName,
			AlertCommonSpec: v3.AlertCommonSpec{
				DisplayName:           "Alert for etcd",
				Description:           "Built-in Alert for etcd component",
				Severity:              "critical",
				InitialWaitSeconds:    180,
				RepeatIntervalSeconds: 3600,
			},
			TargetSystemService: v3.TargetSystemService{
				Condition: "etcd",
			},
		},
		Status: v3.AlertStatus{
			AlertState: "active",
		},
	}

	if _, err := clusterAlerts.Create(etcdRule); err != nil && !apierrors.IsAlreadyExists(err) {
		logrus.Warnf("Failed to create built-in rules for etcd: %v", err)
	}

	cmRule := &v3.ClusterAlert{
		ObjectMeta: metav1.ObjectMeta{
			Name: "clusteralert-controllermanager",
		},
		Spec: v3.ClusterAlertSpec{
			ClusterName: clusterName,
			AlertCommonSpec: v3.AlertCommonSpec{
				DisplayName:           "Alert for controller-manager",
				Description:           "Built-in Alert for controller-manager component",
				Severity:              "critical",
				InitialWaitSeconds:    180,
				RepeatIntervalSeconds: 3600,
			},
			TargetSystemService: v3.TargetSystemService{
				Condition: "controller-manager",
			},
		},
		Status: v3.AlertStatus{
			AlertState: "active",
		},
	}

	if _, err := clusterAlerts.Create(cmRule); err != nil && !apierrors.IsAlreadyExists(err) {
		logrus.Warnf("Failed to create built-in rules for controller manager: %v", err)
	}

	schedulerRule := &v3.ClusterAlert{
		ObjectMeta: metav1.ObjectMeta{
			Name: "clusteralert-scheduler",
		},
		Spec: v3.ClusterAlertSpec{
			ClusterName: clusterName,
			AlertCommonSpec: v3.AlertCommonSpec{
				DisplayName:           "Alert for scheduler",
				Description:           "Built-in Alert for scheduler component",
				Severity:              "critical",
				InitialWaitSeconds:    180,
				RepeatIntervalSeconds: 3600,
			},
			TargetSystemService: v3.TargetSystemService{
				Condition: "scheduler",
			},
		},
		Status: v3.AlertStatus{
			AlertState: "active",
		},
	}

	if _, err := clusterAlerts.Create(schedulerRule); err != nil && !apierrors.IsAlreadyExists(err) {
		logrus.Warnf("Failed to create built-in rules for scheduler: %v", err)
	}

	nodeRule := &v3.ClusterAlert{
		ObjectMeta: metav1.ObjectMeta{
			Name: "clusteralert-node-mem",
		},
		Spec: v3.ClusterAlertSpec{
			ClusterName: clusterName,
			AlertCommonSpec: v3.AlertCommonSpec{
				DisplayName:           "Alert for Node Memory Usage",
				Description:           "Built-in Alert for node mem usage",
				Severity:              "critical",
				InitialWaitSeconds:    180,
				RepeatIntervalSeconds: 3600,
			},
			TargetNode: v3.TargetNode{
				Condition:    "mem",
				MemThreshold: 70,
				Selector: map[string]string{
					"node": "node",
				},
			},
		},
		Status: v3.AlertStatus{
			AlertState: "active",
		},
	}

	if _, err := clusterAlerts.Create(nodeRule); err != nil && !apierrors.IsAlreadyExists(err) {
		logrus.Warnf("Failed to create built-in rules for node: %v", err)
	}

	eventRule := &v3.ClusterAlert{
		ObjectMeta: metav1.ObjectMeta{
			Name: "clusteralert-deploment-event",
		},
		Spec: v3.ClusterAlertSpec{
			ClusterName: clusterName,
			AlertCommonSpec: v3.AlertCommonSpec{
				DisplayName:           "Alert for Warning Event of Deployment",
				Description:           "Built-in Alert for warning event",
				Severity:              "critical",
				InitialWaitSeconds:    180,
				RepeatIntervalSeconds: 3600,
			},
			TargetEvent: v3.TargetEvent{
				Type:         "Warning",
				ResourceKind: "Deployment",
			},
		},
		Status: v3.AlertStatus{
			AlertState: "active",
		},
	}

	if _, err := clusterAlerts.Create(eventRule); err != nil && !apierrors.IsAlreadyExists(err) {
		logrus.Warnf("Failed to create built-in rules for event: %v", err)
	}

}

type ProjectLifecycle struct {
	projectAlerts v3.ProjectAlertInterface
	clusterName   string
}

//Create built-in project alerts
func (l *ProjectLifecycle) Create(obj *v3.Project) (*v3.Project, error) {
	deploymentAlert := &v3.ProjectAlert{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "projectalert-workload",
			Namespace: obj.Name,
		},
		Spec: v3.ProjectAlertSpec{
			ProjectName: l.clusterName + ":" + obj.Name,
			AlertCommonSpec: v3.AlertCommonSpec{
				DisplayName:           "Alert for Workload",
				Description:           "Built-in Alert for Workload",
				Severity:              "critical",
				InitialWaitSeconds:    180,
				RepeatIntervalSeconds: 3600,
			},
			TargetWorkload: v3.TargetWorkload{
				Selector: map[string]string{
					"app": "workload",
				},
				AvailablePercentage: 50,
			},
		},
		Status: v3.AlertStatus{
			AlertState: "active",
		},
	}

	if _, err := l.projectAlerts.Create(deploymentAlert); err != nil && !apierrors.IsAlreadyExists(err) {
		logrus.Warnf("Failed to create built-in rules for deployment: %v", err)
	}

	return obj, nil
}

func (l *ProjectLifecycle) Updated(obj *v3.Project) (*v3.Project, error) {
	return obj, nil
}

func (l *ProjectLifecycle) Remove(obj *v3.Project) (*v3.Project, error) {
	return obj, nil
}
