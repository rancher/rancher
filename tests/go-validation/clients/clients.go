package clients

import (
	"os"

	"github.com/rancher/norman/clientbase"
	v3 "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v1 "github.com/rancher/rancher/pkg/generated/clientset/versioned/typed/provisioning.cattle.io/v1"
	coreV1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

var Host string = os.Getenv("CATTLE_TEST_URL")
var AdminToken string = os.Getenv("ADMIN_TOKEN")
var UserToken string = os.Getenv("USER_TOKEN")

const (
	DOResourceConfig = "digitaloceanconfigs"
)

// BearerTokenList is a helper function that checks to see if there's a user token, and puts it in a list.
func BearerTokensList() map[string]string {
	bearerTokensList := make(map[string]string)
	bearerTokensList["Admin User"] = AdminToken
	if UserToken != "" {
		bearerTokensList["Standard User"] = UserToken
	}

	return bearerTokensList
}

// NewRestConfig is the config used the various clients
func NewRestConfig(bearerToken string) *rest.Config {
	return &rest.Config{
		Host:        Host,
		BearerToken: bearerToken,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}
}

func NewClientOpts(bearerToken string) *clientbase.ClientOpts {
	return &clientbase.ClientOpts{
		URL:      Host + "/v3",
		TokenKey: bearerToken,
		Insecure: true,
	}
}

// NewProvisioningClient creates a *ProvisioningV1Client object to provision clusters using the rancher api
func NewProvisioningClient(bearerToken string) (*v1.ProvisioningV1Client, error) {
	restConfig := NewRestConfig(bearerToken)
	client, err := v1.NewForConfig(restConfig)
	return client, err
}

// NewPodConfigClient is function that creates a dynamic client to create machine pool configs.
func NewPodConfigClient(resource, bearerToken string) (dynamic.NamespaceableResourceInterface, error) {
	restConfig := NewRestConfig(bearerToken)
	dynamic, err := dynamic.NewForConfig(restConfig)

	if err != nil {
		return nil, err
	}
	dynamicResource := dynamic.Resource(schema.GroupVersionResource{
		Group:    "rke-machine-config.cattle.io",
		Version:  "v1",
		Resource: resource,
	})
	return dynamicResource, nil
}

// NewCoreV1Client creates a coreV1.Interface object
func NewCoreV1Client(bearerToken string) (coreV1.Interface, error) {
	restConfig := NewRestConfig(bearerToken)
	client, err := coreV1.NewForConfig(*restConfig)
	return client, err
}

// NewManagementClient creates a coreV1.Interface object
func NewManagementClient(bearerToken string) (*v3.Client, error) {
	clientOpts := NewClientOpts(bearerToken)
	client, err := v3.NewClient(clientOpts)
	return client, err
}
