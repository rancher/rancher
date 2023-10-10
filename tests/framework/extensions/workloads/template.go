package workloads

import (
	"fmt"

	appv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewContainer is a contructor that creates a container for a pod template i.e. corev1.PodTemplateSpec
func NewContainer(containerName, image string, imagePullPolicy corev1.PullPolicy, volumeMounts []corev1.VolumeMount, envFrom []corev1.EnvFromSource, command []string, securityContext *corev1.SecurityContext, args []string) corev1.Container {
	return corev1.Container{
		Name:            containerName,
		Image:           image,
		ImagePullPolicy: imagePullPolicy,
		VolumeMounts:    volumeMounts,
		EnvFrom:         envFrom,
		Command:         command,
		Args:            args,
		SecurityContext: securityContext,
	}
}

// NewPodTemplate is a constructor that creates the pod template for all types of workloads e.g. cronjobs, daemonsets, deployments, and batch jobs
func NewPodTemplate(containers []corev1.Container, volumes []corev1.Volume, imagePullSecrets []corev1.LocalObjectReference, labels map[string]string) corev1.PodTemplateSpec {
	if labels == nil {
		labels = make(map[string]string)
	}

	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: labels,
		},
		Spec: corev1.PodSpec{
			Containers:       containers,
			Volumes:          volumes,
			ImagePullSecrets: imagePullSecrets,
		},
	}
}

// NewDeploymentTemplate is a constructor that creates a deployment template. If the isCattleLabeled true, workloadselector labels are assigned to the deployment and the pod template.
func NewDeploymentTemplate(deploymentName string, namespace string, template corev1.PodTemplateSpec, isCattleLabeled bool, matchLabels map[string]string) *appv1.Deployment {
	if matchLabels == nil {
		matchLabels = make(map[string]string)
	}

	if isCattleLabeled {
		matchLabels["workload.user.cattle.io/workloadselector"] = fmt.Sprintf("apps.deployment-%v-%v", namespace, deploymentName)
		template.ObjectMeta.Labels["workload.user.cattle.io/workloadselector"] = fmt.Sprintf("apps.deployment-%v-%v", namespace, deploymentName)
	}

	return &appv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: namespace,
		},
		Spec: appv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: matchLabels,
			},
			Template: template,
		},
	}

}

// NewDeploymentTemplate is a constructor that creates a daemonset template. If the isCattleLabeled true, workloadselector labels are assigned to the daemonset and the pod template.
func NewDaemonSetTemplate(daemonsetName string, namespace string, template corev1.PodTemplateSpec, isCattleLabeled bool, matchLabels map[string]string) *appv1.DaemonSet {
	if matchLabels == nil {
		matchLabels = map[string]string{}
	}

	if isCattleLabeled {
		matchLabels["workload.user.cattle.io/workloadselector"] = fmt.Sprintf("apps.daemonset-%v-%v", namespace, daemonsetName)
		template.ObjectMeta.Labels["workload.user.cattle.io/workloadselector"] = fmt.Sprintf("apps.daemonset-%v-%v", namespace, daemonsetName)
	}

	return &appv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      daemonsetName,
			Namespace: namespace,
		},
		Spec: appv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: matchLabels,
			},
			Template: template,
		},
	}
}

// NewJobTemplate is a constructor that creates a job template.
func NewJobTemplate(jobName string, namespace string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:      "Never",
					ServiceAccountName: "",
					Containers:         []corev1.Container{},
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}

}
