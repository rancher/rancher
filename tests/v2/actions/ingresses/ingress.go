package ingresses

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rancher/shepherd/clients/rancher"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	ingressActiveTimeout = 10 * time.Minute
	ingressPollInterval  = 2 * time.Second
	IngressTestImage     = "nginx:latest"
	IngressTestPort      = 80
	IngressIPDomainV25   = "sslip.io"
	IngressIPDomainV26   = "nip.io"
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

// WaitForIngressToBeActive waits until the ingress gets an IP address
func WaitForIngressToBeActive(client *rancher.Client, clusterID, namespace, name string) {
	log.Infof("Waiting for ingress %s to be active", name)

	steveClient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		log.Fatalf("Failed to get downstream client: %v", err)
	}

	err = wait.Poll(ingressPollInterval, ingressActiveTimeout, func() (bool, error) {
		ingressObj, err := steveClient.SteveType("networking.k8s.io.ingress").ByID(fmt.Sprintf("%s/%s", namespace, name))
		if err != nil {
			log.Warnf("Failed to get ingress: %v", err)
			return false, nil
		}

		var ingress networkingv1.Ingress
		if err := v1.ConvertToK8sType(ingressObj, &ingress); err != nil {
			log.Warnf("Failed to convert ingress: %v", err)
			return false, nil
		}

		if len(ingress.Status.LoadBalancer.Ingress) > 0 {
			log.Info("Ingress has received an IP address")
			return true, nil
		}

		log.Infof("Waiting for ingress %s to get an IP address...", name)
		return false, nil
	})

	if err != nil {
		log.Fatalf("Timeout waiting for ingress to be active: %v", err)
	}
}

// ValidateIngress verifies the configuration of an ingress resource
func ValidateIngress(t *testing.T, steveClient *v1.Client, ingress *v1.SteveAPIObject, host, path, clusterID string) {
	ingressObj, err := steveClient.SteveType(ingress.Type).ByID(ingress.ID)
	require.NoError(t, err, "Failed to get ingress object")

	var k8sIngress networkingv1.Ingress
	err = v1.ConvertToK8sType(ingressObj, &k8sIngress)
	require.NoError(t, err, "Failed to convert ingress to k8s type")

	require.Len(t, k8sIngress.Spec.Rules, 1, "Should have one rule")
	require.Equal(t, host, k8sIngress.Spec.Rules[0].Host, "Host should match")
	require.Len(t, k8sIngress.Spec.Rules[0].HTTP.Paths, 1, "Should have one path")
	require.Equal(t, path, k8sIngress.Spec.Rules[0].HTTP.Paths[0].Path, "Path should match")
	require.Equal(t, int32(80), k8sIngress.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number, "Port should match")
	require.Greater(t, len(k8sIngress.Status.LoadBalancer.Ingress), 0, "Should have LoadBalancer IP")
	require.NotEmpty(t, k8sIngress.Status.LoadBalancer.Ingress[0].IP, "LoadBalancer IP should not be empty")
}

// ValidateIngressDeleted verifies that an ingress has been deleted
func ValidateIngressDeleted(client *rancher.Client, ingress *v1.SteveAPIObject) {
	log.Infof("Validating ingress %s is deleted", ingress.Name)

	steveClient, err := client.Steve.ProxyDownstream(ingress.ID)
	if err != nil {
		log.Fatalf("Failed to get downstream client: %v", err)
	}

	err = wait.Poll(ingressPollInterval, ingressActiveTimeout, func() (bool, error) {
		_, err := steveClient.SteveType("networking.k8s.io.ingress").ByID(ingress.ID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				log.Info("Ingress has been deleted")
				return true, nil
			}
			return false, fmt.Errorf("error checking ingress: %v", err)
		}
		log.Info("Waiting for ingress to be deleted...")
		return false, nil
	})

	if err != nil {
		log.Fatalf("Failed to validate ingress deletion: %v", err)
	}
}
