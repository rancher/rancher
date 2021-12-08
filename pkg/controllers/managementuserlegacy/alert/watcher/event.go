package watcher

import (
	"context"
	"fmt"
	"strconv"

	"github.com/rancher/rancher/pkg/controllers/managementagent/workload"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/alert/common"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/alert/manager"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

type EventWatcher struct {
	eventLister            v1.EventLister
	clusterAlertRuleLister v3.ClusterAlertRuleLister
	alertManager           *manager.AlertManager
	clusterName            string
	clusterLister          v3.ClusterLister
	workloadFetcher        workloadFetcher
	podLister              v1.PodLister
}

func StartEventWatcher(ctx context.Context, cluster *config.UserContext, manager *manager.AlertManager) {
	events := cluster.Core.Events("")
	workloadFetcher := workloadFetcher{
		workloadController: workload.NewWorkloadController(ctx, cluster.UserOnlyContext(), nil),
	}

	eventWatcher := &EventWatcher{
		eventLister:            events.Controller().Lister(),
		clusterAlertRuleLister: cluster.Management.Management.ClusterAlertRules(cluster.ClusterName).Controller().Lister(),
		alertManager:           manager,
		clusterName:            cluster.ClusterName,
		clusterLister:          cluster.Management.Management.Clusters("").Controller().Lister(),
		workloadFetcher:        workloadFetcher,
		podLister:              cluster.Core.Pods(metav1.NamespaceAll).Controller().Lister(),
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
			ruleID := common.GetRuleID(alert.Spec.GroupName, alert.Name)

			clusterDisplayName := common.GetClusterDisplayName(l.clusterName, l.clusterLister)

			data := map[string]string{}
			data["rule_id"] = ruleID
			data["group_id"] = alert.Spec.GroupName
			data["alert_name"] = alert.Spec.DisplayName
			data["alert_type"] = "event"
			data["event_type"] = alert.Spec.EventRule.EventType
			data["resource_kind"] = alert.Spec.EventRule.ResourceKind
			data["severity"] = alert.Spec.Severity
			data["cluster_name"] = clusterDisplayName
			data["target_name"] = obj.InvolvedObject.Name
			data["target_namespace"] = obj.InvolvedObject.Namespace
			data["event_count"] = strconv.Itoa(int(obj.Count))
			data["event_message"] = obj.Message
			data["event_firstseen"] = fmt.Sprintf("%s", obj.FirstTimestamp)
			data["event_lastseen"] = fmt.Sprintf("%s", obj.LastTimestamp)

			if alert.Spec.EventRule.ResourceKind == "Pod" || alert.Spec.EventRule.ResourceKind == "Deployment" || alert.Spec.EventRule.ResourceKind == "StatefulSet" || alert.Spec.EventRule.ResourceKind == "DaemonSet" {
				workloadName, err := l.getWorkloadInfo(obj.InvolvedObject.Namespace, obj.InvolvedObject.Name, alert.Spec.EventRule.ResourceKind)
				if err != nil {
					errors.Wrap(err, "failed to fetch workload info")
				}

				if workloadName != "" {
					data["workload_name"] = workloadName
				}

			}

			if err := l.alertManager.SendAlert(data); err != nil {
				logrus.Errorf("Failed to send alert: %v", err)
			}
		}

	}

	return nil, nil
}

func (l *EventWatcher) getWorkloadInfo(namespace, name, kind string) (string, error) {
	if kind == "Pod" {
		pod, err := l.podLister.Get(namespace, name)
		if err != nil {
			return "", errors.Wrapf(err, "failed to get pod %s:%s", namespace, name)
		}
		if len(pod.OwnerReferences) == 0 {
			return pod.Name, nil
		}
		ownerRef := pod.OwnerReferences[0]
		name = ownerRef.Name
		kind = ownerRef.Kind
	}

	workloadName, err := l.workloadFetcher.getWorkloadName(namespace, name, kind)
	if err != nil {
		return "", errors.Wrap(err, "Failed to get workload info for alert")
	}
	return workloadName, nil
}

type workloadFetcher struct {
	workloadController workload.CommonController
}

func (w *workloadFetcher) getWorkloadName(namespace, name, kind string) (string, error) {
	if kind == "Deployment" || kind == "StatefulSet" || kind == "DaemonSet" || kind == "CronJob" {
		return name, nil
	}

	workloadID := fmt.Sprintf("%s:%s:%s", kind, namespace, name)
	workload, err := w.workloadController.GetByWorkloadID(workloadID)
	if err != nil {
		return "", errors.Wrapf(err, "get workload %s failed", workloadID)
	}

	allRef := workload.OwnerReferences
	if len(allRef) == 0 {
		return name, nil
	}

	ref := allRef[0]
	refName := ref.Name
	refKind := ref.Kind

	if kind == "Job" && refKind != "CronJob" {
		return name, nil
	}

	refWorkloadID := fmt.Sprintf("%s:%s:%s", refKind, namespace, refName)
	refWorkload, err := w.workloadController.GetByWorkloadID(refWorkloadID)
	if err != nil {
		return "", errors.Wrapf(err, "get workload %s failed", workloadID)
	}

	return w.getWorkloadName(refWorkload.Namespace, refWorkload.Name, refWorkload.Kind)
}
