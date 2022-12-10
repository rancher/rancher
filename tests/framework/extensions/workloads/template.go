package workloads

import (
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewContainer is a contructor that creates a container for a pod template i.e. corev1.PodTemplateSpec
func NewContainer(containerName, image string, imagePullPolicy corev1.PullPolicy, volumeMounts []corev1.VolumeMount, envFrom []corev1.EnvFromSource) corev1.Container {
	return corev1.Container{
		Name:            containerName,
		Image:           image,
		ImagePullPolicy: imagePullPolicy,
		VolumeMounts:    volumeMounts,
		EnvFrom:         envFrom,
	}
}

// NewPodTemplate is a constructor that creates the pod template for all types of workloads e.g. cronjobs, daemonsets, deployments, and batch jobs
func NewPodTemplate(containers []corev1.Container, volumes []corev1.Volume, imagePullSecrets []corev1.LocalObjectReference, labels map[string]string) corev1.PodTemplateSpec {
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

func NewDeploymentTemplate(deploymentName string, namespace string, podSpec corev1.PodTemplateSpec, matchLabels map[string]string) *appv1.Deployment {
	return &appv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: namespace,
		},
		Spec: appv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: matchLabels,
			},
			Template: podSpec,
		},
	}

}
