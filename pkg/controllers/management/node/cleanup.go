package node

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rancher/rancher/pkg/agent/clean"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	util "github.com/rancher/rancher/pkg/cluster"
	"github.com/rancher/rancher/pkg/dialer"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/kubectl"
	"github.com/rancher/rancher/pkg/namespace"
	nodehelper "github.com/rancher/rancher/pkg/node"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemtemplate"
	"github.com/rancher/rancher/pkg/types/config"
	rketypes "github.com/rancher/rke/types"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
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
	if nodehelper.IgnoreNode(node.Status.NodeName, node.Status.NodeLabels) {
		logrus.Debugf("[node-cleanup] Skipping v1.node removal for [%v] node", node.Status.NodeName)
		return node, nil
	}

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

func (m *Lifecycle) drainNode(node *v3.Node) error {
	nodeCopy := node.DeepCopy() // copy for cache protection as we do no updating but need things set for the drain
	cluster, err := m.clusterLister.Get("", nodeCopy.Namespace)
	if err != nil {
		if kerror.IsNotFound(err) {
			return nil
		}
		return err
	}

	nodePool, err := m.getNodePool(node.Spec.NodePoolName)
	if err != nil && !kerror.IsNotFound(err) {
		return err
	}

	if !nodehelper.DrainBeforeDelete(nodeCopy, cluster, nodePool) {
		return nil
	}

	logrus.Infof("[node-cleanup] node [%s] requires draining before delete", nodeCopy.Spec.RequestedHostname)
	kubeConfig, _, err := m.getKubeConfig(cluster)
	if err != nil {
		return fmt.Errorf("node [%s] error getting kubeConfig", nodeCopy.Spec.RequestedHostname)
	}

	if nodeCopy.Spec.NodeDrainInput == nil {
		logrus.Debugf("[node-cleanup] node [%s] has no NodeDrainInput, creating one with 60s timeout",
			nodeCopy.Spec.RequestedHostname)
		nodeCopy.Spec.NodeDrainInput = &rketypes.NodeDrainInput{
			Force:           true,
			DeleteLocalData: true,
			GracePeriod:     60,
			Timeout:         60,
		}
	}

	backoff := wait.Backoff{
		Duration: 2 * time.Second,
		Factor:   1,
		Jitter:   0,
		Steps:    3,
	}

	logrus.Infof("[node-cleanup] node [%s] attempting to drain, retrying up to 3 times", nodeCopy.Spec.RequestedHostname)
	// purposefully ignoring kubectl.drain error. However, if the node fails to drain after 3 attempts
	// wait.ExponentialBackoff will still return a wait.ErrWaitTimeout error, which must also be ignored.
	// Otherwise, the node will not actually delete and resources will be orphaned in the provider.
	_ = wait.ExponentialBackoff(backoff, func() (bool, error) {
		ctx, cancel := context.WithTimeout(m.ctx, time.Duration(nodeCopy.Spec.NodeDrainInput.Timeout)*time.Second)
		defer cancel()

		_, msg, err := kubectl.Drain(ctx, kubeConfig, nodeCopy.Status.NodeName, nodehelper.GetDrainFlags(nodeCopy))
		if ctx.Err() != nil {
			logrus.Errorf("[node-cleanup] node [%s] kubectl drain failed, retrying: %s", nodeCopy.Spec.RequestedHostname, ctx.Err())
			return false, nil
		}
		if err != nil {
			logrus.Errorf("[node-cleanup] node [%s] kubectl drain error, retrying: %s", nodeCopy.Spec.RequestedHostname, err)
			return false, nil
		}

		logrus.Infof("[node-cleanup] node [%s] kubectl drain response: %s", nodeCopy.Spec.RequestedHostname, msg)
		return true, nil
	})
	return nil // always return nil so the node is deleted regardless of drain outcome
}

func (m *Lifecycle) cleanRKENode(node *v3.Node) error {
	if !features.RKE1CustomNodeCleanup.Enabled() {
		return nil
	}

	cluster, err := m.clusterLister.Get("", node.Namespace)
	if err != nil {
		if kerror.IsNotFound(err) {
			return nil // no cluster, we'll never figure out if this is an RKE1 cluster
		}
		return err
	}

	if cluster.Status.Driver != v3.ClusterDriverRKE {
		return nil // not an rke node, bail out
	}

	userContext, err := m.clusterManager.UserContextFromCluster(cluster)
	if err != nil {
		return err
	}
	if userContext == nil {
		logrus.Debugf("[node-cleanup] cluster is already deleted, cannot clean RKE node")
		return nil
	}

	job, err := m.createCleanupJob(userContext, cluster, node)
	if err != nil {
		return err
	}

	if err = m.waitUntilJobCompletes(userContext, job); err != nil && !errors.Is(err, wait.ErrWaitTimeout) {
		return err
	}

	return m.waitUntilJobDeletes(userContext, node.Name, job)
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

func (m *Lifecycle) createCleanupJob(userContext *config.UserContext, cluster *v3.Cluster, node *v3.Node) (*batchv1.Job, error) {
	nodeLabel := "cattle.io/node"

	// find if someone else already kicked this job off
	existingJob, err := userContext.K8sClient.BatchV1().Jobs("default").List(m.ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", nodeLabel, node.Name),
	})
	if err != nil && !kerror.IsNotFound(err) {
		if strings.Contains(err.Error(), dialer.ErrAgentDisconnected.Error()) ||
			strings.Contains(err.Error(), "connection refused") {
			return nil, nil // can't connect, just continue on with deleting v3 node
		}
		return nil, err
	}

	if len(existingJob.Items) != 0 {
		if existingJob.Items[0].DeletionTimestamp.IsZero() {
			// found an existing job that isn't deleting, so assuming another run of the controller is working on it.
			// Return an "already exists" error.
			return nil, &kerror.StatusError{
				ErrStatus: metav1.Status{
					Reason:  metav1.StatusReasonAlreadyExists,
					Message: fmt.Sprintf("job already exists for %s/%s", node.Namespace, node.Name),
				},
			}
		}

		return nil, m.waitUntilJobDeletes(userContext, node.Name, &existingJob.Items[0])
	}

	meta := metav1.ObjectMeta{
		GenerateName: "cattle-node-cleanup-",
		Namespace:    "default",
		Labels: map[string]string{
			"cattle.io/creator": "norman",
			nodeLabel:           node.Name,
		},
	}

	var tolerations []corev1.Toleration

	for _, taint := range node.Spec.InternalNodeSpec.Taints {
		tolerations = append(tolerations, corev1.Toleration{
			Effect:   taint.Effect,
			Key:      taint.Key,
			Operator: "Exists",
		})
	}

	var mounts []corev1.VolumeMount
	var volumes []corev1.Volume

	if os, ok := node.Status.NodeLabels["kubernetes.io/os"]; ok && os == "windows" {
		t := corev1.HostPathType("")
		volumes = append(volumes, corev1.Volume{
			Name: "docker",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "\\\\.\\pipe\\docker_engine",
					Type: &t,
				},
			},
		})
		mounts = append(mounts, corev1.VolumeMount{
			MountPath: "\\\\.\\pipe\\docker_engine",
			Name:      "docker",
		})
	} else {
		socket := corev1.HostPathType("Socket")
		volumes = append(volumes, corev1.Volume{
			Name: "docker",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/run/docker.sock",
					Type: &socket,
				},
			},
		})
		mounts = append(mounts, corev1.VolumeMount{
			MountPath: "/var/run/docker.sock",
			Name:      "docker",
		})
	}

	env := []corev1.EnvVar{
		{
			Name:  "AGENT_IMAGE",
			Value: settings.AgentImage.Get(),
		},
	}

	if cluster.Spec.RancherKubernetesEngineConfig != nil {
		env = append(env,
			corev1.EnvVar{
				Name:  "PREFIX_PATH",
				Value: cluster.Spec.RancherKubernetesEngineConfig.PrefixPath,
			},
			corev1.EnvVar{
				Name:  "WINDOWS_PREFIX_PATH",
				Value: cluster.Spec.RancherKubernetesEngineConfig.WindowsPrefixPath,
			},
		)
	}

	var imagePullSecrets []corev1.LocalObjectReference
	if registrySecret := cluster.GetSecret(v3.ClusterPrivateRegistrySecret); registrySecret != "" {
		privateRegistries, err := m.secretLister.Get(namespace.GlobalNamespace, registrySecret)
		if err != nil {
			return nil, err
		} else if url, err := util.GeneratePrivateRegistryDockerConfig(util.GetPrivateRepo(cluster), privateRegistries.Data[".dockerconfigjson"]); err != nil {
			return nil, err
		} else if url != "" {
			imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: "cattle-private-registry"})
		}
	}

	fiveMin := int32(5 * 60)
	job := batchv1.Job{
		ObjectMeta: meta,
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &fiveMin,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						cleanupPodLabel: node.Name,
					},
				},
				Spec: corev1.PodSpec{
					ImagePullSecrets: imagePullSecrets,
					RestartPolicy:    "Never",
					NodeSelector: map[string]string{
						"kubernetes.io/hostname": node.Status.NodeName,
					},
					Tolerations: tolerations,
					Volumes:     volumes,
					Containers: []corev1.Container{
						{
							Name:            clean.NodeCleanupContainerName,
							Image:           systemtemplate.GetDesiredAgentImage(cluster),
							Args:            []string{"--", "agent", "clean", "job"},
							Env:             env,
							VolumeMounts:    mounts,
							ImagePullPolicy: corev1.PullIfNotPresent,
						},
					},
				},
			},
		},
	}

	return userContext.K8sClient.BatchV1().Jobs("default").Create(context.TODO(), &job, metav1.CreateOptions{})
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
