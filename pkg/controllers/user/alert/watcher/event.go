package watcher

import (
	"context"
	"fmt"
	"strconv"

	"github.com/rancher/rancher/pkg/controllers/user/alert/configsyncer"
	"github.com/rancher/rancher/pkg/controllers/user/alert/manager"

	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

type EventWatcher struct {
	eventLister            v1.EventLister
	clusterAlertRuleLister v3.ClusterAlertRuleLister
	alertManager           *manager.AlertManager
	clusterName            string
	clusterLister          v3.ClusterLister
}

func StartEventWatcher(ctx context.Context, cluster *config.UserContext, manager *manager.AlertManager) {
	events := cluster.Core.Events("")
	eventWatcher := &EventWatcher{
		eventLister:            events.Controller().Lister(),
		clusterAlertRuleLister: cluster.Management.Management.ClusterAlertRules(cluster.ClusterName).Controller().Lister(),
		alertManager:           manager,
		clusterName:            cluster.ClusterName,
		clusterLister:          cluster.Management.Management.Clusters("").Controller().Lister(),
	}

	events.AddHandler(ctx, "cluster-event-alert-watcher", eventWatcher.Sync)
}

func (l *EventWatcher) Sync(key string, obj *corev1.Event) (runtime.Object, error) {
	if l.alertManager.IsDeploy == false {
		return nil, nil
	}

	if obj == nil {
		return nil, nil
	}

	clusterAlerts, err := l.clusterAlertRuleLister.List("", labels.NewSelector())
	if err != nil {
		return nil, err
	}

	for _, alert := range clusterAlerts {
		if alert.Status.AlertState == "inactive" || alert.Status.AlertState == "muted" || alert.Spec.EventRule == nil {
			continue
		}
		if alert.Spec.EventRule.EventType == obj.Type && alert.Spec.EventRule.ResourceKind == obj.InvolvedObject.Kind {
			ruleID := configsyncer.GetRuleID(alert.Spec.GroupName, alert.Name)

			clusterDisplayName := l.clusterName
			cluster, err := l.clusterLister.Get("", l.clusterName)
			if err != nil {
				logrus.Warnf("Failed to get cluster for %s: %v", l.clusterName, err)
			} else {
				clusterDisplayName = cluster.Spec.DisplayName
			}

			data := map[string]string{}
			data["rule_id"] = ruleID
			data["group_id"] = alert.Spec.GroupName
			data["alert_type"] = "event"
			data["event_type"] = alert.Spec.EventRule.EventType
			data["resource_kind"] = alert.Spec.EventRule.ResourceKind
			data["severity"] = alert.Spec.Severity
			data["cluster_name"] = clusterDisplayName
			data["target_name"] = obj.InvolvedObject.Name
			data["event_count"] = strconv.Itoa(int(obj.Count))
			data["event_message"] = obj.Message
			data["event_firstseen"] = fmt.Sprintf("%s", obj.FirstTimestamp)
			data["event_lastseen"] = fmt.Sprintf("%s", obj.LastTimestamp)

			if err := l.alertManager.SendAlert(data); err != nil {
				logrus.Errorf("Failed to send alert: %v", err)
			}
		}

	}

	return nil, nil
}
