package k8s

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
)

const (
	HostnameLabel             = "kubernetes.io/hostname"
	InternalAddressAnnotation = "rke.cattle.io/internal-ip"
	ExternalAddressAnnotation = "rke.cattle.io/external-ip"
	AWSCloudProvider          = "aws"
)

func DeleteNode(k8sClient *kubernetes.Clientset, nodeName, cloudProvider string) error {

	if cloudProvider == AWSCloudProvider {
		node, err := GetNode(k8sClient, nodeName)
		if err != nil {
			return err
		}
		nodeName = node.Name
	}
	return k8sClient.CoreV1().Nodes().Delete(nodeName, &metav1.DeleteOptions{})
}

func GetNodeList(k8sClient *kubernetes.Clientset) (*v1.NodeList, error) {
	return k8sClient.CoreV1().Nodes().List(metav1.ListOptions{})
}

func GetNode(k8sClient *kubernetes.Clientset, nodeName string) (*v1.Node, error) {
	nodes, err := GetNodeList(k8sClient)
	if err != nil {
		return nil, err
	}
	for _, node := range nodes.Items {
		if strings.ToLower(node.Labels[HostnameLabel]) == strings.ToLower(nodeName) {
			return &node, nil
		}
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{}, nodeName)
}

func CordonUncordon(k8sClient *kubernetes.Clientset, nodeName string, cordoned bool) error {
	updated := false
	for retries := 0; retries <= 5; retries++ {
		node, err := GetNode(k8sClient, nodeName)
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

func SyncLabels(k8sClient *kubernetes.Clientset, nodeName string, toAddLabels, toDelLabels map[string]string) error {
	updated := false
	var err error
	for retries := 0; retries <= 5; retries++ {
		if err = doSyncLabels(k8sClient, nodeName, toAddLabels, toDelLabels); err != nil {
			time.Sleep(5 * time.Second)
			continue
		}
		updated = true
		break
	}
	if !updated {
		return fmt.Errorf("Timeout waiting for labels to be synced for node [%s]: %v", nodeName, err)
	}
	return nil
}

func SyncTaints(k8sClient *kubernetes.Clientset, nodeName string, toAddTaints, toDelTaints []string) error {
	updated := false
	var err error
	for retries := 0; retries <= 5; retries++ {
		if err = doSyncTaints(k8sClient, nodeName, toAddTaints, toDelTaints); err != nil {
			time.Sleep(5 * time.Second)
			continue
		}
		updated = true
		break
	}
	if !updated {
		return fmt.Errorf("Timeout waiting for node [%s] to be updated with new set of taints: %v", nodeName, err)
	}
	return nil
}

func doSyncLabels(k8sClient *kubernetes.Clientset, nodeName string, toAddLabels, toDelLabels map[string]string) error {
	node, err := GetNode(k8sClient, nodeName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Debugf("[hosts] Can't find node by name [%s]", nodeName)
			return nil
		}
		return err
	}
	oldLabels := map[string]string{}
	if node.Labels == nil {
		node.Labels = map[string]string{}
	}

	for k, v := range node.Labels {
		oldLabels[k] = v
	}

	// Delete Labels
	for key := range toDelLabels {
		if _, ok := node.Labels[key]; ok {
			delete(node.Labels, key)
		}
	}
	// ADD Labels
	for key, value := range toAddLabels {
		node.Labels[key] = value
	}
	if reflect.DeepEqual(oldLabels, node.Labels) {
		logrus.Debugf("Labels are not changed for node [%s]", node.Name)
		return nil
	}
	_, err = k8sClient.CoreV1().Nodes().Update(node)
	if err != nil {
		logrus.Debugf("Error syncing labels for node [%s]: %v", node.Name, err)
		return err
	}
	return nil
}

func doSyncTaints(k8sClient *kubernetes.Clientset, nodeName string, toAddTaints, toDelTaints []string) error {
	node, err := GetNode(k8sClient, nodeName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Debugf("[hosts] Can't find node by name [%s]", nodeName)
			return nil
		}
		return err
	}
	// Add taints to node
	for _, taintStr := range toAddTaints {
		if isTaintExist(toTaint(taintStr), node.Spec.Taints) {
			continue
		}
		node.Spec.Taints = append(node.Spec.Taints, toTaint(taintStr))
	}
	// Remove Taints from node
	for i, taintStr := range toDelTaints {
		if isTaintExist(toTaint(taintStr), node.Spec.Taints) {
			if len(node.Spec.Taints) == 1 {
				node.Spec.Taints = []v1.Taint{}
			} else {
				node.Spec.Taints = append(node.Spec.Taints[:i], node.Spec.Taints[i+1:]...)
			}
		}
	}

	//node.Spec.Taints
	_, err = k8sClient.CoreV1().Nodes().Update(node)
	if err != nil {
		logrus.Debugf("Error updating node [%s] with new set of taints: %v", node.Name, err)
		return err
	}
	return nil
}

func isTaintExist(taint v1.Taint, taintList []v1.Taint) bool {
	for _, t := range taintList {
		if t.Key == taint.Key && t.Value == taint.Value && t.Effect == taint.Effect {
			return true
		}
	}
	return false
}

func toTaint(taintStr string) v1.Taint {
	taintStruct := strings.Split(taintStr, "=")
	tmp := strings.Split(taintStruct[1], ":")
	key := taintStruct[0]
	value := tmp[0]
	effect := v1.TaintEffect(tmp[1])
	return v1.Taint{
		Key:    key,
		Value:  value,
		Effect: effect,
	}
}

func SetAddressesAnnotations(k8sClient *kubernetes.Clientset, nodeName, internalAddress, externalAddress string) error {
	var listErr error
	for retries := 0; retries <= 5; retries++ {
		node, err := GetNode(k8sClient, nodeName)
		if err != nil {
			listErr = errors.Wrapf(err, "Failed to get kubernetes node [%s]", nodeName)
			time.Sleep(time.Second * 5)
			continue
		}
		currentExternalAnnotation := node.Annotations[ExternalAddressAnnotation]
		currentInternalAnnotation := node.Annotations[ExternalAddressAnnotation]
		if currentExternalAnnotation == externalAddress && currentInternalAnnotation == internalAddress {
			return nil
		}
		node.Annotations[ExternalAddressAnnotation] = externalAddress
		node.Annotations[InternalAddressAnnotation] = internalAddress
		_, err = k8sClient.CoreV1().Nodes().Update(node)
		if err != nil {
			listErr = errors.Wrapf(err, "Error updating node [%s] with address annotations: %v", nodeName, err)
			time.Sleep(time.Second * 5)
			continue
		}
		return nil
	}
	return listErr
}
