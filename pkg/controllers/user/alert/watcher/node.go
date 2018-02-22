package watcher

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rancher/rancher/pkg/controllers/user/alert/manager"
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
		if alert.Spec.TargetNode.NodeName != "" {
			parts := strings.Split(alert.Spec.TargetNode.NodeName, ":")
			id := parts[1]
			newNode, err := w.nodeLister.Get("", id)
			if err != nil {
				logrus.Debugf("Failed to get node %s: %v", id, err)
				continue
			}
			machine := getMachineByNodeName(machines, newNode.Name)
			w.checkNodeCondition(newNode, alert, machine)

		} else if alert.Spec.TargetNode.Selector != nil {

			selector := labels.NewSelector()
			for key, value := range alert.Spec.TargetNode.Selector {
				r, _ := labels.NewRequirement(key, selection.Equals, []string{value})
				selector.Add(*r)
			}
			nodes, err := w.nodeLister.List("", selector)
			if err != nil {
				continue
			}
			for _, node := range nodes {
				machine := getMachineByNodeName(machines, node.Name)
				w.checkNodeCondition(node, alert, machine)
			}
		}
	}
	return nil
}

func getMachineByNodeName(machines []*v3.Node, nodeName string) *v3.Node {
	for _, m := range machines {
		if m.Status.NodeName == nodeName {
			return m
		}
	}

	return nil

}

func (w *NodeWatcher) checkNodeCondition(node *corev1.Node, alert *v3.ClusterAlert, machine *v3.Node) {
	switch alert.Spec.TargetNode.Condition {
	case "notready":
		w.checkNodeReady(node, alert)
	case "mem":
		w.checkNodeMemUsage(node, alert, machine)
	case "cpu":
		w.checkNodeCPUUsage(node, alert, machine)
	}
}

func (w *NodeWatcher) checkNodeMemUsage(node *corev1.Node, alert *v3.ClusterAlert, machine *v3.Node) {
	alertID := alert.Namespace + "-" + alert.Name
	if machine != nil {
		total := machine.Status.InternalNodeStatus.Allocatable.Memory()
		used := machine.Status.Requested.Memory()

		if used.Value()*100.0/total.Value() > int64(alert.Spec.TargetNode.MemThreshold) {
			title := fmt.Sprintf("The memory usage on the node %s is over %s%%", node.Name, strconv.Itoa(alert.Spec.TargetNode.MemThreshold))
			//TODO: how to set unit for display for Quantity
			desc := fmt.Sprintf("*Alert Name*: %s\n*Cluster Name*: %s\n*Used Memory*: %s\n*Total Memory*: %s", alert.Spec.DisplayName, w.clusterName, used.String(), total.String())

			if err := w.alertManager.SendAlert(alertID, desc, title, alert.Spec.Severity); err != nil {
				logrus.Debugf("Failed to send alert: %v", err)
			}
		}
	}
}

func (w *NodeWatcher) checkNodeCPUUsage(node *corev1.Node, alert *v3.ClusterAlert, machine *v3.Node) {
	alertID := alert.Namespace + "-" + alert.Name
	if machine != nil {
		total := machine.Status.InternalNodeStatus.Allocatable.Cpu()
		used := machine.Status.Requested.Cpu()

		if used.MilliValue()*100.0/total.MilliValue() > int64(alert.Spec.TargetNode.CPUThreshold) {
			title := fmt.Sprintf("The CPU usage on the node %s is over %s%%", node.Name, strconv.Itoa(alert.Spec.TargetNode.CPUThreshold))
			desc := fmt.Sprintf("*Alert Name*: %s\n*Cluster Name*: %s\n*Used CPU*: %s m\n*Total CPU*: %s m", alert.Spec.DisplayName, w.clusterName, strconv.FormatInt(used.MilliValue(), 10), strconv.FormatInt(total.MilliValue(), 10))

			if err := w.alertManager.SendAlert(alertID, desc, title, alert.Spec.Severity); err != nil {
				logrus.Debugf("Failed to send alert: %v", err)
			}
		}
	}
}

func (w *NodeWatcher) checkNodeReady(node *corev1.Node, alert *v3.ClusterAlert) {
	alertID := alert.Namespace + "-" + alert.Name
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			if cond.Status == corev1.ConditionFalse {

				title := fmt.Sprintf("The kubelet on the node %s is not healthy", node.Name)
				desc := fmt.Sprintf("*Alert Name*: %s\n*Cluster Name*: %s\n*Logs*: %s", alert.Spec.DisplayName, w.clusterName, cond.Message)

				if err := w.alertManager.SendAlert(alertID, desc, title, alert.Spec.Severity); err != nil {
					logrus.Debugf("Failed to send alert: %v", err)
				}
				return
			}
		}

	}

}
