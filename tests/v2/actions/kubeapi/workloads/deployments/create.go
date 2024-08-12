package deployments

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

// CreateDeployment is a helper function that uses the dynamic client to create a deployment on a namespace for a specific cluster.
func CreateDeployment(client *rancher.Client, clusterName, deploymentName, namespace string, template corev1.PodTemplateSpec, replicas int32) (*appv1.Deployment, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterName)
	if err != nil {
		return nil, err
	}

	labels := map[string]string{}
	labels["workload.user.cattle.io/workloadselector"] = fmt.Sprintf("apps.deployment-%v-%v", namespace, deploymentName)

	template.ObjectMeta = metav1.ObjectMeta{
		Labels: labels,
	}

	template.Spec.RestartPolicy = corev1.RestartPolicyAlways
	deployment := &appv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: namespace,
		},
		Spec: appv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: template,
		},
	}

	deploymentResource := dynamicClient.Resource(DeploymentGroupVersionResource).Namespace(namespace)

	unstructuredResp, err := deploymentResource.Create(context.TODO(), unstructured.MustToUnstructured(deployment), metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	newDeployment := &appv1.Deployment{}
	err = scheme.Scheme.Convert(unstructuredResp, newDeployment, unstructuredResp.GroupVersionKind())
	if err != nil {
		return nil, err
	}

	return newDeployment, nil
}
