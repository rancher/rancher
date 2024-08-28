package pods

import (
	"github.com/rancher/shepherd/extensions/workloads"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	corev1 "k8s.io/api/core/v1"
)

const (
	timeFormat = "2006/01/02 15:04:05"
	imageName  = "nginx"
)

// NewPodTemplateWithConfig is a helper to create a Pod template with a secret/configmap as an environment variable or volume mount or both
func NewPodTemplateWithConfig(secretName, configMapName string, useEnvVars, useVolumes bool) corev1.PodTemplateSpec {
	containerName := namegen.AppendRandomString("testcontainer")
	pullPolicy := corev1.PullAlways

	var envFrom []corev1.EnvFromSource
	if useEnvVars {
		if secretName != "" {
			envFrom = append(envFrom, corev1.EnvFromSource{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				},
			})
		}
		if configMapName != "" {
			envFrom = append(envFrom, corev1.EnvFromSource{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
				},
			})
		}
	}

	var volumes []corev1.Volume
	if useVolumes {
		volumeName := namegen.AppendRandomString("vol")
		optional := false
		if secretName != "" {
			volumes = append(volumes, corev1.Volume{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: secretName,
						Optional:   &optional,
					},
				},
			})
		}
		if configMapName != "" {
			volumes = append(volumes, corev1.Volume{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
						Optional:             &optional,
					},
				},
			})
		}
	}

	container := workloads.NewContainer(containerName, imageName, pullPolicy, nil, envFrom, nil, nil, nil)
	containers := []corev1.Container{container}
	return workloads.NewPodTemplate(containers, volumes, nil, nil, nil)
}
