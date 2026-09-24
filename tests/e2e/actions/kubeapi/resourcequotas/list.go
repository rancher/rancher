package resourcequotas

import (
	"context"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/api/scheme"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceQuotaList is a struct that contains a list of resource quotas.
type ResourceQuotaList struct {
	Items []corev1.ResourceQuota
}

// ListResourceQuotas is a helper function that uses the dynamic client to list resource quotas in a cluster with its list options.
func ListResourceQuotas(client *rancher.Client, clusterID string, namespace string, listOpts metav1.ListOptions) (*ResourceQuotaList, error) {
	resourceQuotaList := new(ResourceQuotaList)

	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return nil, err
	}

	resourceQuotaResource := dynamicClient.Resource(ResourceQuotaGroupVersionResource).Namespace(namespace)
	quotas, err := resourceQuotaResource.List(context.TODO(), listOpts)
	if err != nil {
		return nil, err
	}

	for _, unstructuredQuota := range quotas.Items {
		newQuota := &corev1.ResourceQuota{}
		err := scheme.Scheme.Convert(&unstructuredQuota, newQuota, unstructuredQuota.GroupVersionKind())
		if err != nil {
			return nil, err
		}

		resourceQuotaList.Items = append(resourceQuotaList.Items, *newQuota)
	}

	return resourceQuotaList, nil
}
