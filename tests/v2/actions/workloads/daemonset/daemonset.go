package daemonset

import (
	"context"

	"github.com/rancher/rancher/tests/v2/actions/workloads/deployment"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/unstructured"
	"github.com/rancher/shepherd/extensions/workloads"
	"github.com/rancher/shepherd/pkg/wrangler"
	appv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var DaemonsetGroupVersionResource = schema.GroupVersionResource{
	Group:    "apps",
	Version:  "v1",
	Resource: "daemonsets",
}

const (
	DaemonsetSteveType = "apps.daemonset"
)

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

// UpdateDaemonset is a helper to update daemonsets
func UpdateDaemonset(client *rancher.Client, clusterID, namespaceName string, daemonset *appv1.DaemonSet) (*appv1.DaemonSet, error) {
	var wranglerContext *wrangler.Context
	var err error

	wranglerContext = client.WranglerContext
	if clusterID != "local" {
		wranglerContext, err = client.WranglerContext.DownStreamClusterWranglerContext(clusterID)
		if err != nil {
			return nil, err
		}
	}

	latestDaemonset, err := wranglerContext.Apps.DaemonSet().Get(namespaceName, daemonset.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	daemonset.ResourceVersion = latestDaemonset.ResourceVersion

	updatedDaemonset, err := wranglerContext.Apps.DaemonSet().Update(daemonset)
	if err != nil {
		return nil, err
	}

	return updatedDaemonset, err
}

// DeleteDaemonset is a helper to delete a daemonset
func DeleteDaemonset(client *rancher.Client, clusterID string, daemonset *appv1.DaemonSet) error {
	steveClient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return err
	}

	daemonsetID := daemonset.Namespace + "/" + daemonset.Name
	daemonsetResp, err := steveClient.SteveType(DaemonsetSteveType).ByID(daemonsetID)
	if err != nil {
		return err
	}

	err = steveClient.SteveType(DaemonsetSteveType).Delete(daemonsetResp)
	if err != nil {
		return err
	}

	return nil
}
