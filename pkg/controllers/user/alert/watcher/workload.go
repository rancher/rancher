package watcher

import (
	"context"
	"strconv"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/rancher/pkg/controllers/user/alert/manager"
	"github.com/rancher/rancher/pkg/controllers/user/workload"
	"github.com/rancher/rancher/pkg/ticker"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
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
	workloadController workload.CommonController
	alertManager       *manager.Manager
	projectAlerts      v3.ProjectAlertInterface
	projectAlertLister v3.ProjectAlertLister
	clusterName        string
	clusterLister      v3.ClusterLister
	namespaceIndexer   cache.Indexer
}

func StartWorkloadWatcher(ctx context.Context, cluster *config.UserContext, manager *manager.Manager) {
	nsInformer := cluster.Core.Namespaces("").Controller().Informer()
	nsIndexers := map[string]cache.IndexFunc{
		nsByProjectIndex: nsutils.NsByProjectID,
	}
	nsInformer.AddIndexers(nsIndexers)
	projectAlerts := cluster.Management.Management.ProjectAlerts("")
	d := &WorkloadWatcher{
		projectAlerts:      projectAlerts,
		projectAlertLister: projectAlerts.Controller().Lister(),
		workloadController: workload.NewWorkloadController(cluster.UserOnlyContext(), nil),
		alertManager:       manager,
		clusterName:        cluster.ClusterName,
		clusterLister:      cluster.Management.Management.Clusters("").Controller().Lister(),
		namespaceIndexer:   nsInformer.GetIndexer(),
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

	projectAlerts, err := w.projectAlertLister.List("", labels.NewSelector())
	if err != nil {
		return err
	}

	pAlerts := []*v3.ProjectAlert{}
	for _, alert := range projectAlerts {
		if controller.ObjectInCluster(w.clusterName, alert) {
			pAlerts = append(pAlerts, alert)
		}
	}

	for _, alert := range pAlerts {
		if alert.Status.AlertState == "inactive" {
			continue
		}

		if alert.Spec.TargetWorkload == nil {
			continue
		}

		if alert.Spec.TargetWorkload.WorkloadID != "" {

			wl, err := w.workloadController.GetByWorkloadID(alert.Spec.TargetWorkload.WorkloadID)
			if err != nil {
				if kerrors.IsNotFound(err) || wl == nil {
					if err = w.projectAlerts.DeleteNamespaced(alert.Namespace, alert.Name, &metav1.DeleteOptions{}); err != nil {
						return err
					}
				}
				logrus.Warnf("Fail to get workload for %s, %v", alert.Spec.TargetWorkload.WorkloadID, err)
				continue
			}

			w.checkWorkloadCondition(wl, alert)

		} else if alert.Spec.TargetWorkload.Selector != nil {
			namespaces, err := w.namespaceIndexer.ByIndex(nsByProjectIndex, alert.Spec.ProjectName)
			if err != nil {
				return err
			}
			for _, n := range namespaces {
				namespace, _ := n.(*corev1.Namespace)
				wls, err := w.workloadController.GetWorkloadsMatchingSelector(namespace.Name, alert.Spec.TargetWorkload.Selector)
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

func (w *WorkloadWatcher) checkWorkloadCondition(wl *workload.Workload, alert *v3.ProjectAlert) {

	if wl.Kind == workload.JobType || wl.Kind == workload.CronJobType {
		return
	}

	alertID := alert.Namespace + "-" + alert.Name
	percentage := alert.Spec.TargetWorkload.AvailablePercentage

	if percentage == 0 {
		return
	}

	availableThreshold := int32(percentage) * (wl.Status.Replicas) / 100

	if wl.Status.AvailableReplicas < availableThreshold {

		clusterDisplayName := w.clusterName
		cluster, err := w.clusterLister.Get("", w.clusterName)
		if err != nil {
			logrus.Warnf("Failed to get cluster for %s: %v", w.clusterName, err)
		} else {
			clusterDisplayName = cluster.Spec.DisplayName
		}

		data := map[string]string{}
		data["alert_type"] = "workload"
		data["alert_id"] = alertID
		data["severity"] = alert.Spec.Severity
		data["alert_name"] = alert.Spec.DisplayName
		data["cluster_name"] = clusterDisplayName
		data["workload_name"] = wl.Name
		data["available_percentage"] = strconv.Itoa(percentage)
		data["available_replicas"] = strconv.Itoa(int(wl.Status.AvailableReplicas))
		data["desired_replicas"] = strconv.Itoa(int(wl.Status.Replicas))

		if err := w.alertManager.SendAlert(data); err != nil {
			logrus.Debugf("Failed to send alert: %v", err)
		}
	}

}
