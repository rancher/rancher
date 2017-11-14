package k8s

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
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
