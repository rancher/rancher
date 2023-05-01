package etcdmgmt

import (
	"context"

	"github.com/sirupsen/logrus"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
)

func SafelyRemoved(restConfig *rest.Config, runtime, nodeName string) (bool, error) {
	removeAnnotation := "etcd." + runtime + ".cattle.io/remove"
	removedNodeNameAnnotation := "etcd." + runtime + ".cattle.io/removed-node-name"

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return false, err
	}

	logrus.Debugf("Retrieving node %s from K8s", nodeName)

	node, err := clientset.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		if apierror.IsNotFound(err) {
			logrus.Debugf("Node %s was not found. proceeding with deletion", nodeName)
			return true, nil
		}
		return false, err
	}

	if node.Annotations[removeAnnotation] == "true" {
		// check val to see if it's true, if not, continue
		// check the status of the removal
		logrus.Debugf("etcd member removal is currently in progress per the annotation %s", removeAnnotation)
		return node.Annotations[removedNodeNameAnnotation] != "", nil
	}
	// The remove annotation has not been set to true, so we'll go ahead and set it on the node.
	return false, retry.RetryOnConflict(retry.DefaultRetry,
		func() error {
			node, err = clientset.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			node.Annotations[removeAnnotation] = "true"
			_, err = clientset.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
			return err
		})
}
