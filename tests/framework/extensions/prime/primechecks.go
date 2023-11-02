package primechecks

import (
	"fmt"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	client "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/rancherversion"
)

const (
	PodResourceSteveType = "pod"
	rancherImage         = "rancher"
)

// CheckUIBrand checks the UI brand of Rancher Prime. If the Rancher instance is not Rancher Prime, the UI brand should be blank.
func CheckUIBrand(client *rancher.Client, isPrime bool, rancherBrand *client.Setting, brand string) error {
	if isPrime && brand != rancherBrand.Default {
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
