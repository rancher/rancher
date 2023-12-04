package dynamic

import (
	"context"
	"fmt"

	"k8s.io/client-go/rest"

	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// Client is a struct that embedds the dynamic.Interface(dynamic client) and has Session as an attribute
// The session.Session attributes is passed all way down to the ResourceClient to keep track of the resources created by the dynamic client
type Client struct {
	dynamic.Interface
	ts *session.Session
}

// NewForConfig creates a new dynamic client or returns an error.
func NewForConfig(ts *session.Session, inConfig *rest.Config) (*Client, error) {
	logrus.Debugf("Dynamic Client Host:%s", inConfig.Host)

	dynamicClient, err := dynamic.NewForConfig(inConfig)
	if err != nil {
		return nil, err
	}

	return &Client{
		Interface: dynamicClient,
		ts:        ts,
	}, nil
}

// Resource takes a schema.GroupVersionResource parameter to set the appropriate resource interface e.g.
//
//	 schema.GroupVersionResource {
//		  Group:    "management.cattle.io",
//		  Version:  "v3",
//		  Resource: "users",
//	 }
func (d *Client) Resource(resource schema.GroupVersionResource) *NamespaceableResourceClient {
	return &NamespaceableResourceClient{
		NamespaceableResourceInterface: d.Interface.Resource(resource),
		ts:                             d.ts,
	}
}

// NamespaceableResourceClient is a struct that has dynamic.NamespaceableResourceInterface embedded, and has session.Session as an attribute.
// This is inorder to overwrite dynamic.NamespaceableResourceInterface's Namespace function.
type NamespaceableResourceClient struct {
	dynamic.NamespaceableResourceInterface
	ts *session.Session
}

// Namespace returns a dynamic.ResourceInterface that is embedded in ResourceClient, so ultimately its Create is overwritten.
func (d *NamespaceableResourceClient) Namespace(s string) *ResourceClient {
	return &ResourceClient{
		ResourceInterface: d.NamespaceableResourceInterface.Namespace(s),
		ts:                d.ts,
	}
}

// ResourceClient has dynamic.ResourceInterface embedded so dynamic.ResourceInterface's Create can be overwritten.
type ResourceClient struct {
	dynamic.ResourceInterface
	ts *session.Session
}

var (
	// some GVKs are special and cannot be cleaned up because they do not exist
	// after being created (eg: SelfSubjectAccessReview). We'll not register
	// cleanup functions when creating objects of these kinds.
	noCleanupGVKs = []schema.GroupVersionKind{
		{
			Group:   "authorization.k8s.io",
			Version: "v1",
			Kind:    "SelfSubjectAccessReview",
		},
	}
)

func needsCleanup(obj *unstructured.Unstructured) bool {
	for _, gvk := range noCleanupGVKs {
		if obj.GroupVersionKind() == gvk {
			return false
		}
	}
	return true
}

// Create is dynamic.ResourceInterface's Create function, that is being overwritten to register its delete function to the session.Session
// that is being reference.
func (c *ResourceClient) Create(ctx context.Context, obj *unstructured.Unstructured, opts metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	unstructuredObj, err := c.ResourceInterface.Create(ctx, obj, opts, subresources...)
	if err != nil {
		return nil, err
	}

	if needsCleanup(obj) {
		c.ts.RegisterCleanupFunc(func() error {
			err := c.Delete(context.TODO(), unstructuredObj.GetName(), metav1.DeleteOptions{}, subresources...)
			if errors.IsNotFound(err) {
				return nil
			}

			name := unstructuredObj.GetName()
			if unstructuredObj.GetNamespace() != "" {
				name = unstructuredObj.GetNamespace() + "/" + name
			}
			gvk := unstructuredObj.GetObjectKind().GroupVersionKind()

			return fmt.Errorf("unable to delete (%v) %v: %w", gvk, name, err)
		})
	}

	return unstructuredObj, err
}
