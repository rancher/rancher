package nodesyncer

import (
	"fmt"

	"strings"

	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/kubectl"
	nodehelper "github.com/rancher/rancher/pkg/node"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	drainTokenPrefix = "drain-node-"
	description      = "token for drain"
)

func (m *NodesSyncer) syncCordonFields(key string, obj *v3.Node) error {
	if obj == nil || obj.DeletionTimestamp != nil || obj.Spec.DesiredNodeUnschedulable == "" || obj.Spec.DesiredNodeUnschedulable == "drain" {
		return nil
	}
	node, err := nodehelper.GetNodeForMachine(obj, m.nodeLister)
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
	nodeCopy := obj.DeepCopy()
	if !desiredValue {
		removeDrainCondition(nodeCopy)
	}
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
	kubeConfig, err := d.getKubeConfig()
	if err != nil {
		return err
	}
	nodeName := obj.Spec.RequestedHostname
	updatedObj, err := v3.NodeConditionDrained.DoUntilTrue(obj, func() (runtime.Object, error) {
		_, msg, err := kubectl.Drain(kubeConfig, nodeName, getFlags(obj.Spec.NodeDrainInput))
		if err != nil {
			errMsg := filterErrorMsg(msg, nodeName)
			return obj, fmt.Errorf("%s", errMsg)
		}
		return obj, nil
	})
	nodeCopy := updatedObj.(*v3.Node).DeepCopy()
	if err == nil {
		nodeCopy.Spec.DesiredNodeUnschedulable = ""
	}
	if _, err := d.machines.Update(nodeCopy); err != nil {
		return err
	}
	if err != nil {
		return fmt.Errorf("Error draining node [%s] in cluster [%s] : %s", nodeName, d.clusterName, err)
	}
	return nil
}

func (d *NodeDrain) getKubeConfig() (*clientcmdapi.Config, error) {
	cluster, err := d.clusterLister.Get("", d.clusterName)
	if err != nil {
		return nil, err
	}
	user, err := d.systemAccountManager.GetSystemUser(cluster)
	if err != nil {
		return nil, err
	}
	token, err := d.userManager.EnsureToken(drainTokenPrefix+user.Name, description, user.Name)
	if err != nil {
		return nil, err
	}
	kubeConfig := d.kubeConfigGetter.KubeConfig(d.clusterName, token)
	for k := range kubeConfig.Clusters {
		kubeConfig.Clusters[k].InsecureSkipTLSVerify = true
	}
	return kubeConfig, nil
}

func getFlags(input *v3.NodeDrainInput) []string {
	return []string{
		fmt.Sprintf("--delete-local-data=%v", input.DeleteLocalData),
		fmt.Sprintf("--force=%v", input.Force),
		fmt.Sprintf("--grace-period=%v", input.GracePeriod),
		fmt.Sprintf("--ignore-daemonsets=%v", input.IgnoreDaemonSets),
		fmt.Sprintf("--timeout=%s", convert.ToString(input.Timeout)+"s")}
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
