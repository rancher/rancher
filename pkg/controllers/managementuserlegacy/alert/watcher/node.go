package watcher

import (
	"context"
	"strconv"
	"strings"
	"time"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/alert/common"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/alert/manager"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	nodeHelper "github.com/rancher/rancher/pkg/node"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v2/pkg/ticker"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

type NodeWatcher struct {
	machineLister          v3.NodeLister
	nodeLister             v1.NodeLister
	clusterAlertRule       v3.ClusterAlertRuleInterface
	clusterAlertRuleLister v3.ClusterAlertRuleLister
	alertManager           *manager.AlertManager
	clusterName            string
	clusterLister          v3.ClusterLister
}

func StartNodeWatcher(ctx context.Context, cluster *config.UserContext, manager *manager.AlertManager) {
	clusterAlerts := cluster.Management.Management.ClusterAlertRules(cluster.ClusterName)
	n := &NodeWatcher{
		machineLister:          cluster.Management.Management.Nodes(cluster.ClusterName).Controller().Lister(),
		nodeLister:             cluster.Core.Nodes("").Controller().Lister(),
		clusterAlertRule:       clusterAlerts,
		clusterAlertRuleLister: clusterAlerts.Controller().Lister(),
		alertManager:           manager,
		clusterName:            cluster.ClusterName,
		clusterLister:          cluster.Management.Management.Clusters("").Controller().Lister(),
	}
	go n.watch(ctx, syncInterval)
}

func (w *NodeWatcher) watch(ctx context.Context, interval time.Duration) {
	for range ticker.Context(ctx, interval) {
		err := w.watchRule()
		if err != nil {
			logrus.Infof("Failed to watch node, error: %v", err)
		}
	}
}

func (w *NodeWatcher) watchRule() error {
	if w.alertManager.IsDeploy == false {
		return nil
	}

	clusterAlerts, err := w.clusterAlertRuleLister.List("", labels.NewSelector())
	if err != nil {
		return err
	}

	machines, err := w.machineLister.List("", labels.NewSelector())
	if err != nil {
		return err
	}

	for _, alert := range clusterAlerts {
		if alert.Status.AlertState == "inactive" || alert.Spec.NodeRule == nil {
			continue
		}

		if alert.Spec.NodeRule.NodeName != "" {
			parts := strings.Split(alert.Spec.NodeRule.NodeName, ":")
			if len(parts) != 2 {
				continue
			}
			id := parts[1]
			machine := getMachineByID(machines, id)
			if machine == nil {
				if err = w.clusterAlertRule.Delete(alert.Name, &metav1.DeleteOptions{}); err != nil {
					return err
				}
				continue
			}
			w.checkNodeCondition(alert, machine)

		} else if alert.Spec.NodeRule.Selector != nil {

			selector := labels.NewSelector()
			for key, value := range alert.Spec.NodeRule.Selector {
				r, err := labels.NewRequirement(key, selection.Equals, []string{value})
				if err != nil {
					logrus.Warnf("Fail to create new requirement foo %s: %v", key, err)
					continue
				}
				selector = selector.Add(*r)
			}
			nodes, err := w.nodeLister.List("", selector)
			if err != nil {
				logrus.Warnf("Fail to list node: %v", err)
				continue
			}
			for _, node := range nodes {
				machine := nodeHelper.GetNodeByNodeName(machines, node.Name)
				// handle the case when v3.node can't be found for v1.node
				if machine == nil {
					logrus.Warnf("Failed to find node %s", node.Name)
					continue
				}
				w.checkNodeCondition(alert, machine)
			}
		}

	}

	return nil
}

func getMachineByID(machines []*v3.Node, id string) *v3.Node {
	for _, m := range machines {
		if m.Name == id {
			return m
		}
	}
	return nil
}

func (w *NodeWatcher) checkNodeCondition(alert *v3.ClusterAlertRule, machine *v3.Node) {
	switch alert.Spec.NodeRule.Condition {
	case "notready":
		w.checkNodeReady(alert, machine)
	case "mem":
		w.checkNodeMemUsage(alert, machine)
	case "cpu":
		w.checkNodeCPUUsage(alert, machine)
	}
}

func (w *NodeWatcher) checkNodeMemUsage(alert *v3.ClusterAlertRule, machine *v3.Node) {
	if v32.NodeConditionProvisioned.IsTrue(machine) {
		total := machine.Status.InternalNodeStatus.Allocatable.Memory()
		used := machine.Status.Requested.Memory()

		if used.Value()*100.0/total.Value() > int64(alert.Spec.NodeRule.MemThreshold) {
			ruleID := common.GetRuleID(alert.Spec.GroupName, alert.Name)

			clusterDisplayName := common.GetClusterDisplayName(w.clusterName, w.clusterLister)

			data := map[string]string{}
			data["rule_id"] = ruleID
			data["group_id"] = alert.Spec.GroupName
			data["alert_name"] = alert.Spec.DisplayName
			data["alert_type"] = "nodeMemory"
			data["severity"] = alert.Spec.Severity
			data["cluster_name"] = clusterDisplayName
			data["mem_threshold"] = strconv.Itoa(alert.Spec.NodeRule.MemThreshold)
			data["used_mem"] = used.String()
			data["total_mem"] = total.String()
			data["node_name"] = nodeHelper.GetNodeName(machine)

			if err := w.alertManager.SendAlert(data); err != nil {
				logrus.Debugf("Failed to send alert: %v", err)
			}
		}
	}
}

func (w *NodeWatcher) checkNodeCPUUsage(alert *v3.ClusterAlertRule, machine *v3.Node) {
	if v32.NodeConditionProvisioned.IsTrue(machine) {
		total := machine.Status.InternalNodeStatus.Allocatable.Cpu()
		used := machine.Status.Requested.Cpu()
		if used.MilliValue()*100.0/total.MilliValue() > int64(alert.Spec.NodeRule.CPUThreshold) {
			ruleID := common.GetRuleID(alert.Spec.GroupName, alert.Name)

			clusterDisplayName := common.GetClusterDisplayName(w.clusterName, w.clusterLister)

			data := map[string]string{}
			data["rule_id"] = ruleID
			data["group_id"] = alert.Spec.GroupName
			data["alert_name"] = alert.Spec.DisplayName
			data["alert_type"] = "nodeCPU"
			data["severity"] = alert.Spec.Severity
			data["cluster_name"] = clusterDisplayName
			data["cpu_threshold"] = strconv.Itoa(alert.Spec.NodeRule.CPUThreshold)
			data["used_cpu"] = strconv.FormatInt(used.MilliValue(), 10)
			data["total_cpu"] = strconv.FormatInt(total.MilliValue(), 10)
			data["node_name"] = nodeHelper.GetNodeName(machine)

			if err := w.alertManager.SendAlert(data); err != nil {
				logrus.Debugf("Failed to send alert: %v", err)
			}
		}
	}
}

func (w *NodeWatcher) checkNodeReady(alert *v3.ClusterAlertRule, machine *v3.Node) {
	for _, cond := range machine.Status.InternalNodeStatus.Conditions {
		if cond.Type == corev1.NodeReady {
			if cond.Status != corev1.ConditionTrue {
				ruleID := common.GetRuleID(alert.Spec.GroupName, alert.Name)

				clusterDisplayName := common.GetClusterDisplayName(w.clusterName, w.clusterLister)

				data := map[string]string{}
				data["rule_id"] = ruleID
				data["group_id"] = alert.Spec.GroupName
				data["alert_name"] = alert.Spec.DisplayName
				data["alert_type"] = "nodeHealthy"
				data["severity"] = alert.Spec.Severity
				data["cluster_name"] = clusterDisplayName
				data["node_name"] = nodeHelper.GetNodeName(machine)

				if cond.Message != "" {
					data["logs"] = cond.Message
				}
				if err := w.alertManager.SendAlert(data); err != nil {
					logrus.Errorf("Failed to send alert: %v", err)
				}
				return
			}
		}
	}
}
