package rkeworkerupgrader

import (
	"fmt"

	nodehelper "github.com/rancher/rancher/pkg/node"
	nodeserver "github.com/rancher/rancher/pkg/rkenodeconfigserver"
	rkeservices "github.com/rancher/rke/services"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

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

func (uh *upgradeHandler) processNode(node *v3.Node, cluster *v3.Cluster, msg string) error {
	nodePlan, err := uh.getNodePlan(node, cluster)
	if err != nil {
		return fmt.Errorf("setNodePlan: error getting node plan for [%s]: %v", node.Name, err)
	}

	nodeCopy := node.DeepCopy()
	nodeCopy.Status.NodePlan.Plan = nodePlan
	nodeCopy.Status.NodePlan.Version = cluster.Status.NodeVersion
	nodeCopy.Status.NodePlan.AgentCheckInterval = nodeserver.AgentCheckIntervalDuringUpgrade

	v3.NodeConditionUpgraded.Unknown(nodeCopy)
	v3.NodeConditionUpgraded.Message(nodeCopy, msg)

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

	// skip nodes marked for ignore by user
	if ignoreNode(node) {
		logrus.Debugf("cluster [%s] worker-upgrade: node [%s] is marked with ignoreLabel %s: %v", clusterName, node.Name, ignoreKey, ignoreValue)
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

func (uh *upgradeHandler) filterNodes(nodes []*v3.Node, expectedVersion int) (map[string]*v3.Node, map[string]*v3.Node, map[string]*v3.Node, map[string]*v3.Node, int, int, int) {
	done, upgrading, filtered := 0, 0, 0
	toPrepareMap, toProcessMap, upgradedMap, notReadyMap := map[string]*v3.Node{}, map[string]*v3.Node{}, map[string]*v3.Node{}, map[string]*v3.Node{}

	for _, node := range nodes {

		if skipNode(node) {
			continue
		}

		filtered++

		// check for nodeConditionReady
		if !nodehelper.IsMachineReady(node) {
			notReadyMap[node.Name] = node
			logrus.Debugf("cluster [%s] worker-upgrade: node [%s] is not ready", node.Namespace, node.Name)
			continue
		}

		if node.Status.AppliedNodeVersion == expectedVersion {
			if v3.NodeConditionUpgraded.IsUnknown(node) {
				upgradedMap[node.Name] = node
			}
			if !node.Spec.InternalNodeSpec.Unschedulable {
				logrus.Debugf("cluster [%s] worker-upgrade: node [%s] is done", node.Namespace, node.Name)
				done++
			} else {
				// node hasn't un-cordoned, so consider it upgrading in terms of maxUnavailable count
				upgrading++
			}
			continue
		}

		if preparingNode(node) {
			// draining or cordoning
			upgrading++
			continue
		}

		if preparedNode(node) {
			// node ready to upgrade
			upgrading++
			toProcessMap[node.Name] = node
			continue
		}

		toPrepareMap[node.Name] = node
	}

	return toPrepareMap, toProcessMap, upgradedMap, notReadyMap, filtered, upgrading, done
}

func preparingNode(node *v3.Node) bool {
	return node.Spec.DesiredNodeUnschedulable == "drain" || node.Spec.DesiredNodeUnschedulable == "true"
}

func preparedNode(node *v3.Node) bool {
	return v3.NodeConditionDrained.IsTrue(node) || node.Spec.InternalNodeSpec.Unschedulable || v3.NodeConditionUpgraded.IsUnknown(node)
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

func ignoreNode(node *v3.Node) bool {
	return node.Status.NodeLabels != nil && node.Status.NodeLabels[ignoreKey] == ignoreValue
}
