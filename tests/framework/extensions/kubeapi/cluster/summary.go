package cluster

import (
	"context"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/wrangler/v2/pkg/summary"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// IsClusterActive is a helper function that uses the dynamic client to return cluster's ready state.
func IsClusterActive(client *rancher.Client, clusterID string) (ready bool, err error) {
	dynamic, err := client.GetRancherDynamicClient()
	if err != nil {
		return
	}

	unstructuredCluster, err := dynamic.Resource(schema.GroupVersionResource{Group: "management.cattle.io", Version: "v3", Resource: "clusters"}).Get(context.TODO(), clusterID, metav1.GetOptions{})
	if err != nil {
		return
	}

	summarized := summary.Summarize(unstructuredCluster)

	return summarized.IsReady(), nil
}
