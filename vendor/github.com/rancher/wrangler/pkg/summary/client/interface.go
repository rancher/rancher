package client

import (
	"github.com/rancher/wrangler/pkg/summary"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
)

type Interface interface {
	Resource(resource schema.GroupVersionResource) NamespaceableResourceInterface
}

type ResourceInterface interface {
	List(opts metav1.ListOptions) (*summary.SummarizedObjectList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
}

type NamespaceableResourceInterface interface {
	Namespace(string) ResourceInterface
	ResourceInterface
}
