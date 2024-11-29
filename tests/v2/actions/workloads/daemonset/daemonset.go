package daemonset

import (
	"context"

	"github.com/rancher/rancher/tests/v2/actions/workloads/deployment"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/unstructured"
	"github.com/rancher/shepherd/extensions/workloads"
	appv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var DaemonsetGroupVersionResource = schema.GroupVersionResource{
	Group:    "apps",
	Version:  "v1",
	Resource: "daemonsets",
}

// CreateDaemonset is a helper to create a daemonset
func CreateDaemonset(client *rancher.Client, clusterID, namespaceName string, replicaCount int, secretName, configMapName string, useEnvVars, useVolumes bool) (*appv1.DaemonSet, error) {
	deploymentTemplate, err := deployment.CreateDeployment(client, clusterID, namespaceName, replicaCount, secretName, configMapName, useEnvVars, useVolumes, false, true)
	if err != nil {
		return nil, err
	}

	createdDaemonset := workloads.NewDaemonSetTemplate(deploymentTemplate.Name, namespaceName, deploymentTemplate.Spec.Template, true, nil)

	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return nil, err
	}

	daemonsetResource := dynamicClient.Resource(DaemonsetGroupVersionResource).Namespace(namespaceName)

	_, err = daemonsetResource.Create(context.TODO(), unstructured.MustToUnstructured(createdDaemonset), metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return createdDaemonset, nil
}
