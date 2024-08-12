package workloads

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/tests/v2/actions/kubeapi/secrets"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/api/scheme"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewImagePullSecret is a contructor that creates an image pull secret for a pod template i.e. corev1.PodTemplateSpec
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
