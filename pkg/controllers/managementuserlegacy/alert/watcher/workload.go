package watcher

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/rancher/pkg/controllers/managementagent/workload"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/alert/common"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/alert/manager"
	appsv1 "github.com/rancher/rancher/pkg/generated/norman/apps/v1"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v2/pkg/ticker"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	nsutils "github.com/rancher/rancher/pkg/namespace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	syncInterval     = 30 * time.Second
	nsByProjectIndex = "projectalert.cluster.cattle.io/ns-by-project"
)

type WorkloadWatcher struct {
	workloadController          workload.CommonController
	alertManager                *manager.AlertManager
	projectAlertPolicies        v3.ProjectAlertRuleInterface
	projectAlertRuleLister      v3.ProjectAlertRuleLister
	clusterName                 string
	clusterLister               v3.ClusterLister
	projectLister               v3.ProjectLister
	namespaceIndexer            cache.Indexer
	replicationControllerLister v1.ReplicationControllerLister
	replicaSetLister            appsv1.ReplicaSetLister
	daemonsetLister             appsv1.DaemonSetLister
	deploymentLister            appsv1.DeploymentLister
	statefulsetLister           appsv1.StatefulSetLister
}

func StartWorkloadWatcher(ctx context.Context, cluster *config.UserContext, manager *manager.AlertManager) {
	nsInformer := cluster.Core.Namespaces("").Controller().Informer()
	nsIndexers := map[string]cache.IndexFunc{
		nsByProjectIndex: nsutils.NsByProjectID,
	}
	nsInformer.AddIndexers(nsIndexers)
	projectAlerts := cluster.Management.Management.ProjectAlertRules("")
	d := &WorkloadWatcher{
		projectAlertPolicies:        projectAlerts,
		projectAlertRuleLister:      projectAlerts.Controller().Lister(),
		workloadController:          workload.NewWorkloadController(ctx, cluster.UserOnlyContext(), nil),
		alertManager:                manager,
		clusterName:                 cluster.ClusterName,
		clusterLister:               cluster.Management.Management.Clusters("").Controller().Lister(),
		projectLister:               cluster.Management.Management.Projects(cluster.ClusterName).Controller().Lister(),
		namespaceIndexer:            nsInformer.GetIndexer(),
		replicationControllerLister: cluster.Core.ReplicationControllers(metav1.NamespaceAll).Controller().Lister(),
		replicaSetLister:            cluster.Apps.ReplicaSets(metav1.NamespaceAll).Controller().Lister(),
		daemonsetLister:             cluster.Apps.DaemonSets(metav1.NamespaceAll).Controller().Lister(),
		deploymentLister:            cluster.Apps.Deployments(metav1.NamespaceAll).Controller().Lister(),
		statefulsetLister:           cluster.Apps.StatefulSets(metav1.NamespaceAll).Controller().Lister(),
	}

	go d.watch(ctx, syncInterval)
}

func (w *WorkloadWatcher) watch(ctx context.Context, interval time.Duration) {
	for range ticker.Context(ctx, interval) {
		err := w.watchRule()
		if err != nil {
			logrus.Infof("Failed to watch deployment, error: %v", err)
		}
	}
}

func (w *WorkloadWatcher) watchRule() error {
	if w.alertManager.IsDeploy == false {
		return nil
	}

	projectAlerts, err := w.projectAlertRuleLister.List("", labels.NewSelector())
	if err != nil {
		return err
	}

	pAlerts := []*v3.ProjectAlertRule{}
	for _, alert := range projectAlerts {
		if controller.ObjectInCluster(w.clusterName, alert) {
			pAlerts = append(pAlerts, alert)
		}
	}

	for _, alert := range pAlerts {
		if alert.Status.AlertState == "inactive" || alert.Spec.WorkloadRule == nil {
			continue
		}

		if alert.Spec.WorkloadRule.WorkloadID != "" {

			wl, err := w.workloadController.GetByWorkloadID(alert.Spec.WorkloadRule.WorkloadID)
			if err != nil {
				if kerrors.IsNotFound(err) || wl == nil {
					if err = w.projectAlertPolicies.DeleteNamespaced(alert.Namespace, alert.Name, &metav1.DeleteOptions{}); err != nil {
						return err
					}
				}
				logrus.Warnf("Fail to get workload for %s, %v", alert.Spec.WorkloadRule.WorkloadID, err)
				continue
			}

			w.checkWorkloadCondition(wl, alert)

		} else if alert.Spec.WorkloadRule.Selector != nil {
			namespaces, err := w.namespaceIndexer.ByIndex(nsByProjectIndex, alert.Spec.ProjectName)
			if err != nil {
				return err
			}
			for _, n := range namespaces {
				namespace, _ := n.(*corev1.Namespace)
				wls, err := w.workloadController.GetWorkloadsMatchingSelector(namespace.Name, alert.Spec.WorkloadRule.Selector)
				if err != nil {
					logrus.Warnf("Fail to list workload: %v", err)
					continue
				}

				for _, wl := range wls {
					w.checkWorkloadCondition(wl, alert)
				}
			}
		}

	}

	return nil
}

func (w *WorkloadWatcher) checkWorkloadCondition(wl *workload.Workload, alert *v3.ProjectAlertRule) {

	if wl.Kind == workload.JobType || wl.Kind == workload.CronJobType {
		return
	}

	percentage := alert.Spec.WorkloadRule.AvailablePercentage

	if percentage == 0 {
		return
	}
	desiredReplicas, err := w.getDesiredReplicas(fmt.Sprintf("%s:%s:%s", wl.Kind, wl.Namespace, wl.Name))
	if err != nil {
		logrus.Errorf("Failed to get workload %s desired replicas %v", wl.Name, err)
		return
	}
	availableThreshold := float32(percentage) * float32(desiredReplicas) / 100
	if float32(wl.Status.AvailableReplicas) < availableThreshold {
		ruleID := common.GetRuleID(alert.Spec.GroupName, alert.Name)

		clusterDisplayName := common.GetClusterDisplayName(w.clusterName, w.clusterLister)
		projectDisplayName := common.GetProjectDisplayName(alert.Spec.ProjectName, w.projectLister)

		data := map[string]string{}
		data["rule_id"] = ruleID
		data["group_id"] = alert.Spec.GroupName
		data["alert_type"] = "workload"
		data["alert_name"] = alert.Spec.DisplayName
		data["severity"] = alert.Spec.Severity
		data["cluster_name"] = clusterDisplayName
		data["project_name"] = projectDisplayName
		data["workload_name"] = wl.Name
		data["workload_namespace"] = wl.Namespace
		data["workload_kind"] = wl.Kind
		data["available_percentage"] = strconv.Itoa(percentage)
		data["available_replicas"] = strconv.Itoa(int(wl.Status.AvailableReplicas))
		data["desired_replicas"] = strconv.Itoa(int(desiredReplicas))

		if err := w.alertManager.SendAlert(data); err != nil {
			logrus.Errorf("Failed to send alert: %v", err)
		}
	}

}

func (w *WorkloadWatcher) getDesiredReplicas(workloadName string) (int32, error) {
	var desiredReplicas int32
	splitted := strings.Split(workloadName, ":")
	if len(splitted) != 3 {
		return desiredReplicas, errors.New("invalid workloadName: " + workloadName)
	}
	workloadType := strings.ToLower(splitted[0])
	namespace := splitted[1]
	name := splitted[2]
	switch workloadType {
	case workload.ReplicationControllerType:
		o, err := w.replicationControllerLister.Get(namespace, name)
		if err != nil {
			return desiredReplicas, err
		}
		if o.Spec.Replicas != nil {
			return *o.Spec.Replicas, nil
		}
	case workload.ReplicaSetType:
		o, err := w.replicaSetLister.Get(namespace, name)
		if err != nil {
			return desiredReplicas, err
		}
		if o.Spec.Replicas != nil {
			return *o.Spec.Replicas, nil
		}
	case workload.DaemonSetType:
		o, err := w.daemonsetLister.Get(namespace, name)
		if err != nil {
			return desiredReplicas, err
		}
		return o.Status.DesiredNumberScheduled, nil
	case workload.StatefulSetType:
		o, err := w.statefulsetLister.Get(namespace, name)
		if err != nil {
			return desiredReplicas, err
		}
		if o.Spec.Replicas != nil {
			return *o.Spec.Replicas, nil
		}
	default:
		o, err := w.deploymentLister.Get(namespace, name)
		if err != nil {
			return desiredReplicas, err
		}
		if o.Spec.Replicas != nil {
			return *o.Spec.Replicas, nil
		}
	}
	return desiredReplicas, errors.New("empty replicas")
}
