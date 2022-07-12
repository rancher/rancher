package rke

import (
	rkeClientV1 "github.com/rancher/rancher/pkg/generated/clientset/versioned/typed/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"k8s.io/client-go/rest"
)

// Client is a struct that embeds the `ProvisioningV1Interface` (provisioning client) and has Session as an attribute
// The session.Session attributes is passed all way down to the Cluster
type Client struct {
	rkeClientV1.RkeV1Interface
	ts *session.Session
}


// NewForConfig creates a new ProvisioningV1Client for the given config. It also takes session.Session as parameter to track the resources
// the ProvisioningV1Client creates
func NewForConfig(c *rest.Config, ts *session.Session) (*Client, error) {
	rkeClient, err := rkeClientV1.NewForConfig(c)
	if err != nil {
		return nil, err
	}

	return &Client{rkeClient, ts}, nil
}

