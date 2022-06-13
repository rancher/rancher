package workloads

import (
	corev1 "k8s.io/api/core/v1"
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
func NewPodTemplate(containers []corev1.Container, volumes []corev1.Volume, imagePullSecrets []corev1.LocalObjectReference) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers:       containers,
			Volumes:          volumes,
			ImagePullSecrets: imagePullSecrets,
		},
	}
}
