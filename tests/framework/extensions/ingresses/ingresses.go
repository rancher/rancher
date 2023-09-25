package ingresses

import (
	"fmt"
	"net/http"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
)

const (
	IngressSteveType = "networking.k8s.io.ingress"
)

// GetExternalIngressResponse gets a response from a specific hostname and path.
// Returns the response and an error if any.
func GetExternalIngressResponse(client *rancher.Client, hostname string, path string, isWithTLS bool) (*http.Response, error) {
	protocol := "http"

	if isWithTLS {
		protocol = "https"
	}

	url := fmt.Sprintf("%s://%s/%s", protocol, hostname, path)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+client.RancherConfig.AdminToken)

	resp, err := client.Management.APIBaseClient.Ops.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return resp, nil
}

// IsIngressExternallyAccessible checks if the ingress is accessible externally,
// it returns true if the ingress is accessible, false if it is not, and an error if there is an error.
func IsIngressExternallyAccessible(client *rancher.Client, hostname string, path string, isWithTLS bool) (bool, error) {
	resp, err := GetExternalIngressResponse(client, hostname, path, isWithTLS)
	if err != nil {
		return false, err
	}

	return resp.StatusCode == http.StatusOK, nil
}
