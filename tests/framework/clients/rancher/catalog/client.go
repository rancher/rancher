package catalog

import (
	catalogClientV1 "github.com/rancher/rancher/pkg/generated/clientset/versioned/typed/catalog.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"k8s.io/client-go/rest"
)

// Client is a struct that embedds the `CatalogV1Interface` (catalog client)
type Client struct {
	catalogClientV1.CatalogV1Interface
}

// NewForConfig creates a new CatalogV1Client for the given config. I
func NewForConfig(c *rest.Config, ts *session.Session) (*Client, error) {
	catalogClient, err := catalogClientV1.NewForConfig(c)
	if err != nil {
		return nil, err
	}

	return &Client{catalogClient}, nil
}
