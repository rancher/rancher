package clientbase

import (
	"encoding/json"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type ObjectFactory interface {
	Object() runtime.Object
	List() runtime.Object
}

type UnstructuredObjectFactory struct {
}

func (u *UnstructuredObjectFactory) Object() runtime.Object {
	return &unstructured.Unstructured{}
}

func (u *UnstructuredObjectFactory) List() runtime.Object {
	return &unstructured.UnstructuredList{}
}

type GenericClient interface {
	UnstructuredClient() GenericClient
	GroupVersionKind() schema.GroupVersionKind
	Create(o runtime.Object) (runtime.Object, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (runtime.Object, error)
	Get(name string, opts metav1.GetOptions) (runtime.Object, error)
	Update(name string, o runtime.Object) (runtime.Object, error)
	DeleteNamespaced(namespace, name string, opts *metav1.DeleteOptions) error
	Delete(name string, opts *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (runtime.Object, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOptions *metav1.DeleteOptions, listOptions metav1.ListOptions) error
	Patch(name string, o runtime.Object, data []byte, subresources ...string) (runtime.Object, error)
	ObjectFactory() ObjectFactory
}

type ObjectClient struct {
	restClient rest.Interface
	resource   *metav1.APIResource
	gvk        schema.GroupVersionKind
	ns         string
	Factory    ObjectFactory
}

func NewObjectClient(namespace string, restClient rest.Interface, apiResource *metav1.APIResource, gvk schema.GroupVersionKind, factory ObjectFactory) *ObjectClient {
	return &ObjectClient{
		restClient: restClient,
		resource:   apiResource,
		gvk:        gvk,
		ns:         namespace,
		Factory:    factory,
	}
}

func (p *ObjectClient) UnstructuredClient() GenericClient {
	return &ObjectClient{
		restClient: p.restClient,
		resource:   p.resource,
		gvk:        p.gvk,
		ns:         p.ns,
		Factory:    &UnstructuredObjectFactory{},
	}
}

func (p *ObjectClient) GroupVersionKind() schema.GroupVersionKind {
	return p.gvk
}

func (p *ObjectClient) getAPIPrefix() string {
	if p.gvk.Group == "" {
		return "api"
	}
	return "apis"
}

func (p *ObjectClient) Create(o runtime.Object) (runtime.Object, error) {
	ns := p.ns
	if obj, ok := o.(metav1.Object); ok && obj.GetNamespace() != "" {
		ns = obj.GetNamespace()
	}
	if t, err := meta.TypeAccessor(o); err == nil {
		if t.GetKind() == "" {
			t.SetKind(p.gvk.Kind)
		}
		if t.GetAPIVersion() == "" {
			apiVersion, _ := p.gvk.ToAPIVersionAndKind()
			t.SetAPIVersion(apiVersion)
		}
	}
	result := p.Factory.Object()
	err := p.restClient.Post().
		Prefix(p.getAPIPrefix(), p.gvk.Group, p.gvk.Version).
		NamespaceIfScoped(ns, p.resource.Namespaced).
		Resource(p.resource.Name).
		Body(o).
		Do().
		Into(result)
	return result, err
}

func (p *ObjectClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (runtime.Object, error) {
	result := p.Factory.Object()
	req := p.restClient.Get().
		Prefix(p.getAPIPrefix(), p.gvk.Group, p.gvk.Version)
	if namespace != "" {
		req = req.Namespace(namespace)
	}
	err := req.
		Resource(p.resource.Name).
		VersionedParams(&opts, dynamic.VersionedParameterEncoderWithV1Fallback).
		Name(name).
		Do().
		Into(result)
	return result, err

}

func (p *ObjectClient) Get(name string, opts metav1.GetOptions) (runtime.Object, error) {
	result := p.Factory.Object()
	err := p.restClient.Get().
		Prefix(p.getAPIPrefix(), p.gvk.Group, p.gvk.Version).
		NamespaceIfScoped(p.ns, p.resource.Namespaced).
		Resource(p.resource.Name).
		VersionedParams(&opts, dynamic.VersionedParameterEncoderWithV1Fallback).
		Name(name).
		Do().
		Into(result)
	return result, err
}

func (p *ObjectClient) Update(name string, o runtime.Object) (runtime.Object, error) {
	ns := p.ns
	if obj, ok := o.(metav1.Object); ok && obj.GetNamespace() != "" {
		ns = obj.GetNamespace()
	}
	result := p.Factory.Object()
	if len(name) == 0 {
		return result, errors.New("object missing name")
	}
	err := p.restClient.Put().
		Prefix(p.getAPIPrefix(), p.gvk.Group, p.gvk.Version).
		NamespaceIfScoped(ns, p.resource.Namespaced).
		Resource(p.resource.Name).
		Name(name).
		Body(o).
		Do().
		Into(result)
	return result, err
}

func (p *ObjectClient) DeleteNamespaced(namespace, name string, opts *metav1.DeleteOptions) error {
	req := p.restClient.Delete().
		Prefix(p.getAPIPrefix(), p.gvk.Group, p.gvk.Version)
	if namespace != "" {
		req = req.Namespace(namespace)
	}
	return req.Resource(p.resource.Name).
		Name(name).
		Body(opts).
		Do().
		Error()
}

func (p *ObjectClient) Delete(name string, opts *metav1.DeleteOptions) error {
	return p.restClient.Delete().
		Prefix(p.getAPIPrefix(), p.gvk.Group, p.gvk.Version).
		NamespaceIfScoped(p.ns, p.resource.Namespaced).
		Resource(p.resource.Name).
		Name(name).
		Body(opts).
		Do().
		Error()
}

func (p *ObjectClient) List(opts metav1.ListOptions) (runtime.Object, error) {
	result := p.Factory.List()
	return result, p.restClient.Get().
		Prefix(p.getAPIPrefix(), p.gvk.Group, p.gvk.Version).
		NamespaceIfScoped(p.ns, p.resource.Namespaced).
		Resource(p.resource.Name).
		VersionedParams(&opts, dynamic.VersionedParameterEncoderWithV1Fallback).
		Do().
		Into(result)
}

func (p *ObjectClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	r, err := p.restClient.Get().
		Prefix(p.getAPIPrefix(), p.gvk.Group, p.gvk.Version).
		Prefix("watch").
		NamespaceIfScoped(p.ns, p.resource.Namespaced).
		Resource(p.resource.Name).
		VersionedParams(&opts, dynamic.VersionedParameterEncoderWithV1Fallback).
		Stream()
	if err != nil {
		return nil, err
	}
	return watch.NewStreamWatcher(&dynamicDecoder{
		factory: p.Factory,
		dec:     json.NewDecoder(r),
		close:   r.Close,
	}), nil
}

func (p *ObjectClient) DeleteCollection(deleteOptions *metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	return p.restClient.Delete().
		Prefix(p.getAPIPrefix(), p.gvk.Group, p.gvk.Version).
		NamespaceIfScoped(p.ns, p.resource.Namespaced).
		Resource(p.resource.Name).
		VersionedParams(&listOptions, dynamic.VersionedParameterEncoderWithV1Fallback).
		Body(deleteOptions).
		Do().
		Error()
}

func (p *ObjectClient) Patch(name string, o runtime.Object, data []byte, subresources ...string) (runtime.Object, error) {
	ns := p.ns
	if obj, ok := o.(metav1.Object); ok && obj.GetNamespace() != "" {
		ns = obj.GetNamespace()
	}
	result := p.Factory.Object()
	if len(name) == 0 {
		return result, errors.New("object missing name")
	}
	err := p.restClient.Patch(types.StrategicMergePatchType).
		Prefix(p.getAPIPrefix(), p.gvk.Group, p.gvk.Version).
		NamespaceIfScoped(ns, p.resource.Namespaced).
		Resource(p.resource.Name).
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return result, err
}

func (p *ObjectClient) ObjectFactory() ObjectFactory {
	return p.Factory
}

type dynamicDecoder struct {
	factory ObjectFactory
	dec     *json.Decoder
	close   func() error
}

func (d *dynamicDecoder) Close() {
	d.close()
}

func (d *dynamicDecoder) Decode() (action watch.EventType, object runtime.Object, err error) {
	e := dynamicEvent{
		Object: holder{
			factory: d.factory,
		},
	}
	if err := d.dec.Decode(&e); err != nil {
		return watch.Error, nil, err
	}
	return e.Type, e.Object.obj, nil
}

type dynamicEvent struct {
	Type   watch.EventType
	Object holder
}

type holder struct {
	factory ObjectFactory
	obj     runtime.Object
}

func (h *holder) UnmarshalJSON(b []byte) error {
	h.obj = h.factory.Object()
	return json.Unmarshal(b, h.obj)
}
