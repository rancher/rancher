package workloads

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/pkg/api/scheme"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/secrets"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewContainer(containerName, image string, imagePullPolicy corev1.PullPolicy, volumeMounts []corev1.VolumeMount) corev1.Container {
	return corev1.Container{
		Name:            containerName,
		Image:           image,
		ImagePullPolicy: imagePullPolicy,
		VolumeMounts:    volumeMounts,
	}
}

func NewImagePullSecret(client *rancher.Client, clusterName, namespace string) (*corev1.LocalObjectReference, error) {
	k8sClient, err := client.GetDownStreamClusterClient(clusterName)
	if err != nil {
		return nil, err
	}

	resp, err := k8sClient.Resource(secrets.SecretGroupVersionResource).Namespace(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	secrets := resp.Items

	if len(secrets) < 1 {
		return nil, fmt.Errorf("chosen namespace has no secrets")
	}

	secret := resp.Items[0]

	newSecret := &corev1.Secret{}
	err = scheme.Scheme.Convert(&secret, newSecret, secret.GroupVersionKind())
	if err != nil {
		return nil, err
	}

	return &corev1.LocalObjectReference{
		Name: newSecret.Name,
	}, nil
}

func NewTemplate(containers []corev1.Container, imagePullSecret *corev1.LocalObjectReference) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: containers,
			ImagePullSecrets: []corev1.LocalObjectReference{
				*imagePullSecret,
			},
		},
	}
}
