package dynamic

import (
	"context"

	"github.com/rancher/rancher/tests/automation-framework/testsession"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type dynamicClient struct {
	dynamic.Interface
	testSession *testsession.TestSession
}

var _ Interface = &dynamicClient{}

type Interface interface {
	Resource(resource schema.GroupVersionResource) NamespaceableResourceInterface
}

type ResourceInterface interface {
	Create(ctx context.Context, obj *unstructured.Unstructured, options metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error)
	Update(ctx context.Context, obj *unstructured.Unstructured, options metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error)
	UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, options metav1.UpdateOptions) (*unstructured.Unstructured, error)
	Delete(ctx context.Context, name string, options metav1.DeleteOptions, subresources ...string) error
	DeleteCollection(ctx context.Context, options metav1.DeleteOptions, listOptions metav1.ListOptions) error
	Get(ctx context.Context, name string, options metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error)
	List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, options metav1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error)
}

type NamespaceableResourceInterface interface {
	Namespace(string) ResourceInterface
	ResourceInterface
}

type dynamicResourceClient struct {
	dynamic.NamespaceableResourceInterface
	*dynamicClient
}

// NewForConfig creates a new dynamic client or returns an error.
func NewForConfig(inConfig *rest.Config, testSession *testsession.TestSession) (Interface, error) {
	dynamic, err := dynamic.NewForConfig(inConfig)
	if err != nil {
		return nil, err
	}

	return &dynamicClient{Interface: dynamic, testSession: testSession}, nil
}

func (c *dynamicClient) Resource(resource schema.GroupVersionResource) NamespaceableResourceInterface {
	return &dynamicResourceClient{c.Interface.Resource(resource), c}
}

func (c *dynamicResourceClient) Namespace(ns string) ResourceInterface {
	ret := *c
	ret.NamespaceableResourceInterface.Namespace(ns)
	return &ret
}

func (c *dynamicResourceClient) Create(ctx context.Context, obj *unstructured.Unstructured, opts metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	resp, err := c.NamespaceableResourceInterface.Create(ctx, obj, opts, subresources...)
	if err != nil {
		return nil, err
	}

	deleteFunction := func() error {
		c.Delete(ctx, resp.GetName(), metav1.DeleteOptions{}, subresources...)
		return nil
	}

	c.dynamicClient.testSession.RegisterCleanupFunc(deleteFunction)
	return resp, nil
}

func (c *dynamicResourceClient) Update(ctx context.Context, obj *unstructured.Unstructured, opts metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return c.NamespaceableResourceInterface.Update(ctx, obj, opts, subresources...)
}

func (c *dynamicResourceClient) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, opts metav1.UpdateOptions) (*unstructured.Unstructured, error) {
	return c.NamespaceableResourceInterface.UpdateStatus(ctx, obj, opts)
}

func (c *dynamicResourceClient) Delete(ctx context.Context, name string, opts metav1.DeleteOptions, subresources ...string) error {
	return c.NamespaceableResourceInterface.Delete(ctx, name, opts, subresources...)
}

func (c *dynamicResourceClient) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	return c.NamespaceableResourceInterface.DeleteCollection(ctx, opts, listOptions)
}

func (c *dynamicResourceClient) Get(ctx context.Context, name string, opts metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return c.NamespaceableResourceInterface.Get(ctx, name, opts, subresources...)
}

func (c *dynamicResourceClient) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	return c.NamespaceableResourceInterface.List(ctx, opts)
}

func (c *dynamicResourceClient) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return c.NamespaceableResourceInterface.Watch(ctx, opts)
}

func (c *dynamicResourceClient) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return c.NamespaceableResourceInterface.Patch(ctx, name, pt, data, opts, subresources...)
}
