package daemonsets

import (
	"context"
	"fmt"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/unstructured"
	"github.com/rancher/shepherd/pkg/api/scheme"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateDaemonSet is a helper function that uses the dynamic client to create a daemon set on a namespace for a specific cluster.
func CreateDaemonSet(client *rancher.Client, clusterName, daemonSetName, namespace string, template corev1.PodTemplateSpec) (*appv1.DaemonSet, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterName)
	if err != nil {
		return nil, err
	}

	labels := map[string]string{}
	labels["workload.user.cattle.io/workloadselector"] = fmt.Sprintf("apps.daemonset-%v-%v", namespace, daemonSetName)

	template.ObjectMeta = metav1.ObjectMeta{
		Labels: labels,
	}
	template.Spec.RestartPolicy = corev1.RestartPolicyAlways
	daemonSet := &appv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      daemonSetName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: template,
		},
	}

	daemonSetResource := dynamicClient.Resource(DaemonSetGroupVersionResource).Namespace(namespace)

	unstructuredResp, err := daemonSetResource.Create(context.TODO(), unstructured.MustToUnstructured(daemonSet), metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	newDaemonSet := &appv1.DaemonSet{}
	err = scheme.Scheme.Convert(unstructuredResp, newDaemonSet, unstructuredResp.GroupVersionKind())
	if err != nil {
		return nil, err
	}

	return daemonSet, nil
}
