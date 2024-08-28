package ingresses

import (
	"context"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/unstructured"
	"github.com/rancher/shepherd/pkg/api/scheme"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateIngress is a helper function that uses the dynamic client to create an ingress on a namespace for a specific cluster.
func CreateIngress(client *rancher.Client, clusterID, ingressName, namespace string, ingressSpec *networkingv1.IngressSpec) (*networkingv1.Ingress, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return nil, err
	}

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ingressName,
			Namespace: namespace,
		},
		Spec: *ingressSpec,
	}

	ingressResource := dynamicClient.Resource(IngressesGroupVersionResource).Namespace(namespace)

	unstructuredResp, err := ingressResource.Create(context.TODO(), unstructured.MustToUnstructured(ingress), metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	newIngress := &networkingv1.Ingress{}
	err = scheme.Scheme.Convert(unstructuredResp, newIngress, unstructuredResp.GroupVersionKind())
	if err != nil {
		return nil, err
	}

	return newIngress, nil
}
