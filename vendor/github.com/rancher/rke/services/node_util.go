package services

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/k8s"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	k8sutil "k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/drain"
)

func CheckNodeReady(kubeClient *kubernetes.Clientset, runHost *hosts.Host, component string) error {
	for retries := 0; retries < k8s.MaxRetries; retries++ {
		logrus.Infof("[%s] Now checking status of node %v, try #%v", component, runHost.HostnameOverride, retries+1)
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

func getNodeListForUpgrade(kubeClient *kubernetes.Clientset, hostsFailed *sync.Map, newHosts, inactiveHosts map[string]bool, component string) ([]v1.Node, error) {
	var nodeList []v1.Node
	nodes, err := k8s.GetNodeList(kubeClient)
	if err != nil {
		return nodeList, err
	}
	logrus.Infof("[%s] Getting list of nodes for upgrade", component)
	for _, node := range nodes.Items {
		if _, ok := hostsFailed.Load(node.Labels[k8s.HostnameLabel]); ok {
			continue
		}
		// exclude hosts that are newly added to the cluster since they can take time to come up
		if newHosts[node.Labels[k8s.HostnameLabel]] {
			continue
		}
		if inactiveHosts[node.Labels[k8s.HostnameLabel]] {
			continue
		}
		nodeList = append(nodeList, node)
	}
	return nodeList, nil
}

func CalculateMaxUnavailable(maxUnavailableVal string, numHosts int, role string) (int, error) {
	// if maxUnavailable is given in percent, round down
	maxUnavailableParsed := k8sutil.Parse(maxUnavailableVal)
	logrus.Debugf("Provided value for maxUnavailable: %v", maxUnavailableParsed)
	if maxUnavailableParsed.Type == k8sutil.Int {
		if maxUnavailableParsed.IntVal <= 0 {
			return 0, fmt.Errorf("invalid input for max_unavailable_%s: %v, value must be > 0", role, maxUnavailableParsed.IntVal)
		}
	} else {
		if strings.HasPrefix(maxUnavailableParsed.StrVal, "-") || maxUnavailableParsed.StrVal == "0%" {
			return 0, fmt.Errorf("invalid input for max_unavailable_%s: %v, value must be > 0", role, maxUnavailableParsed.StrVal)
		}
	}
	maxUnavailable, err := k8sutil.GetValueFromIntOrPercent(&maxUnavailableParsed, numHosts, false)
	if err != nil {
		logrus.Errorf("Unable to parse max_unavailable, should be a number or percentage of nodes, error: %v", err)
		return 0, err
	}
	if maxUnavailable == 0 {
		// In case there is only one node and rounding down maxUnvailable percentage led to 0
		logrus.Infof("max_unavailable_%s got rounded down to 0, resetting to 1", role)
		maxUnavailable = 1
	}
	logrus.Debugf("Parsed value of maxUnavailable: %v", maxUnavailable)
	return maxUnavailable, nil
}

func ResetMaxUnavailable(maxUnavailable, lenInactiveHosts int, component string) (int, error) {
	if maxUnavailable > WorkerThreads {
		/* upgrading a large number of nodes in parallel leads to a large number of goroutines, which has led to errors regarding too many open sockets
		Because of this RKE switched to using workerpools. 50 workerthreads has been sufficient to optimize rke up, upgrading at most 50 nodes in parallel.
		So the user configurable maxUnavailable will be respected only as long as it's less than 50 and capped at 50 */
		maxUnavailable = WorkerThreads
		logrus.Infof("Resetting %s to 50, to avoid issues related to upgrading large number of nodes in parallel", "max_unavailable_"+component)
	}

	if lenInactiveHosts > 0 {
		if lenInactiveHosts >= maxUnavailable {
			return 0, fmt.Errorf("cannot proceed with upgrade of %s since %v host(s) cannot be reached prior to upgrade", component, lenInactiveHosts)
		}
		maxUnavailable -= lenInactiveHosts
		logrus.Infof("Resetting %s to %v since %v host(s) cannot be reached prior to upgrade", "max_unavailable_"+component, maxUnavailable, lenInactiveHosts)
	}
	return maxUnavailable, nil
}
