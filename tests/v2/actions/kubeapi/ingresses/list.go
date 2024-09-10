package ingresses

import (
	"context"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/api/scheme"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IngressList is a struct that contains a list of deployments.
type IngressList struct {
	Items []networkingv1.Ingress
}

// ListIngresses is a helper function that uses the dynamic client to list ingresses on a namespace for a specific cluster with its list options.
func ListIngresses(client *rancher.Client, clusterID, namespace string, listOpts metav1.ListOptions) (*IngressList, error) {
	ingressList := new(IngressList)

	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return nil, err
	}

	ingressResource := dynamicClient.Resource(IngressesGroupVersionResource).Namespace(namespace)
	ingresses, err := ingressResource.List(context.TODO(), listOpts)
	if err != nil {
		return nil, err
	}

	for _, unstructuredIngress := range ingresses.Items {
		newIngress := &networkingv1.Ingress{}
		err := scheme.Scheme.Convert(&unstructuredIngress, newIngress, unstructuredIngress.GroupVersionKind())
		if err != nil {
			return nil, err
		}

		ingressList.Items = append(ingressList.Items, *newIngress)
	}

	return ingressList, nil
}

// Names is a method that accepts IngressList as a receiver,
// returns each ingress name in the list as a new slice of strings.
func (list *IngressList) Names() []string {
	var ingressNames []string

	for _, ingress := range list.Items {
		ingressNames = append(ingressNames, ingress.Name)
	}

	return ingressNames
}
