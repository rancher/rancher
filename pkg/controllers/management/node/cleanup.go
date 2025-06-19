package node

import (
	"context"
	"fmt"
	"strings"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/dialer"

	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	kerror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	cleanupPodLabel                    = "rke.cattle.io/cleanup-node"
	userNodeRemoveAnnotationPrefix     = "lifecycle.cattle.io/create.user-node-remove_"
	userNodeRemoveCleanupAnnotationOld = "nodes.management.cattle.io/user-node-remove-cleanup"
)

func (m *Lifecycle) deleteV1Node(node *v3.Node) (runtime.Object, error) {
	logrus.Debugf("[node-cleanup] Deleting v1.node for [%v] node", node.Status.NodeName)

	if node.Status.NodeName == "" {
		logrus.Debugf("[node-cleanup] Skipping v1.node removal for machine [%v] without node name", node.Name)
		return node, nil
	}

	cluster, err := m.clusterLister.Get("", node.Namespace)
	if err != nil {
		if kerror.IsNotFound(err) {
			logrus.Debugf("[node-cleanup] Skipping v1.node removal for machine [%v] without cluster [%v]", node.Name, node.Namespace)
			return node, nil
		}
		return node, err
	}
	userClient, err := m.clusterManager.UserContextFromCluster(cluster)
	if err != nil {
		return node, err
	}
	if userClient == nil {
		logrus.Debugf("[node-cleanup] cluster is already deleted, cannot delete RKE node")
		return node, nil
	}

	ctx, cancel := context.WithTimeout(context.TODO(), 45*time.Second)
	defer cancel()
	err = userClient.K8sClient.CoreV1().Nodes().Delete(
		ctx, node.Status.NodeName, metav1.DeleteOptions{})
	if err != nil && !kerror.IsNotFound(err) &&
		ctx.Err() != context.DeadlineExceeded &&
		!strings.Contains(err.Error(), dialer.ErrAgentDisconnected.Error()) &&
		!strings.Contains(err.Error(), "connection refused") {
		return node, err
	}

	return node, nil
}

func (m *Lifecycle) waitForJobCondition(userContext *config.UserContext, job *batchv1.Job, condition func(*batchv1.Job, error) bool, logMessage string) error {
	if job == nil {
		return nil
	}
	backoff := wait.Backoff{
		Duration: 3 * time.Second,
		Factor:   1,
		Jitter:   0,
		Steps:    10,
	}

	logrus.Infof("[node-cleanup] validating cleanup job %s %sd, retrying up to 10 times", job.Name, logMessage)
	// purposefully ignoring error, if the drain fails this falls back to deleting the node as usual
	return wait.ExponentialBackoff(backoff, func() (bool, error) {
		ctx, cancel := context.WithTimeout(m.ctx, backoff.Duration)
		defer cancel()

		j, err := userContext.K8sClient.BatchV1().Jobs(job.Namespace).Get(ctx, job.Name, metav1.GetOptions{})
		if ctx.Err() != nil {
			logrus.Errorf("[node-cleanup] context failed while retrieving job %s, retrying: %s", job.Name, ctx.Err())
			return false, nil
		}
		if err != nil {
			// kubectl failed continue on with delete any way
			logrus.Errorf("[node-cleanup] failed to get job %s, retrying: %v", job.Name, err)
		}

		if !condition(j, err) {
			logrus.Infof("[node-cleanup] waiting for %s job to %s", job.Name, logMessage)
			return false, nil
		}

		logrus.Infof("[node-cleanup] finished waiting for job %s to %s", job.Name, logMessage)
		return true, nil
	})
}

func (m *Lifecycle) waitUntilJobCompletes(userContext *config.UserContext, job *batchv1.Job) error {
	return m.waitForJobCondition(
		userContext,
		job,
		func(j *batchv1.Job, err error) bool { return err == nil && j.Status.Succeeded > 0 },
		"complete",
	)
}

func (m *Lifecycle) waitUntilJobDeletes(userContext *config.UserContext, nodeName string, job *batchv1.Job) error {
	return m.waitForJobCondition(userContext, job, func(j *batchv1.Job, err error) bool {
		if err == nil {
			if j.DeletionTimestamp.IsZero() {
				err = userContext.BatchV1.Jobs(j.Namespace).Delete(j.Name, &metav1.DeleteOptions{PropagationPolicy: &[]metav1.DeletionPropagation{metav1.DeletePropagationForeground}[0]})
			} else if pods, err := userContext.Core.Pods(j.Namespace).List(metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", cleanupPodLabel, nodeName)}); err != nil && !kerror.IsNotFound(err) {
				logrus.Errorf("[node-cleanup] failed to list cleanup pods for node %s: %v", nodeName, err)
				return false
			} else if err == nil && len(pods.Items) > 0 {
				if err = userContext.Core.Pods(j.Namespace).Delete(pods.Items[0].Name, &metav1.DeleteOptions{GracePeriodSeconds: &[]int64{0}[0]}); err != nil {
					logrus.Errorf("[node-cleanup] failed to delete cleanup pod %s for node %s: %v", pods.Items[0].Name, nodeName, err)
					return false
				}
			}
		}
		return kerror.IsNotFound(err)
	},
		"delete")
}

func (m *Lifecycle) userNodeRemoveCleanup(obj *v3.Node) *v3.Node {
	obj = obj.DeepCopy()
	obj.SetFinalizers(removeFinalizerWithPrefix(obj.GetFinalizers(), userNodeRemoveFinalizerPrefix))

	if obj.DeletionTimestamp == nil {
		annos := obj.GetAnnotations()
		if annos == nil {
			annos = make(map[string]string)
		} else {
			annos = removeAnnotationWithPrefix(annos, userNodeRemoveAnnotationPrefix)
			delete(annos, userNodeRemoveCleanupAnnotationOld)
		}

		annos[userNodeRemoveCleanupAnnotation] = "true"
		obj.SetAnnotations(annos)
	}
	return obj
}

func removeFinalizerWithPrefix(finalizers []string, prefix string) []string {
	var nf []string
	for _, finalizer := range finalizers {
		if strings.HasPrefix(finalizer, prefix) {
			logrus.Debugf("[node-cleanup] finalizer with prefix [%s] will be removed", prefix)
			continue
		}
		nf = append(nf, finalizer)
	}
	return nf
}

func removeAnnotationWithPrefix(annotations map[string]string, prefix string) map[string]string {
	for k := range annotations {
		if strings.HasPrefix(k, prefix) {
			logrus.Debugf("[node-cleanup] annotation with prefix [%s] will be removed", prefix)
			delete(annotations, k)
		}
	}
	return annotations
}
