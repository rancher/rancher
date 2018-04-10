package watcher

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rancher/rancher/pkg/controllers/user/alert/manager"
	"github.com/rancher/rancher/pkg/ticker"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SysComponentWatcher struct {
	componentStatuses  v1.ComponentStatusInterface
	clusterAlertLister v3.ClusterAlertLister
	alertManager       *manager.Manager
	clusterName        string
}

func StartSysComponentWatcher(ctx context.Context, cluster *config.UserContext, manager *manager.Manager) {

	s := &SysComponentWatcher{
		componentStatuses:  cluster.Core.ComponentStatuses(""),
		clusterAlertLister: cluster.Management.Management.ClusterAlerts(cluster.ClusterName).Controller().Lister(),
		alertManager:       manager,
		clusterName:        cluster.ClusterName,
	}
	go s.watch(ctx, syncInterval)
}

func (w *SysComponentWatcher) watch(ctx context.Context, interval time.Duration) {
	for range ticker.Context(ctx, interval) {
		err := w.watchRule()
		if err != nil {
			logrus.Infof("Failed to watch system component", err)
		}
	}
}

func (w *SysComponentWatcher) watchRule() error {
	if w.alertManager.IsDeploy == false {
		return nil
	}

	clusterAlerts, err := w.clusterAlertLister.List("", labels.NewSelector())
	if err != nil {
		return err
	}

	statuses, err := w.componentStatuses.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, alert := range clusterAlerts {
		if alert.Status.AlertState == "inactive" {
			continue
		}
		if alert.Spec.TargetSystemService != nil {
			w.checkComponentHealthy(statuses, alert)
		}
	}
	return nil
}

func (w *SysComponentWatcher) checkComponentHealthy(statuses *v1.ComponentStatusList, alert *v3.ClusterAlert) {
	alertID := alert.Namespace + "-" + alert.Name
	for _, cs := range statuses.Items {
		if strings.HasPrefix(cs.Name, alert.Spec.TargetSystemService.Condition) {
			for _, cond := range cs.Conditions {
				if cond.Type == corev1.ComponentHealthy {
					if cond.Status == corev1.ConditionFalse {
						title := fmt.Sprintf("The system component %s is not running", alert.Spec.TargetSystemService.Condition)
						desc := fmt.Sprintf("*Alert Name*: %s\n*Cluster Name*: %s\n*Logs*: %s", alert.Spec.DisplayName, w.clusterName, cond.Message)

						if err := w.alertManager.SendAlert(alertID, desc, title, alert.Spec.Severity); err != nil {
							logrus.Debugf("Failed to send alert: %v", err)
						}
						return
					}
				}
			}
		}
	}

}
