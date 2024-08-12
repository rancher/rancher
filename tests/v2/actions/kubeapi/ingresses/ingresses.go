package ingresses

import (
	"github.com/rancher/shepherd/clients/rancher"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// IngressesGroupVersionResource is the required Group Version Resource for accessing ingresses in a cluster,
// using the dynamic client.
var IngressesGroupVersionResource = schema.GroupVersionResource{
	Group:    "networking.k8s.io",
	Version:  "v1",
	Resource: "ingresses",
}

// GetIngressByName is a helper function that returns the ingress by name in a specific cluster, uses ListIngresses to get the ingress.
func GetIngressByName(client *rancher.Client, clusterID, namespaceName, ingressName string) (*networkingv1.Ingress, error) {
	var ingress *networkingv1.Ingress

	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	if err != nil {
		return ingress, err
	}

	ingressesList, err := ListIngresses(adminClient, clusterID, namespaceName, metav1.ListOptions{})
	if err != nil {
		return ingress, err
	}

	for i, ingress := range ingressesList.Items {
		if ingress.Name == ingressName {
			return &ingressesList.Items[i], nil
		}
	}

	return ingress, nil
}
