package client

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
)

type Client struct {
	RESTClient rest.Interface
	Namespaced bool
	GVR        schema.GroupVersionResource
	resource   string
	prefix     []string
}

func NewClient(gvr schema.GroupVersionResource, mapper meta.RESTMapper, client rest.Interface) (*Client, error) {
	kind, err := mapper.KindFor(gvr)
	if err != nil {
		return nil, err
	}

	mapping, err := mapper.RESTMapping(kind.GroupKind(), kind.Version)
	if err != nil {
		return nil, err
	}

	var (
		prefix []string
	)

	if gvr.Group == "" {
		prefix = []string{
			"api",
			gvr.Version,
		}
	} else {
		prefix = []string{
			"apis",
			gvr.Group,
			gvr.Version,
		}
	}

	return &Client{
		RESTClient: client,
		Namespaced: mapping.Scope.Name() == meta.RESTScopeNameNamespace,
		GVR:        gvr,
		prefix:     prefix,
		resource:   gvr.Resource,
	}, nil
}

func (c *Client) Get(ctx context.Context, namespace, name string, result runtime.Object, options metav1.GetOptions) (err error) {
	err = c.RESTClient.Get().
		Prefix(c.prefix...).
		NamespaceIfScoped(namespace, c.Namespaced).
		Resource(c.resource).
		Name(name).
		VersionedParams(&options, metav1.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

func (c *Client) List(ctx context.Context, namespace string, result runtime.Object, opts metav1.ListOptions) (err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	r := c.RESTClient.Get()
	if namespace != "" {
		r = r.NamespaceIfScoped(namespace, c.Namespaced)
	}
	err = r.Resource(c.resource).
		Prefix(c.prefix...).
		VersionedParams(&opts, metav1.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

func (c *Client) Watch(ctx context.Context, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.RESTClient.Get().
		Prefix(c.prefix...).
		NamespaceIfScoped(namespace, c.Namespaced).
		Resource(c.resource).
		VersionedParams(&opts, metav1.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

func (c *Client) Create(ctx context.Context, namespace string, obj, result runtime.Object, opts metav1.CreateOptions) (err error) {
	err = c.RESTClient.Post().
		Prefix(c.prefix...).
		NamespaceIfScoped(namespace, c.Namespaced).
		Resource(c.resource).
		VersionedParams(&opts, metav1.ParameterCodec).
		Body(obj).
		Do(ctx).
		Into(result)
	return
}

func (c *Client) Update(ctx context.Context, namespace string, obj, result runtime.Object, opts metav1.UpdateOptions) (err error) {
	m, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	err = c.RESTClient.Put().
		Prefix(c.prefix...).
		NamespaceIfScoped(namespace, c.Namespaced).
		Resource(c.resource).
		Name(m.GetName()).
		VersionedParams(&opts, metav1.ParameterCodec).
		Body(obj).
		Do(ctx).
		Into(result)
	return
}

func (c *Client) UpdateStatus(ctx context.Context, namespace string, obj, result runtime.Object, opts metav1.UpdateOptions) (err error) {
	m, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	err = c.RESTClient.Put().
		Prefix(c.prefix...).
		NamespaceIfScoped(namespace, c.Namespaced).
		Resource(c.resource).
		Name(m.GetName()).
		SubResource("status").
		VersionedParams(&opts, metav1.ParameterCodec).
		Body(obj).
		Do(ctx).
		Into(result)
	return
}

func (c *Client) Delete(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error {
	return c.RESTClient.Delete().
		Prefix(c.prefix...).
		NamespaceIfScoped(namespace, c.Namespaced).
		Resource(c.resource).
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

func (c *Client) DeleteCollection(ctx context.Context, namespace string, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.RESTClient.Delete().
		Prefix(c.prefix...).
		NamespaceIfScoped(namespace, c.Namespaced).
		Resource(c.resource).
		VersionedParams(&listOpts, metav1.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

func (c *Client) Patch(ctx context.Context, namespace, name string, pt types.PatchType, data []byte, result runtime.Object, opts metav1.PatchOptions, subresources ...string) (err error) {
	err = c.RESTClient.Patch(pt).
		Prefix(c.prefix...).
		Namespace(namespace).
		Resource(c.resource).
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, metav1.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
