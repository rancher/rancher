package k8s

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func DeleteNode(k8sClient *kubernetes.Clientset, nodeName string) error {
	return k8sClient.CoreV1().Nodes().Delete(nodeName, &metav1.DeleteOptions{})
}

func GetNodeList(k8sClient *kubernetes.Clientset) (*v1.NodeList, error) {
	return k8sClient.CoreV1().Nodes().List(metav1.ListOptions{})
}

func GetNode(k8sClient *kubernetes.Clientset, nodeName string) (*v1.Node, error) {
	return k8sClient.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
}
func CordonUncordon(k8sClient *kubernetes.Clientset, nodeName string, cordoned bool) error {
	updated := false
	for retries := 0; retries <= 5; retries++ {
		node, err := k8sClient.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
		if err != nil {
			logrus.Debugf("Error getting node %s: %v", nodeName, err)
			time.Sleep(time.Second * 5)
			continue
		}
		if node.Spec.Unschedulable == cordoned {
			logrus.Debugf("Node %s is already cordoned: %v", nodeName, cordoned)
			return nil
		}
		node.Spec.Unschedulable = cordoned
		_, err = k8sClient.CoreV1().Nodes().Update(node)
		if err != nil {
			logrus.Debugf("Error setting cordoned state for node %s: %v", nodeName, err)
			time.Sleep(time.Second * 5)
			continue
		}
		updated = true
	}
	if !updated {
		return fmt.Errorf("Failed to set cordonded state for node: %s", nodeName)
	}
	return nil
}

func IsNodeReady(node v1.Node) bool {
	nodeConditions := node.Status.Conditions
	for _, condition := range nodeConditions {
		if condition.Type == "Ready" && condition.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

func RemoveTaintFromNodeByKey(k8sClient *kubernetes.Clientset, nodeName, taintKey string) error {
	updated := false
	var err error
	var node *v1.Node
	for retries := 0; retries <= 5; retries++ {
		node, err = GetNode(k8sClient, nodeName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				logrus.Debugf("[hosts] Can't find node by name [%s]", nodeName)
				return nil
			}
			return err
		}
		foundTaint := false
		for i, taint := range node.Spec.Taints {
			if taint.Key == taintKey {
				foundTaint = true
				node.Spec.Taints = append(node.Spec.Taints[:i], node.Spec.Taints[i+1:]...)
				break
			}
		}
		if !foundTaint {
			return nil
		}
		_, err = k8sClient.CoreV1().Nodes().Update(node)
		if err != nil {
			logrus.Debugf("Error updating node [%s] with new set of taints: %v", node.Name, err)
			time.Sleep(time.Second * 5)
			continue
		}
		updated = true
		break
	}
	if !updated {
		return fmt.Errorf("Timeout waiting for node [%s] to be updated with new set of taints: %v", node.Name, err)
	}
	return nil
}
