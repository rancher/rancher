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
}

func StartEventWatcher(cluster *config.UserContext, manager *manager.Manager) {
	events := cluster.Core.Events("")

	eventWatcher := &EventWatcher{
		eventLister:        events.Controller().Lister(),
		clusterAlertLister: cluster.Management.Management.ClusterAlerts(cluster.ClusterName).Controller().Lister(),
		alertManager:       manager,
		clusterName:        cluster.ClusterName,
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
		if alert.Status.AlertState == "inactive" {
			continue
		}
		alertID := alert.Namespace + "-" + alert.Name
		target := alert.Spec.TargetEvent
		if target.ResourceKind != "" {
			if target.EventType == obj.Type && target.ResourceKind == obj.InvolvedObject.Kind {

				title := fmt.Sprintf("%s event of %s occurred", target.EventType, target.ResourceKind)
				//TODO: how to set unit for display for Quantity
				desc := fmt.Sprintf("*Alert Name*: %s\n*Cluster Name*: %s\n*Target*: %s\n*Count*: %s\n*Event Message*: %s\n*First Seen*: %s\n*Last Seen*: %s",
					alert.Spec.DisplayName, l.clusterName, obj.InvolvedObject.Name, strconv.Itoa(int(obj.Count)), obj.Message, obj.FirstTimestamp, obj.LastTimestamp)

				if err := l.alertManager.SendAlert(alertID, desc, title, alert.Spec.Severity); err != nil {
					logrus.Debugf("Failed to send alert: %v", err)
				}
			}
		}
	}

	return nil
}
