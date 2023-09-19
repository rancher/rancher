package rke

import (
	rkeClientV1 "github.com/rancher/rancher/pkg/generated/clientset/versioned/typed/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"k8s.io/client-go/rest"
)

// Client is a struct that embeds the `RkeV1Interface` (provisioning client) and has Session as an attribute
// The session.Session attributes is passed all way down to the Cluster
type Client struct {
	rkeClientV1.RkeV1Interface
	ts *session.Session
}

// RKEControlPlane is a struct that embeds RKEControlPlaneInterface and has session.Session as an attribute to keep track of the resources created by RKEControlPlaneInterface
type RKEControlPlane struct { // nolint:all
	rkeClientV1.RKEControlPlaneInterface
	ts *session.Session
}

// NewForConfig creates a new RkeV1Client for the given config. It also takes session.Session as parameter to track the resources
// the ProvisioningV1Client creates
func NewForConfig(c *rest.Config, ts *session.Session) (*Client, error) {
	rkeClient, err := rkeClientV1.NewForConfig(c)
	if err != nil {
		return nil, err
	}

	return &Client{rkeClient, ts}, nil
}

// RKEControlPlanes takes a namespace and returns an RKEControlPlane object that is used for the CRUD of a pkg/apis/rke.cattle.io/v1 RKEControlPlane
func (p *Client) RKEControlPlanes(namespace string) *RKEControlPlane {
	return &RKEControlPlane{p.RkeV1Interface.RKEControlPlanes(namespace), p.ts}
}
