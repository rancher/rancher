package watcher

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/controller"
	"github.com/rancher/rancher/pkg/controllers/managementagent/workload"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/alert/common"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/alert/manager"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v2/pkg/ticker"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

type PodWatcher struct {
	podLister               v1.PodLister
	alertManager            *manager.AlertManager
	projectAlertPolicies    v3.ProjectAlertRuleInterface
	projectAlertGroupLister v3.ProjectAlertRuleLister
	clusterName             string
	podRestartTrack         sync.Map
	clusterLister           v3.ClusterLister
	projectLister           v3.ProjectLister
	workloadFetcher         workloadFetcher
}

type restartTrack struct {
	Count int32
	Time  time.Time
}

func StartPodWatcher(ctx context.Context, cluster *config.UserContext, manager *manager.AlertManager) {
	projectAlertPolicies := cluster.Management.Management.ProjectAlertRules("")
	workloadFetcher := workloadFetcher{
		workloadController: workload.NewWorkloadController(ctx, cluster.UserOnlyContext(), nil),
	}

	podWatcher := &PodWatcher{
		podLister:               cluster.Core.Pods("").Controller().Lister(),
		projectAlertPolicies:    projectAlertPolicies,
		projectAlertGroupLister: projectAlertPolicies.Controller().Lister(),
		alertManager:            manager,
		clusterName:             cluster.ClusterName,
		podRestartTrack:         sync.Map{},
		clusterLister:           cluster.Management.Management.Clusters("").Controller().Lister(),
		projectLister:           cluster.Management.Management.Projects(cluster.ClusterName).Controller().Lister(),
		workloadFetcher:         workloadFetcher,
	}

	projectAlertLifecycle := &ProjectAlertLifecycle{
		podWatcher: podWatcher,
	}
	projectAlertPolicies.AddClusterScopedLifecycle(ctx, "pod-target-alert-watcher", cluster.ClusterName, projectAlertLifecycle)

	go podWatcher.watch(ctx, syncInterval)
}

func (w *PodWatcher) watch(ctx context.Context, interval time.Duration) {
	for range ticker.Context(ctx, interval) {
		err := w.watchRule()
		if err != nil {
			logrus.Infof("Failed to watch pod, error: %v", err)
		}
	}
}

type ProjectAlertLifecycle struct {
	podWatcher *PodWatcher
}

func (l *ProjectAlertLifecycle) Create(obj *v3.ProjectAlertRule) (runtime.Object, error) {
	return obj, nil
}

func (l *ProjectAlertLifecycle) Updated(obj *v3.ProjectAlertRule) (runtime.Object, error) {
	return obj, nil
}

func (l *ProjectAlertLifecycle) Remove(obj *v3.ProjectAlertRule) (runtime.Object, error) {
	l.podWatcher.podRestartTrack.Delete(obj.Namespace + ":" + obj.Name)
	return obj, nil
}

func (w *PodWatcher) watchRule() error {
	if w.alertManager.IsDeploy == false {
		return nil
	}

	projectAlerts, err := w.projectAlertGroupLister.List("", labels.NewSelector())
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
		if alert.Status.AlertState == "inactive" || alert.Spec.PodRule == nil {
			continue
		}

		parts := strings.Split(alert.Spec.PodRule.PodName, ":")
		if len(parts) < 2 {
			//TODO: for invalid format pod
			if err = w.projectAlertPolicies.DeleteNamespaced(alert.Namespace, alert.Name, &metav1.DeleteOptions{}); err != nil {
				return err
			}
			continue
		}

		ns := parts[0]
		podID := parts[1]
		newPod, err := w.podLister.Get(ns, podID)
		if err != nil {
			//TODO: what to do when pod not found
			if kerrors.IsNotFound(err) || newPod == nil {
				if err = w.projectAlertPolicies.DeleteNamespaced(alert.Namespace, alert.Name, &metav1.DeleteOptions{}); err != nil {
					return err
				}
			}
			logrus.Debugf("Failed to get pod %s: %v", podID, err)

			continue
		}

		switch alert.Spec.PodRule.Condition {
		case "notrunning":
			w.checkPodRunning(newPod, alert)
		case "notscheduled":
			w.checkPodScheduled(newPod, alert)
		case "restarts":
			w.checkPodRestarts(newPod, alert)
		}
	}

	return nil
}

func (w *PodWatcher) checkPodRestarts(pod *corev1.Pod, alert *v3.ProjectAlertRule) {

	for _, containerStatus := range pod.Status.ContainerStatuses {
		curCount := containerStatus.RestartCount
		preCount := w.getRestartTimeFromTrack(alert, curCount)

		if curCount-preCount >= int32(alert.Spec.PodRule.RestartTimes) {
			ruleID := common.GetRuleID(alert.Spec.GroupName, alert.Name)

			details := ""
			if containerStatus.State.Waiting != nil {
				details = containerStatus.State.Waiting.Message
			}

			clusterDisplayName := common.GetClusterDisplayName(w.clusterName, w.clusterLister)
			projectDisplayName := common.GetProjectDisplayName(alert.Spec.ProjectName, w.projectLister)

			data := map[string]string{}
			data["rule_id"] = ruleID
			data["group_id"] = alert.Spec.GroupName
			data["alert_name"] = alert.Spec.DisplayName
			data["alert_type"] = "podRestarts"
			data["severity"] = alert.Spec.Severity
			data["cluster_name"] = clusterDisplayName
			data["project_name"] = projectDisplayName
			data["namespace"] = pod.Namespace
			data["pod_name"] = pod.Name
			data["container_name"] = containerStatus.Name
			data["restart_times"] = strconv.Itoa(alert.Spec.PodRule.RestartTimes)
			data["restart_interval"] = strconv.Itoa(alert.Spec.PodRule.RestartIntervalSeconds)

			if details != "" {
				data["logs"] = details
			}

			workloadName, err := w.getWorkloadInfo(pod)
			if err != nil {
				logrus.Warnf("Failed to get workload info for %s:%s %v", pod.Namespace, pod.Name, err)
			}
			if workloadName != "" {
				data["workload_name"] = workloadName
			}

			if err := w.alertManager.SendAlert(data); err != nil {
				logrus.Debugf("Error occurred while getting pod %s: %v", alert.Spec.PodRule.PodName, err)
			}
		}

		return
	}

}

func (w *PodWatcher) getRestartTimeFromTrack(alert *v3.ProjectAlertRule, curCount int32) int32 {
	name := alert.Name
	namespace := alert.Namespace
	now := time.Now()
	currentRestartTrack := restartTrack{Count: curCount, Time: now}
	currentRestartTrackArr := []restartTrack{currentRestartTrack}

	obj, loaded := w.podRestartTrack.LoadOrStore(namespace+":"+name, currentRestartTrackArr)
	if loaded {
		tracks := obj.([]restartTrack)
		for i, track := range tracks {
			if now.Sub(track.Time).Seconds() < float64(alert.Spec.PodRule.RestartIntervalSeconds) {
				tracks = tracks[i:]
				tracks = append(tracks, currentRestartTrack)
				w.podRestartTrack.Store(namespace+":"+name, tracks)
				return track.Count
			}
		}
	}

	return curCount
}

func (w *PodWatcher) checkPodRunning(pod *corev1.Pod, alert *v3.ProjectAlertRule) {
	if !w.checkPodScheduled(pod, alert) {
		return
	}

	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Running == nil {
			ruleID := common.GetRuleID(alert.Spec.GroupName, alert.Name)

			//TODO: need to consider all the cases
			details := ""
			if containerStatus.State.Waiting != nil {
				details = containerStatus.State.Waiting.Message
			}

			if containerStatus.State.Terminated != nil {
				details = containerStatus.State.Terminated.Message
			}

			clusterDisplayName := common.GetClusterDisplayName(w.clusterName, w.clusterLister)
			projectDisplayName := common.GetProjectDisplayName(alert.Spec.ProjectName, w.projectLister)

			data := map[string]string{}
			data["rule_id"] = ruleID
			data["group_id"] = alert.Spec.GroupName
			data["alert_name"] = alert.Spec.DisplayName
			data["alert_type"] = "podNotRunning"
			data["severity"] = alert.Spec.Severity
			data["cluster_name"] = clusterDisplayName
			data["namespace"] = pod.Namespace
			data["project_name"] = projectDisplayName
			data["pod_name"] = pod.Name
			data["container_name"] = containerStatus.Name

			if details != "" {
				data["logs"] = details
			}

			workloadName, err := w.getWorkloadInfo(pod)
			if err != nil {
				logrus.Warnf("Failed to get workload info for %s:%s %v", pod.Namespace, pod.Name, err)
			}
			if workloadName != "" {
				data["workload_name"] = workloadName
			}

			if err := w.alertManager.SendAlert(data); err != nil {
				logrus.Debugf("Error occurred while send alert %s: %v", alert.Spec.PodRule.PodName, err)
			}
			return
		}
	}
}

func (w *PodWatcher) checkPodScheduled(pod *corev1.Pod, alert *v3.ProjectAlertRule) bool {

	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodScheduled && condition.Status == corev1.ConditionFalse {
			ruleID := common.GetRuleID(alert.Spec.GroupName, alert.Name)
			details := condition.Message

			clusterDisplayName := common.GetClusterDisplayName(w.clusterName, w.clusterLister)
			projectDisplayName := common.GetProjectDisplayName(alert.Spec.ProjectName, w.projectLister)

			data := map[string]string{}
			data["rule_id"] = ruleID
			data["group_id"] = alert.Spec.GroupName
			data["alert_type"] = "podNotScheduled"
			data["alert_name"] = alert.Spec.DisplayName
			data["severity"] = alert.Spec.Severity
			data["cluster_name"] = clusterDisplayName
			data["namespace"] = pod.Namespace
			data["project_name"] = projectDisplayName
			data["pod_name"] = pod.Name

			if details != "" {
				data["logs"] = details
			}

			workloadName, err := w.getWorkloadInfo(pod)
			if err != nil {
				logrus.Warnf("Failed to get workload info for %s:%s %v", pod.Namespace, pod.Name, err)
			}
			if workloadName != "" {
				data["workload_name"] = workloadName
			}

			if err := w.alertManager.SendAlert(data); err != nil {
				logrus.Debugf("Error occurred while getting pod %s: %v", alert.Spec.PodRule.PodName, err)
			}
			return false
		}
	}

	return true

}

func (w *PodWatcher) getWorkloadInfo(pod *corev1.Pod) (string, error) {
	if len(pod.OwnerReferences) == 0 {
		return pod.Name, nil
	}
	ownerRef := pod.OwnerReferences[0]
	workloadName, err := w.workloadFetcher.getWorkloadName(pod.Namespace, ownerRef.Name, ownerRef.Kind)
	if err != nil {
		return "", errors.Wrap(err, "Failed to get workload info for alert")
	}
	return workloadName, nil
}
