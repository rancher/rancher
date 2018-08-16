package nodesyncer

import (
	"fmt"

	"strings"

	"context"

	"sync"

	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/kubectl"
	nodehelper "github.com/rancher/rancher/pkg/node"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	drainTokenPrefix = "drain-node-"
	description      = "token for drain"
)

var nodeMapLock = sync.Mutex{}

func (m *NodesSyncer) syncCordonFields(key string, obj *v3.Node) error {
	if obj == nil || obj.DeletionTimestamp != nil || obj.Spec.DesiredNodeUnschedulable == "" {
		return nil
	}

	if obj.Spec.DesiredNodeUnschedulable != "true" && obj.Spec.DesiredNodeUnschedulable != "false" {
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
	if obj == nil || obj.DeletionTimestamp != nil || obj.Spec.DesiredNodeUnschedulable == "" {
		return nil
	}

	if obj.Spec.DesiredNodeUnschedulable != "drain" && obj.Spec.DesiredNodeUnschedulable != "stopDrain" {
		return nil
	}

	defer nodeMapLock.Unlock()
	if obj.Spec.DesiredNodeUnschedulable == "drain" {
		nodeMapLock.Lock()
		if _, ok := d.nodesToContext[obj.Name]; ok {
			return nil
		}
		ctx, cancel := context.WithCancel(d.ctx)
		d.nodesToContext[obj.Name] = cancel
		go d.drain(ctx, obj, cancel)

	} else if obj.Spec.DesiredNodeUnschedulable == "stopDrain" {
		nodeMapLock.Lock()
		cancelFunc, ok := d.nodesToContext[obj.Name]
		nodeMapLock.Unlock()
		if ok {
			cancelFunc()
		}
		return d.resetDesiredNodeUnschedulable(obj)
	}
	return nil
}

func (d *NodeDrain) drain(ctx context.Context, obj *v3.Node, cancel context.CancelFunc) {
	defer deleteFromContextMap(d.nodesToContext, obj.Name)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		stopped := false
		nodeName := obj.Spec.RequestedHostname
		updatedObj, err := v3.NodeConditionDrained.DoUntilTrue(obj, func() (runtime.Object, error) {
			kubeConfig, err := d.getKubeConfig()
			if err != nil {
				if err == context.Canceled {
					stopped = true
					logrus.Infof(fmt.Sprintf("Stopped draining %s in %s", obj.Name, obj.ClusterName))
				}
				logrus.Errorf("nodeDrain: error getting kubeConfig for node %s", obj.Name)
				return obj, fmt.Errorf("error getting kubeConfig for node %s", obj.Name)
			}
			_, msg, err := kubectl.Drain(ctx, kubeConfig, nodeName, getFlags(obj.Spec.NodeDrainInput))
			if err != nil {
				errMsg := filterErrorMsg(msg, nodeName)
				return obj, fmt.Errorf("%s", errMsg)
			}
			return obj, nil
		})
		if !stopped {
			nodeCopy := updatedObj.(*v3.Node).DeepCopy()
			if err == nil {
				nodeCopy.Spec.DesiredNodeUnschedulable = ""
			}
			_, updateErr := d.machines.Update(nodeCopy)
			if err != nil || updateErr != nil {
				logrus.Errorf("nodeDrain: [%s] in cluster [%s] : %v %v", nodeName, d.clusterName, err, updateErr)
				d.machines.Controller().Enqueue("", fmt.Sprintf("%s/%s", d.clusterName, obj.Name))
			}
		}
		cancel()
	}
}

func (d *NodeDrain) resetDesiredNodeUnschedulable(obj *v3.Node) error {
	nodeCopy := obj.DeepCopy()
	removeDrainCondition(nodeCopy)
	nodeCopy.Spec.DesiredNodeUnschedulable = ""
	if _, err := d.machines.Update(nodeCopy); err != nil {
		return err
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

func deleteFromContextMap(data map[string]context.CancelFunc, id string) {
	nodeMapLock.Lock()
	delete(data, id)
	nodeMapLock.Unlock()
}
