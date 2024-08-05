package pods

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/extensions/kubeconfig"
	"github.com/rancher/shepherd/extensions/workloads"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	corev1 "k8s.io/api/core/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
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
	return workloads.NewPodTemplate(containers, volumes, nil, nil)
}

// CheckPodLogsForErrors is a helper to check pod logs for errors
func CheckPodLogsForErrors(client *rancher.Client, clusterID string, podName string, namespace string, errorPattern string, startTime time.Time) error {
	startTimeUTC := startTime.UTC()

	errorRegex := regexp.MustCompile(errorPattern)
	timeRegex := regexp.MustCompile(`^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}`)

	var errorMessage string

	kwait.Poll(defaults.TenSecondTimeout, defaults.TwoMinuteTimeout, func() (bool, error) {
		podLogs, err := kubeconfig.GetPodLogs(client, clusterID, podName, namespace, "")
		if err != nil {
			return false, err
		}

		segments := strings.Split(podLogs, "\n")
		for _, segment := range segments {
			timeMatches := timeRegex.FindStringSubmatch(segment)
			if len(timeMatches) > 0 {
				segmentTime, err := time.Parse(timeFormat, timeMatches[0])
				if err != nil {
					continue
				}

				segmentTimeUTC := segmentTime.UTC()
				if segmentTimeUTC.After(startTimeUTC) {
					if matches := errorRegex.FindStringSubmatch(segment); len(matches) > 0 {
						errorMessage = "error logs found in rancher: " + segment
						return true, nil
					}
				}
			}
		}
		return false, nil
	})

	if errorMessage != "" {
		return errors.New(errorMessage)
	}

	return nil
}
