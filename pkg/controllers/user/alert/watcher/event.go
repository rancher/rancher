package watcher

import (
	"fmt"
	"strconv"

	"github.com/rancher/rancher/pkg/controllers/user/alert/manager"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type EventWatcher struct {
	eventLister        v1.EventLister
	clusterAlertLister v3.ClusterAlertLister
	alertManager       *manager.Manager
	clusterName        string
	clusterLister      v3.ClusterLister
}

func StartEventWatcher(cluster *config.UserContext, manager *manager.Manager) {
	events := cluster.Core.Events("")

	eventWatcher := &EventWatcher{
		eventLister:        events.Controller().Lister(),
		clusterAlertLister: cluster.Management.Management.ClusterAlerts(cluster.ClusterName).Controller().Lister(),
		alertManager:       manager,
		clusterName:        cluster.ClusterName,
		clusterLister:      cluster.Management.Management.Clusters("").Controller().Lister(),
	}

	events.AddHandler("cluster-event-alerter", eventWatcher.Sync)
}

func (l *EventWatcher) Sync(key string, obj *corev1.Event) error {
	if l.alertManager.IsDeploy == false {
		return nil
	}

	if obj == nil {
		return nil
	}

	clusterAlerts, err := l.clusterAlertLister.List("", labels.NewSelector())
	if err != nil {
		return err
	}

	for _, alert := range clusterAlerts {
		if alert.Status.AlertState == "inactive" || alert.Status.AlertState == "muted" {
			continue
		}
		alertID := alert.Namespace + "-" + alert.Name
		target := alert.Spec.TargetEvent
		if target != nil {
			if target.EventType == obj.Type && target.ResourceKind == obj.InvolvedObject.Kind {

				clusterDisplayName := l.clusterName
				cluster, err := l.clusterLister.Get("", l.clusterName)
				if err != nil {
					logrus.Warnf("Failed to get cluster for %s: %v", l.clusterName, err)
				} else {
					clusterDisplayName = cluster.Spec.DisplayName
				}

				data := map[string]string{}
				data["alert_type"] = "event"
				data["alert_id"] = alertID
				data["event_type"] = target.EventType
				data["resource_kind"] = target.ResourceKind
				data["severity"] = alert.Spec.Severity
				data["alert_name"] = alert.Spec.DisplayName
				data["cluster_name"] = clusterDisplayName
				data["target_name"] = obj.InvolvedObject.Name
				data["event_count"] = strconv.Itoa(int(obj.Count))
				data["event_message"] = obj.Message
				data["event_firstseen"] = fmt.Sprintf("%s", obj.FirstTimestamp)
				data["event_lastseen"] = fmt.Sprintf("%s", obj.LastTimestamp)

				if err := l.alertManager.SendAlert(data); err != nil {
					logrus.Debugf("Failed to send alert: %v", err)
				}
			}
		}
	}

	return nil
}
