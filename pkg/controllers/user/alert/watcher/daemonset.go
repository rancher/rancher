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

const (
	syncInterval = 30 * time.Second
)

type DaemonsetWatcher struct {
	daemonsetLister    v1beta2.DaemonSetLister
	projectAlertLister v3.ProjectAlertLister
	alertManager       *manager.Manager
	clusterName        string
}

func StartDaemonsetWatcher(ctx context.Context, cluster *config.UserContext, manager *manager.Manager) {

	d := &DaemonsetWatcher{
		daemonsetLister:    cluster.Apps.DaemonSets("").Controller().Lister(),
		projectAlertLister: cluster.Management.Management.ProjectAlerts("").Controller().Lister(),
		alertManager:       manager,
		clusterName:        cluster.ClusterName,
	}

	go d.watch(ctx, syncInterval)
}

func (w *DaemonsetWatcher) watch(ctx context.Context, interval time.Duration) {
	for range ticker.Context(ctx, interval) {
		err := w.watchRule()
		if err != nil {
			logrus.Infof("Failed to watch daemonset", err)
		}
	}
}

func (w *DaemonsetWatcher) watchRule() error {
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
		if alert.Status.State == "inactive" {
			continue
		}

		if alert.Spec.TargetWorkload.Type == "daemonset" {
			if alert.Spec.TargetWorkload.WorkloadID != "" {
				parts := strings.Split(alert.Spec.TargetWorkload.WorkloadID, ":")
				ns := parts[0]
				id := parts[1]
				ds, err := w.daemonsetLister.Get(ns, id)
				if err != nil {
					logrus.Debugf("Failed to get daemonset %s: %v", id, err)
					continue
				}
				w.checkUnavailble(ds, alert)

			} else if alert.Spec.TargetWorkload.Selector != nil {

				selector := labels.NewSelector()
				for key, value := range alert.Spec.TargetWorkload.Selector {
					r, _ := labels.NewRequirement(key, selection.Equals, []string{value})
					selector.Add(*r)
				}
				dsList, err := w.daemonsetLister.List("", selector)
				if err != nil {
					continue
				}
				for _, ds := range dsList {
					w.checkUnavailble(ds, alert)
				}
			}
		}
	}

	return nil
}

func (w *DaemonsetWatcher) checkUnavailble(ds *appsv1beta2.DaemonSet, alert *v3.ProjectAlert) {
	alertID := alert.Namespace + "-" + alert.Name
	percentage := alert.Spec.TargetWorkload.AvailablePercentage

	if percentage == 0 {
		return
	}

	availableThreshold := int32(percentage) * (ds.Status.DesiredNumberScheduled) / 100

	if ds.Status.NumberAvailable <= availableThreshold {
		title := fmt.Sprintf("The daemonset %s has available replicas less than %s%%", ds.Name, strconv.Itoa(percentage))
		//TODO: get reason or log
		desc := fmt.Sprintf("*Alert Name*: %s\n*Cluster Name*: %s\n*Ready Replicas*: %s\n*Desired Replicas*: %s", alert.Spec.DisplayName, w.clusterName, strconv.Itoa(int(ds.Status.NumberAvailable)),
			strconv.Itoa(int(ds.Status.DesiredNumberScheduled)))

		if err := w.alertManager.SendAlert(alertID, desc, title, alert.Spec.Severity); err != nil {
			logrus.Debugf("Failed to send alert: %v", err)
		}
	}

}
