package primechecks

import (
	"fmt"
	"strings"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	client "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/rancherversion"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

const (
	PodResourceSteveType = "pod"
	rancherImage         = "rancher"
)

// CheckUIBrand checks the UI brand of Rancher Prime. If the Rancher instance is not Rancher Prime, the UI brand should be blank.
func CheckUIBrand(client *rancher.Client, isPrime bool, rancherBrand *client.Setting, brand string) error {
	if isPrime && brand != rancherBrand.Value {
		return fmt.Errorf("error: Rancher Prime UI brand %s does not match defined UI brand %s", rancherBrand.Value, brand)
	}

	return nil
}

// CheckVersion checks the if Rancher Prime is set to true and the version of Rancher.
func CheckVersion(isPrime bool, rancherVersion string, serverConfig *rancherversion.Config) error {
	if isPrime && rancherVersion != serverConfig.RancherVersion {
		return fmt.Errorf("error: Rancher Prime: %t | Version: %s", isPrime, serverConfig.RancherVersion)
	}

	return nil
}

// CheckSystemDefaultRegistry checks if the system default registry is set to the expected value.
func CheckSystemDefaultRegistry(isPrime bool, primeRegistry string, registry *client.Setting) error {
	if isPrime && primeRegistry != registry.Value {
		return fmt.Errorf("error: Rancher Prime system default registry %s does not match user defined registry %s", registry.Value, primeRegistry)
	}

	return nil
}

// CheckLocalClusterRancherImages checks if the Rancher images are set to the expected registry.
func CheckLocalClusterRancherImages(client *rancher.Client, isPrime bool, rancherVersion, primeRegistry, clusterID string) ([]string, []error) {
	downstreamClient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return nil, []error{err}
	}

	steveClient := downstreamClient.SteveType(PodResourceSteveType)

	pods, err := steveClient.List(nil)
	if err != nil {
		return nil, []error{err}
	}

	var imageResults []string
	var imageErrors []error

	for _, pod := range pods.Data {
		podStatus := &corev1.PodStatus{}
		err = v1.ConvertToK8sType(pod.Status, podStatus)
		if err != nil {
			return nil, []error{err}
		}

		image := podStatus.ContainerStatuses[0].Image

		if (strings.Contains(image, primeRegistry) && isPrime) || (strings.Contains(image, rancherImage) && !isPrime) {
			imageResults = append(imageResults, fmt.Sprintf("INFO: %s: %s\n", pod.Name, image))
			logrus.Infof("Pod %s is using image: %s", pod.Name, image)
		} else if strings.Contains(image, rancherImage) && isPrime {
			imageErrors = append(imageErrors, fmt.Errorf("ERROR: %s: %s", pod.Name, image))
			logrus.Infof("Pod %s is using image: %s", pod.Name, image)
		}
	}

	return imageResults, imageErrors
}
