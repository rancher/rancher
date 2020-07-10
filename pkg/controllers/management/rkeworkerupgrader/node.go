package rkeworkerupgrader

import (
	"fmt"
	"sort"

	nodehelper "github.com/rancher/rancher/pkg/node"
	nodeserver "github.com/rancher/rancher/pkg/rkenodeconfigserver"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	rkeservices "github.com/rancher/rke/services"
	"github.com/sirupsen/logrus"
)

type upgradeStatus struct {
	/*
		prepare: pre-process for upgrade: cordon/drain
		process: update node plan, state: upgrading
	*/
	// active => cordon/drain
	toPrepare []*v3.Node
	// cordon/drain => upgrading
	toProcess []*v3.Node
	// upgrading => upgraded => uncordon
	upgraded []*v3.Node
	// unavailable nodes
	notReady []*v3.Node
	// upgraded active nodes
	done int
	// nodes qualified to upgrade
	filtered int
	// nodes in upgrading state
	upgrading int
}

func (uh *upgradeHandler) prepareNode(node *v3.Node, toDrain bool, nodeDrainInput *v3.NodeDrainInput) error {
	var nodeCopy *v3.Node
	if toDrain {
		if node.Spec.DesiredNodeUnschedulable == "drain" {
			return nil
		}
		nodeCopy = node.DeepCopy()
		nodeCopy.Spec.DesiredNodeUnschedulable = "drain"
		nodeCopy.Spec.NodeDrainInput = nodeDrainInput
	} else {
		if node.Spec.DesiredNodeUnschedulable == "true" || node.Spec.InternalNodeSpec.Unschedulable {
			return nil
		}
		nodeCopy = node.DeepCopy()
		nodeCopy.Spec.DesiredNodeUnschedulable = "true"
	}

	if _, err := uh.nodes.Update(nodeCopy); err != nil {
		return err
	}
	return nil
}

func (uh *upgradeHandler) setNodePlan(node *v3.Node, cluster *v3.Cluster, upgrade bool) error {
	nodePlan, err := uh.getNodePlan(node, cluster)
	if err != nil {
		return fmt.Errorf("setNodePlan: error getting node plan for [%s]: %v", node.Name, err)
	}

	nodeCopy := node.DeepCopy()
	nodeCopy.Status.NodePlan.Plan = nodePlan
	nodeCopy.Status.NodePlan.Version = cluster.Status.NodeVersion

	if upgrade {
		nodeCopy.Status.NodePlan.AgentCheckInterval = nodeserver.AgentCheckIntervalDuringUpgrade
		v3.NodeConditionUpgraded.Unknown(nodeCopy)
		v3.NodeConditionUpgraded.Message(nodeCopy, "upgrading")
	}

	if _, err := uh.nodes.Update(nodeCopy); err != nil {
		return err
	}

	return nil
}

func (uh *upgradeHandler) updateNodeActive(node *v3.Node) error {
	nodeCopy := node.DeepCopy()
	v3.NodeConditionUpgraded.True(nodeCopy)
	v3.NodeConditionUpgraded.Message(nodeCopy, "")

	// reset the node
	nodeCopy.Spec.DesiredNodeUnschedulable = "false"
	nodeCopy.Status.NodePlan.AgentCheckInterval = nodeserver.DefaultAgentCheckInterval

	if _, err := uh.nodes.Update(nodeCopy); err != nil {
		return err
	}

	return nil
}

func skipNode(node *v3.Node) bool {
	clusterName := node.Namespace
	if node.DeletionTimestamp != nil {
		logrus.Debugf("cluster [%s] worker-upgrade: node [%s] is getting deleted", clusterName, node.Name)
		return true
	}

	if node.Status.NodeConfig == nil {
		logrus.Debugf("cluster [%s] worker-upgrade: node [%s] nodeConfig is empty", clusterName, node.Name)
		return true
	}

	if !workerOnly(node.Status.NodeConfig.Role) {
		logrus.Debugf("cluster [%s] worker-upgrade: node [%s] is not a workerOnly node", clusterName, node.Name)
		return true
	}

	// skip provisioning nodes
	if !v3.NodeConditionProvisioned.IsTrue(node) {
		logrus.Debugf("cluster [%s] worker-upgrade: node [%s] is not provisioned", clusterName, node.Name)
		return true
	}

	// skip registering nodes
	if !v3.NodeConditionRegistered.IsTrue(node) {
		logrus.Debugf("cluster [%s] worker-upgrade: node [%s] is not registered", clusterName, node.Name)
		return true
	}

	return false
}

func (uh *upgradeHandler) filterNodes(nodes []*v3.Node, expectedVersion int, drain bool) *upgradeStatus {
	status := &upgradeStatus{}
	for _, node := range nodes {

		if skipNode(node) {
			continue
		}

		status.filtered++

		// check for nodeConditionReady
		if !nodehelper.IsMachineReady(node) {
			// update plan for nodes that were attempted for upgrade
			if v3.NodeConditionUpgraded.IsUnknown(node) {
				status.upgrading++
				status.toProcess = append(status.toProcess, node)
			} else {
				status.notReady = append(status.notReady, node)
			}
			logrus.Debugf("cluster [%s] worker-upgrade: node [%s] is not ready", node.Namespace, node.Name)
			continue
		}

		if node.Status.AppliedNodeVersion == expectedVersion {
			if v3.NodeConditionUpgraded.IsUnknown(node) {
				status.upgraded = append(status.upgraded, node)
			}
			if !node.Spec.InternalNodeSpec.Unschedulable {
				logrus.Debugf("cluster [%s] worker-upgrade: node [%s] is done", node.Namespace, node.Name)
				status.done++
			} else {
				// node hasn't un-cordoned, so consider it upgrading in terms of maxUnavailable count
				status.upgrading++
			}
			continue
		}

		if preparingNode(node, drain) {
			// draining or cordoning
			status.upgrading++
			continue
		}

		if preparedNode(node, drain) {
			// node ready to upgrade
			status.upgrading++
			status.toProcess = append(status.toProcess, node)
			continue
		}

		status.toPrepare = append(status.toPrepare, node)
	}

	sortByNodeName(status.toPrepare)
	sortByNodeName(status.toProcess)
	sortByNodeName(status.upgraded)
	sortByNodeName(status.notReady)

	return status
}

func sortByNodeName(arr []*v3.Node) {
	// v3.Node.Name is auto generated, format: `m-xxxx`
	sort.Slice(arr, func(i, j int) bool { return arr[i].Name < arr[j].Name })
}

func preparingNode(node *v3.Node, drain bool) bool {
	if drain {
		return node.Spec.DesiredNodeUnschedulable == "drain"
	}
	return node.Spec.DesiredNodeUnschedulable == "true"
}

func preparedNode(node *v3.Node, drain bool) bool {
	if drain {
		return v3.NodeConditionDrained.IsTrue(node)
	}
	return node.Spec.InternalNodeSpec.Unschedulable
}

func workerOnly(roles []string) bool {
	worker := false
	for _, role := range roles {
		if role == rkeservices.ETCDRole {
			return false
		}
		if role == rkeservices.ControlRole {
			return false
		}
		if role == rkeservices.WorkerRole {
			worker = true
		}
	}
	return worker
}
