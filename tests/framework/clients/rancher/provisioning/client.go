package provisioning

import (
	provisionClientV1 "github.com/rancher/rancher/pkg/generated/clientset/versioned/typed/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"k8s.io/client-go/rest"
)

type Client struct {
	provisionClientV1.ProvisioningV1Interface
	ts *session.Session
}

type Cluster struct {
	provisionClientV1.ClusterInterface
	ts *session.Session
}

// NewForConfig creates a new ProvisioningV1Client for the given config.
func NewForConfig(c *rest.Config, ts *session.Session) (*Client, error) {
	provClient, err := provisionClientV1.NewForConfig(c)
	if err != nil {
		return nil, err
	}

	return &Client{provClient, ts}, nil
}

func (p *Client) Clusters(namespace string) *Cluster {
	return &Cluster{p.ProvisioningV1Interface.Clusters(namespace), p.ts}
}
