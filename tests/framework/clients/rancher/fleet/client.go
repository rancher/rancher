package fleet

import (
	fleetClientV1 "github.com/rancher/rancher/pkg/generated/clientset/versioned/typed/fleet.cattle.io/v1alpha1"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"k8s.io/client-go/rest"
)

// Client is a struct that embedds the `FleetV1alpha1Interface` (fleet client) and has Session as an attribute
// The session.Session attributes is passed all way down to the Cluster
type Client struct {
	fleetClientV1.FleetV1alpha1Interface
	ts *session.Session
}

// GitRepo is a struct that embedds GitRepoInterface and has session.Session as an attribute to keep track of the resources created by GitRepoInterface
type GitRepo struct {
	fleetClientV1.GitRepoInterface
	ts *session.Session
}

// ClusterGroup is a struct that embedds ClusterGroupInterface and has session.Session as an attribute to keep track of the resources created by ClusterGrougInterface
type ClusterGroup struct {
	fleetClientV1.ClusterGroupInterface
	ts *session.Session
}

// NewForConfig creates a new FleetV1alpha1Client for the given config. It also takes session.Session as parameter to track the resources
// the FleetV1alpha1Client creates
func NewForConfig(c *rest.Config, ts *session.Session) (*Client, error) {
	fleetClient, err := fleetClientV1.NewForConfig(c)
	if err != nil {
		return nil, err
	}

	return &Client{fleetClient, ts}, nil
}

// GitRepos takes a namespace a returns a GitRepo object that is used for the CRUD of a github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1 GitRepo
func (c *Client) GitRepos(namespace string) *GitRepo {
	return &GitRepo{c.FleetV1alpha1Interface.GitRepos(namespace), c.ts}
}

// ClusterGroups takes a namespace a returns a ClusterGroups object that is used for the CRUD of a github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1 ClusterGroup
func (c *Client) ClusterGroups(namespace string) *ClusterGroup {
	return &ClusterGroup{c.FleetV1alpha1Interface.ClusterGroups(namespace), c.ts}
}
