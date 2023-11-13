package ingresses

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/defaults"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	"github.com/sirupsen/logrus"
	networking "k8s.io/api/networking/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	IngressSteveType = "networking.k8s.io.ingress"
	pod              = "pod"
	IngressNginx     = "ingress-nginx"
	RancherWebhook   = "rancher-webhook"
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

// CreateIngress will create an Ingress object in the downstream cluster.
func CreateIngress(client *v1.Client, ingressName string, ingressTemplate networking.Ingress) (*v1.SteveAPIObject, error) {
	podClient := client.SteveType(pod)
	err := kwait.Poll(15*time.Second, defaults.FiveMinuteTimeout, func() (done bool, err error) {
		newPods, err := podClient.List(nil)
		if err != nil {
			return false, nil
		}
		if len(newPods.Data) != 0 {
			return true, nil
		}
		for _, pod := range newPods.Data {
			if strings.Contains(pod.Name, IngressNginx) || strings.Contains(pod.Name, RancherWebhook) {
				isReady, podError := pods.IsPodReady(&pod)

				if podError != nil {
					return false, nil
				}

				return isReady, nil
			}
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}

	logrus.Infof("Create Ingress: %v", ingressName)
	ingressResp, err := client.SteveType(IngressSteveType).Create(ingressTemplate)
	if err != nil {
		logrus.Errorf("Failed to create ingress: %v", err)
		return nil, err
	}

	logrus.Infof("Successfully created ingress: %v", ingressName)

	return ingressResp, err
}
