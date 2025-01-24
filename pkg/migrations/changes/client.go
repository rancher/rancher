package changes

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// Interface provides a read-only version of the dynamic.Interface.
//
// https://pkg.go.dev/k8s.io/client-go/dynamic#Interface
type Interface interface {
	Resource(resource schema.GroupVersionResource) NamespaceableResourceReader
}

// ResourceReader is a read-only version of the dynamic.ResourceInterface.
//
// https://pkg.go.dev/k8s.io/client-go/dynamic#NamespaceableResourceInterface
type ResourceReader interface {
	Get(ctx context.Context, name string, options metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error)
	List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error)
}

// NamespaceableResourceReader is a read-only version of the dynamic.ResourceInterface.
//
// https://pkg.go.dev/k8s.io/client-go/dynamic#ResourceInterface
type NamespaceableResourceReader interface {
	Namespace(string) ResourceReader
	ResourceReader
}

type ReadonlyDynamicClient struct {
	delegate dynamic.Interface
}

type readonlyResourceClient struct {
	delegate dynamic.ResourceInterface
}

type readonlyNamespaceableResourceClient struct {
	delegate dynamic.NamespaceableResourceInterface
}

func (r *readonlyNamespaceableResourceClient) Namespace(s string) ResourceReader {
	return &readonlyResourceClient{delegate: r.delegate.Namespace(s)}
}

func (r *readonlyResourceClient) Get(ctx context.Context, name string, options metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return r.delegate.Get(ctx, name, options, subresources...)
}

func (r *readonlyResourceClient) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	return r.delegate.List(ctx, opts)
}

func (r *readonlyNamespaceableResourceClient) Get(ctx context.Context, name string, options metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return r.delegate.Get(ctx, name, options, subresources...)
}

func (r *readonlyNamespaceableResourceClient) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	return r.delegate.List(ctx, opts)
}

func (r *ReadonlyDynamicClient) Resource(resource schema.GroupVersionResource) NamespaceableResourceReader {
	return &readonlyNamespaceableResourceClient{delegate: r.delegate.Resource(resource)}
}

// ClientFrom returns a read-only version of the provided dynamic.Interface
// which delegates to the underlying client.
func ClientFrom(client dynamic.Interface) *ReadonlyDynamicClient {
	return &ReadonlyDynamicClient{delegate: client}
}
