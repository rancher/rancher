package watcher

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/rancher/pkg/controllers/user/alert/manager"
	"github.com/rancher/rancher/pkg/ticker"
	"github.com/rancher/types/apis/apps/v1beta2"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

	appsv1beta2 "k8s.io/api/apps/v1beta2"
)

type StatefulsetWatcher struct {
	statefulsetLister  v1beta2.StatefulSetLister
	projectAlertLister v3.ProjectAlertLister
	alertManager       *manager.Manager
	clusterName        string
}

func StartStatefulsetWatcher(ctx context.Context, cluster *config.UserContext, manager *manager.Manager) {
	s := &StatefulsetWatcher{
		statefulsetLister:  cluster.Apps.StatefulSets("").Controller().Lister(),
		projectAlertLister: cluster.Management.Management.ProjectAlerts("").Controller().Lister(),
		alertManager:       manager,
		clusterName:        cluster.ClusterName,
	}

	go s.watch(ctx, syncInterval)
}

func (w *StatefulsetWatcher) watch(ctx context.Context, interval time.Duration) {
	for range ticker.Context(ctx, interval) {
		err := w.watchRule()
		if err != nil {
			logrus.Infof("Failed to watch statefulset", err)
		}
	}
}

func (w *StatefulsetWatcher) watchRule() error {
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
		if alert.Spec.TargetWorkload.Type == "statefulset" {
			if alert.Spec.TargetWorkload.WorkloadID != "" {
				parts := strings.Split(alert.Spec.TargetWorkload.WorkloadID, ":")
				ns := parts[0]
				id := parts[1]
				ss, err := w.statefulsetLister.Get(ns, id)
				if err != nil {
					logrus.Debugf("Failed to get statefulset %s: %v", id, err)
					continue
				}
				w.checkUnavailble(ss, alert)

			} else if alert.Spec.TargetWorkload.Selector != nil {

				selector := labels.NewSelector()
				for key, value := range alert.Spec.TargetWorkload.Selector {
					r, err := labels.NewRequirement(key, selection.Equals, []string{value})
					if err != nil {
						logrus.Warnf("Fail to create new requirement foo %s: %v", key, err)
						continue
					}
					selector = selector.Add(*r)
				}
				ss, err := w.statefulsetLister.List("", selector)
				if err != nil {
					continue
				}
				for _, s := range ss {
					w.checkUnavailble(s, alert)
				}

			}
		}
	}

	return nil
}

func (w *StatefulsetWatcher) checkUnavailble(ss *appsv1beta2.StatefulSet, alert *v3.ProjectAlert) {
	alertID := alert.Namespace + "-" + alert.Name
	percentage := alert.Spec.TargetWorkload.AvailablePercentage

	if percentage == 0 {
		return
	}

	availableThreshold := int32(percentage) * (*ss.Spec.Replicas) / 100

	if ss.Status.ReadyReplicas <= availableThreshold {
		title := fmt.Sprintf("The stateful set %s has available replicas less than %s%%", ss.Name, strconv.Itoa(percentage))
		//TODO: get reason or log
		desc := fmt.Sprintf("*Alert Name*: %s\n*Cluster Name*: %s\n*Ready Replicas*: %s\n*Desired Replicas*: %s", alert.Spec.DisplayName, w.clusterName, strconv.Itoa(int(ss.Status.ReadyReplicas)),
			strconv.Itoa(int(*ss.Spec.Replicas)))

		if err := w.alertManager.SendAlert(alertID, desc, title, alert.Spec.Severity); err != nil {
			logrus.Debugf("Failed to send alert: %v", err)
		}
	}

}
