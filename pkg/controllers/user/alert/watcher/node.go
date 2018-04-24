package watcher

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rancher/rancher/pkg/controllers/user/alert/manager"
	nodeHelper "github.com/rancher/rancher/pkg/node"
	"github.com/rancher/rancher/pkg/ticker"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

type NodeWatcher struct {
	machineLister      v3.NodeLister
	nodeLister         v1.NodeLister
	clusterAlertLister v3.ClusterAlertLister
	alertManager       *manager.Manager
	clusterName        string
}

func StartNodeWatcher(ctx context.Context, cluster *config.UserContext, manager *manager.Manager) {

	n := &NodeWatcher{
		machineLister:      cluster.Management.Management.Nodes(cluster.ClusterName).Controller().Lister(),
		nodeLister:         cluster.Core.Nodes("").Controller().Lister(),
		clusterAlertLister: cluster.Management.Management.ClusterAlerts(cluster.ClusterName).Controller().Lister(),
		alertManager:       manager,
		clusterName:        cluster.ClusterName,
	}
	go n.watch(ctx, syncInterval)
}

func (w *NodeWatcher) watch(ctx context.Context, interval time.Duration) {
	for range ticker.Context(ctx, interval) {
		err := w.watchRule()
		if err != nil {
			logrus.Infof("Failed to watch node", err)
		}
	}
}

func (w *NodeWatcher) watchRule() error {
	if w.alertManager.IsDeploy == false {
		return nil
	}

	clusterAlerts, err := w.clusterAlertLister.List("", labels.NewSelector())
	if err != nil {
		return err
	}

	machines, err := w.machineLister.List("", labels.NewSelector())
	if err != nil {
		return err
	}

	for _, alert := range clusterAlerts {
		if alert.Status.AlertState == "inactive" {
			continue
		}

		if alert.Spec.TargetNode == nil {
			continue
		}

		if alert.Spec.TargetNode.NodeName != "" {
			parts := strings.Split(alert.Spec.TargetNode.NodeName, ":")
			if len(parts) != 2 {
				continue
			}
			id := parts[1]
			machine := getMachineByID(machines, id)
			w.checkNodeCondition(alert, machine)

		} else if alert.Spec.TargetNode.Selector != nil {

			selector := labels.NewSelector()
			for key, value := range alert.Spec.TargetNode.Selector {
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

func (w *NodeWatcher) checkNodeCondition(alert *v3.ClusterAlert, machine *v3.Node) {
	switch alert.Spec.TargetNode.Condition {
	case "notready":
		w.checkNodeReady(alert, machine)
	case "mem":
		w.checkNodeMemUsage(alert, machine)
	case "cpu":
		w.checkNodeCPUUsage(alert, machine)
	}
}

func (w *NodeWatcher) checkNodeMemUsage(alert *v3.ClusterAlert, machine *v3.Node) {
	alertID := alert.Namespace + "-" + alert.Name
	if machine != nil {
		total := machine.Status.InternalNodeStatus.Allocatable.Memory()
		used := machine.Status.Requested.Memory()

		if used.Value()*100.0/total.Value() > int64(alert.Spec.TargetNode.MemThreshold) {
			title := fmt.Sprintf("The memory usage on the node %s is over %s%%", nodeHelper.GetNodeName(machine), strconv.Itoa(alert.Spec.TargetNode.MemThreshold))
			//TODO: how to set unit for display for Quantity
			desc := fmt.Sprintf("*Alert Name*: %s\n*Severity*: %s\n*Cluster Name*: %s\n*Used Memory*: %s\n*Total Memory*: %s", alert.Spec.DisplayName, alert.Spec.Severity, w.clusterName, used.String(), total.String())

			if err := w.alertManager.SendAlert(alertID, desc, title, alert.Spec.Severity); err != nil {
				logrus.Debugf("Failed to send alert: %v", err)
			}
		}
	}
}

func (w *NodeWatcher) checkNodeCPUUsage(alert *v3.ClusterAlert, machine *v3.Node) {
	alertID := alert.Namespace + "-" + alert.Name
	if machine != nil {
		total := machine.Status.InternalNodeStatus.Allocatable.Cpu()
		used := machine.Status.Requested.Cpu()

		if used.MilliValue()*100.0/total.MilliValue() > int64(alert.Spec.TargetNode.CPUThreshold) {
			title := fmt.Sprintf("The CPU usage on the node %s is over %s%%", nodeHelper.GetNodeName(machine), strconv.Itoa(alert.Spec.TargetNode.CPUThreshold))
			desc := fmt.Sprintf("*Alert Name*: %s\n*Severity*: %s\n*Cluster Name*: %s\n*Used CPU*: %s m\n*Total CPU*: %s m", alert.Spec.DisplayName, alert.Spec.Severity, w.clusterName, strconv.FormatInt(used.MilliValue(), 10), strconv.FormatInt(total.MilliValue(), 10))

			if err := w.alertManager.SendAlert(alertID, desc, title, alert.Spec.Severity); err != nil {
				logrus.Debugf("Failed to send alert: %v", err)
			}
		}
	}
}

func (w *NodeWatcher) checkNodeReady(alert *v3.ClusterAlert, machine *v3.Node) {
	alertID := alert.Namespace + "-" + alert.Name
	for _, cond := range machine.Status.InternalNodeStatus.Conditions {
		if cond.Type == corev1.NodeReady {
			if cond.Status != corev1.ConditionTrue {

				title := fmt.Sprintf("The kubelet on the node %s is not healthy", nodeHelper.GetNodeName(machine))
				desc := fmt.Sprintf("*Alert Name*: %s\n*Severity*: %s\n*Cluster Name*: %s", alert.Spec.DisplayName, alert.Spec.Severity, w.clusterName)

				if cond.Message != "" {
					desc = desc + fmt.Sprintf("\n*Logs*: %s", cond.Message)
				}
				if err := w.alertManager.SendAlert(alertID, desc, title, alert.Spec.Severity); err != nil {
					logrus.Debugf("Failed to send alert: %v", err)
				}
				return
			}
		}
	}
}
