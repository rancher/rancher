package provisioning

import (
	provisionClientV1 "github.com/rancher/rancher/pkg/generated/clientset/versioned/typed/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"k8s.io/client-go/rest"
)

// Client is a struct that embedds the `ProvisioningV1Interface` (provisioning client) and has Session as an attribute
// The session.Session attributes is passed all way down to the Cluster
type Client struct {
	provisionClientV1.ProvisioningV1Interface
	ts *session.Session
}

// Clusters is a struct that embedds the RESTClient and has session.Session and the namespace as an attribute to keep track of the resources created by the RESTClient
type Clusters struct {
	provisionClientV1.ClusterInterface
	client rest.Interface
	ts     *session.Session
	ns     string
}

// NewForConfig creates a new ProvisioningV1Client for the given config. It also takes session.Session as parameter to track the resources
// the ProvisioningV1Client creates
func NewForConfig(c *rest.Config, ts *session.Session) (*Client, error) {
	provClient, err := provisionClientV1.NewForConfig(c)
	if err != nil {
		return nil, err
	}

	return &Client{provClient, ts}, nil
}

// Clusters takes a namespace a returns a Cluster object that is used for the CRUD of a pkg/apis/provisioning.cattle.io/v1 Cluster
func (p *Client) Clusters(namespace string) *Clusters {
	return &Clusters{p.ProvisioningV1Interface.Clusters(namespace), p.RESTClient(), p.ts, namespace}
}
