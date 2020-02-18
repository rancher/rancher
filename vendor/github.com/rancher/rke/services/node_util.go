package services

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/k8s"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/drain"
)

func checkNodeReady(kubeClient *kubernetes.Clientset, runHost *hosts.Host, component string) error {
	for retries := 0; retries < k8s.MaxRetries; retries++ {
		logrus.Debugf("[%s] Now checking status of node %v", component, runHost.HostnameOverride)
		k8sNode, err := k8s.GetNode(kubeClient, runHost.HostnameOverride)
		if err != nil {
			return fmt.Errorf("[%s] Error getting node %v: %v", component, runHost.HostnameOverride, err)
		}
		logrus.Debugf("[%s] Found node by name %s", component, runHost.HostnameOverride)
		if k8s.IsNodeReady(*k8sNode) {
			return nil
		}
		time.Sleep(time.Second * k8s.RetryInterval)
	}
	return fmt.Errorf("host %v not ready", runHost.HostnameOverride)
}

func cordonAndDrainNode(kubeClient *kubernetes.Clientset, host *hosts.Host, drainNode bool, drainHelper drain.Helper, component string) error {
	logrus.Debugf("[%s] Cordoning node %v", component, host.HostnameOverride)
	if err := k8s.CordonUncordon(kubeClient, host.HostnameOverride, true); err != nil {
		return err
	}
	if !drainNode {
		return nil
	}
	logrus.Debugf("[%s] Draining node %v", component, host.HostnameOverride)
	if err := drain.RunNodeDrain(&drainHelper, host.HostnameOverride); err != nil {
		return fmt.Errorf("error draining node %v: %v", host.HostnameOverride, err)
	}
	return nil
}

func getDrainHelper(kubeClient *kubernetes.Clientset, upgradeStrategy v3.NodeUpgradeStrategy) drain.Helper {
	drainHelper := drain.Helper{
		Client:              kubeClient,
		Force:               upgradeStrategy.DrainInput.Force,
		IgnoreAllDaemonSets: upgradeStrategy.DrainInput.IgnoreDaemonSets,
		DeleteLocalData:     upgradeStrategy.DrainInput.DeleteLocalData,
		GracePeriodSeconds:  upgradeStrategy.DrainInput.GracePeriod,
		Timeout:             time.Second * time.Duration(upgradeStrategy.DrainInput.Timeout),
		Out:                 bytes.NewBuffer([]byte{}),
		ErrOut:              bytes.NewBuffer([]byte{}),
	}
	return drainHelper
}

func getNodeListForUpgrade(kubeClient *kubernetes.Clientset, hostsFailed *sync.Map, newHosts map[string]bool, isUpgradeForWorkerPlane bool) ([]v1.Node, error) {
	var nodeList []v1.Node
	nodes, err := k8s.GetNodeList(kubeClient)
	if err != nil {
		return nodeList, err
	}
	for _, node := range nodes.Items {
		if isUpgradeForWorkerPlane {
			// exclude hosts that are already included in failed hosts list
			if _, ok := hostsFailed.Load(node.Name); ok {
				continue
			}
		}
		// exclude hosts that are newly added to the cluster since they can take time to come up
		if newHosts[node.Name] {
			continue
		}
		nodeList = append(nodeList, node)
	}
	return nodeList, nil
}
