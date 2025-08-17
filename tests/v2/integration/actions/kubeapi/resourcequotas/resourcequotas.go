package resourcequotas

import (
	"fmt"

	"github.com/rancher/shepherd/clients/rancher"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ResourceQuotaGroupVersionResource is the required Group Version Resource for accessing resource quotas in a cluster,
// using the dynamic client.
var ResourceQuotaGroupVersionResource = schema.GroupVersionResource{
	Group:    "",
	Version:  "v1",
	Resource: "resourcequotas",
}

// GetResourceQuotaByName is a helper function that returns the resource quota by name in a specific cluster.
func GetResourceQuotaByName(client *rancher.Client, clusterID, name string) (*corev1.ResourceQuota, error) {
	resourceQuotaList, err := ListResourceQuotas(client, clusterID, "", metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for i, q := range resourceQuotaList.Items {
		if name == q.Name {
			quota := &resourceQuotaList.Items[i]
			return quota, nil
		}
	}

	return nil, fmt.Errorf("quota %s not found in %s cluster", name, clusterID)
}
