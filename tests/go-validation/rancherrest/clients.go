package rancherrest

import (
	"context"

	v1 "github.com/rancher/rancher/pkg/generated/clientset/versioned/typed/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/rancher/tests/go-validation/utils"
	"github.com/rancher/wrangler/pkg/kubeconfig"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

const (
	DOResourceConfig = "digitaloceanconfigs"
)

type Client struct {
	Context *wrangler.Context
	Dynamic dynamic.Interface
	Ctx     context.Context
	cancel  func()
}

type PodConfigClient struct {
	dynamic.NamespaceableResourceInterface
	Ctx context.Context
}

type ProvisioningV1Client struct {
	*v1.ProvisioningV1Client
}

// NewClient creates a dynamic client used for things like creating cloud credentials, and machine pools
func NewClient() (*Client, error) {
	ctx := context.Background()
	restConfig := utils.NewRestConfig()
	nonInteractive := kubeconfig.GetNonInteractiveClientConfig("")

	wranglerContext, err := wrangler.NewContext(ctx, nonInteractive, restConfig)
	if err != nil {
		return nil, err
	}

	dynamic, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)

	client := &Client{
		Context: wranglerContext,
		Dynamic: dynamic,
		Ctx:     ctx,
		cancel:  cancel,
	}

	return client, nil
}

func (c *Client) Close() {
	c.cancel()
}

// NewClusterClient creates a *ProvisioningV1Client object that encapsulates *v1.ProvisioningV1Client to provision clusters using the rancher api
func NewProvisioningClient() (*ProvisioningV1Client, error) {
	restConfig := utils.NewRestConfig()
	client, err := v1.NewForConfig(restConfig)

	provisioningV1Client := &ProvisioningV1Client{
		client,
	}
	return provisioningV1Client, err
}

// NewPodConfigClient is function that creates a dynamic client to create machine pools. It's a member function
// of Client using it's dynamic resource.
func (c *Client) NewPodConfigClient(resource string) *PodConfigClient {
	dynamicResource := c.Dynamic.Resource(schema.GroupVersionResource{
		Group:    "rke-machine-config.cattle.io",
		Version:  "v1",
		Resource: resource,
	})

	return &PodConfigClient{
		NamespaceableResourceInterface: dynamicResource,
		Ctx:                            c.Ctx,
	}
}
