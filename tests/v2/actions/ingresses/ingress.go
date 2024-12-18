package ingresses

import (
	"fmt"

	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

// UpdateIngress updates an existing ingress with new specifications
func UpdateIngress(steveClient *v1.Client, ingress *v1.SteveAPIObject, updatedIngressSpec *networkingv1.Ingress) (*v1.SteveAPIObject, error) {
	existingIngressObj, err := steveClient.SteveType(ingress.Type).ByID(ingress.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing ingress: %v", err)
	}

	updatedIngressSpec.ResourceVersion = existingIngressObj.ResourceVersion

	updatedIngressObj, err := steveClient.SteveType(ingress.Type).Update(existingIngressObj, updatedIngressSpec)
	if err != nil {
		return nil, fmt.Errorf("failed to update ingress: %v", err)
	}

	return updatedIngressObj, nil
}
