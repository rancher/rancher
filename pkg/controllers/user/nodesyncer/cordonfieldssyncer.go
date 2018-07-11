package nodesyncer

import (
	"fmt"

	"strings"

	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/kubectl"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	drainTokenPrefix = "drain-node-"
	description      = "token for drain"
)

func (m *NodesSyncer) syncCordonFields(key string, obj *v3.Node) error {
	if obj == nil || obj.DeletionTimestamp != nil || obj.Spec.DesiredNodeUnschedulable == "" || obj.Spec.DesiredNodeUnschedulable == "drain" {
		return nil
	}
	nodes, err := m.nodeLister.List("", labels.NewSelector())
	if err != nil {
		return err
	}
	node, err := m.getNode(obj, nodes)
	if err != nil {
		return err
	}
	desiredValue := convert.ToBool(obj.Spec.DesiredNodeUnschedulable)
	if node.Spec.Unschedulable != desiredValue {
		toUpdate := node.DeepCopy()
		toUpdate.Spec.Unschedulable = desiredValue
		if _, err := m.nodeClient.Update(toUpdate); err != nil {
			return err
		}
	}
	if !desiredValue {
		removeDrainCondition(obj)
	}
	nodeCopy := obj.DeepCopy()
	nodeCopy.Spec.DesiredNodeUnschedulable = ""
	_, err = m.machines.Update(nodeCopy)
	if err != nil {
		return err
	}
	return nil
}

func (d *NodeDrain) drainNode(key string, obj *v3.Node) error {
	if obj == nil || obj.DeletionTimestamp != nil || obj.Spec.DesiredNodeUnschedulable != "drain" {
		return nil
	}
	cluster, err := d.clusterLister.Get("", d.clusterName)
	if err != nil {
		return err
	}
	user, err := d.systemAccountManager.GetSystemUser(cluster)
	if err != nil {
		return err
	}
	token, err := d.userManager.EnsureToken(drainTokenPrefix+user.Name, description, user.Name)
	if err != nil {
		return err
	}

	kubeConfig := d.kubeConfigGetter.KubeConfig(d.clusterName, token)
	for _, cluster := range kubeConfig.Clusters {
		if !cluster.InsecureSkipTLSVerify {
			cluster.InsecureSkipTLSVerify = true
		}
	}
	nodeName := obj.Spec.RequestedHostname
	updateDrainCondition(obj, "unknown", "")
	_, err, msg := kubectl.Drain(kubeConfig, nodeName, getFlags(obj.Spec.NodeDrainInput))
	errMsg := ""
	if err != nil {
		errMsg = filterErrorMsg(msg, nodeName)
		updateDrainCondition(obj, "false", errMsg)
	} else {
		updateDrainCondition(obj, "true", "node successfully drained")
	}

	nodeCopy := obj.DeepCopy()
	nodeCopy.Spec.DesiredNodeUnschedulable = ""
	if _, err := d.machines.Update(nodeCopy); err != nil {
		return err
	}
	if len(errMsg) > 0 {
		return fmt.Errorf("Error draining node [%s] in cluster [%s] : %s", nodeName, d.clusterName, errMsg)
	}
	return nil
}

func getFlags(input *v3.NodeDrainInput) []string {
	return []string{
		fmt.Sprintf("--delete-local-data=%v", input.DeleteLocalData),
		fmt.Sprintf("--force=%v", input.Force),
		fmt.Sprintf("--grace-period=%v", input.GracePeriod),
		fmt.Sprintf("--ignore-daemonsets=%v", input.IgnoreDaemonSets),
		fmt.Sprintf("--timeout=%s", convert.ToString(input.Timeout)+"s")}
}

func updateDrainCondition(obj *v3.Node, status string, msg string) {
	if status == "unknown" {
		v3.NodeConditionDrained.Unknown(obj)
	} else if status == "true" {
		v3.NodeConditionDrained.True(obj)
	} else if status == "false" {
		v3.NodeConditionDrained.False(obj)
	}
	v3.NodeConditionDrained.Message(obj, msg)
}

func filterErrorMsg(msg string, nodeName string) string {
	upd := []string{}
	lines := strings.Split(msg, "\n")
	for _, line := range lines[1:] {
		if strings.HasPrefix(line, "WARNING") || strings.HasPrefix(line, nodeName) {
			continue
		}
		if strings.Contains(line, "aborting") {
			continue
		}
		if strings.HasPrefix(line, "There are pending nodes ") {
			// for only one node in our case
			continue
		}
		if strings.HasPrefix(line, "There are pending pods ") {
			// already considered error
			continue
		}
		if strings.HasPrefix(line, "error") && strings.Contains(line, "unable to drain node") {
			// actual reason at end
			continue
		}
		if strings.HasPrefix(line, "pod") && strings.Contains(line, "evicted") {
			// evicted successfully
			continue
		}
		upd = append(upd, line)
	}
	return strings.Join(upd, "\n")
}

func removeDrainCondition(obj *v3.Node) {
	exists := false
	for _, condition := range obj.Status.Conditions {
		if condition.Type == "Drained" {
			exists = true
			break
		}
	}
	if exists {
		var conditions []v3.NodeCondition
		for _, condition := range obj.Status.Conditions {
			if condition.Type == "Drained" {
				continue
			}
			conditions = append(conditions, condition)
		}
		obj.Status.Conditions = conditions
	}
}
