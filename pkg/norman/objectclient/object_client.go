package objectclient

import (
	"context"

	"github.com/pkg/errors"
	"github.com/rancher/lasso/pkg/client"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
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
	UpdateStatus(name string, o runtime.Object) (runtime.Object, error)
	DeleteNamespaced(namespace, name string, opts *metav1.DeleteOptions) error
	Delete(name string, opts *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (runtime.Object, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (runtime.Object, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOptions *metav1.DeleteOptions, listOptions metav1.ListOptions) error
	Patch(name string, o runtime.Object, patchType types.PatchType, data []byte, subresources ...string) (runtime.Object, error)
	ObjectFactory() ObjectFactory
}

type ObjectClient struct {
	ctx      context.Context
	client   *client.Client
	resource *metav1.APIResource
	gvk      schema.GroupVersionKind
	ns       string
	Factory  ObjectFactory
}

func NewObjectClient(namespace string, client *client.Client, apiResource *metav1.APIResource, gvk schema.GroupVersionKind, factory ObjectFactory) *ObjectClient {
	return &ObjectClient{
		ctx:      context.TODO(),
		client:   client,
		resource: apiResource,
		gvk:      gvk,
		ns:       namespace,
		Factory:  factory,
	}
}

func (p *ObjectClient) UnstructuredClient() GenericClient {
	return &ObjectClient{
		ctx:      p.ctx,
		client:   p.client,
		resource: p.resource,
		gvk:      p.gvk,
		ns:       p.ns,
		Factory:  &UnstructuredObjectFactory{},
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
	obj, ok := o.(metav1.Object)
	if ok && obj.GetNamespace() != "" {
		ns = obj.GetNamespace()
	}

	if ok {
		labels := obj.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		} else {
			ls := make(map[string]string)
			for k, v := range labels {
				ls[k] = v
			}
			labels = ls

		}
		labels["cattle.io/creator"] = "norman"
		obj.SetLabels(labels)
	}

	logrus.Tracef("REST CREATE %s/%s/%s/%s/%s", p.getAPIPrefix(), p.gvk.Group, p.gvk.Version, ns, p.resource.Name)
	result := p.ObjectFactory().Object()
	return result, p.client.Create(p.ctx, ns, o, result, metav1.CreateOptions{})
}

func (p *ObjectClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (runtime.Object, error) {
	logrus.Tracef("REST GET %s/%s/%s/%s/%s/%s", p.getAPIPrefix(), p.gvk.Group, p.gvk.Version, namespace, p.resource.Name, name)
	result := p.Factory.Object()
	return result, p.client.Get(p.ctx, namespace, name, result, opts)
}

func (p *ObjectClient) Get(name string, opts metav1.GetOptions) (runtime.Object, error) {
	logrus.Tracef("REST GET %s/%s/%s/%s/%s/%s", p.getAPIPrefix(), p.gvk.Group, p.gvk.Version, p.ns, p.resource.Name, name)
	result := p.Factory.Object()
	return result, p.client.Get(p.ctx, p.ns, name, result, opts)
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
	logrus.Tracef("REST UPDATE %s/%s/%s/%s/%s/%s", p.getAPIPrefix(), p.gvk.Group, p.gvk.Version, ns, p.resource.Name, name)
	return result, p.client.Update(p.ctx, ns, o, result, metav1.UpdateOptions{})
}

func (p *ObjectClient) UpdateStatus(name string, o runtime.Object) (runtime.Object, error) {
	ns := p.ns
	if obj, ok := o.(metav1.Object); ok && obj.GetNamespace() != "" {
		ns = obj.GetNamespace()
	}
	result := p.Factory.Object()
	if len(name) == 0 {
		return result, errors.New("object missing name")
	}
	logrus.Tracef("REST UPDATE %s/%s/%s/%s/status/%s/%s", p.getAPIPrefix(), p.gvk.Group, p.gvk.Version, ns, p.resource.Name, name)
	return result, p.client.UpdateStatus(p.ctx, ns, o, result, metav1.UpdateOptions{})
}

func (p *ObjectClient) DeleteNamespaced(namespace, name string, opts *metav1.DeleteOptions) error {
	logrus.Tracef("REST DELETE %s/%s/%s/%s/%s/%s", p.getAPIPrefix(), p.gvk.Group, p.gvk.Version, namespace, p.resource.Name, name)
	if opts == nil {
		opts = &metav1.DeleteOptions{}
	}
	return p.client.Delete(p.ctx, namespace, name, *opts)
}

func (p *ObjectClient) Delete(name string, opts *metav1.DeleteOptions) error {
	logrus.Tracef("REST DELETE %s/%s/%s/%s/%s/%s", p.getAPIPrefix(), p.gvk.Group, p.gvk.Version, p.ns, p.resource.Name, name)
	if opts == nil {
		opts = &metav1.DeleteOptions{}
	}
	return p.client.Delete(p.ctx, p.ns, name, *opts)
}

func (p *ObjectClient) List(opts metav1.ListOptions) (runtime.Object, error) {
	result := p.Factory.List()
	logrus.Tracef("REST LIST %s/%s/%s/%s/%s", p.getAPIPrefix(), p.gvk.Group, p.gvk.Version, p.ns, p.resource.Name)
	return result, p.client.List(p.ctx, p.ns, result, opts)
}

func (p *ObjectClient) ListNamespaced(namespace string, opts metav1.ListOptions) (runtime.Object, error) {
	result := p.Factory.List()
	logrus.Tracef("REST LIST %s/%s/%s/%s/%s", p.getAPIPrefix(), p.gvk.Group, p.gvk.Version, namespace, p.resource.Name)
	return result, p.client.List(p.ctx, namespace, result, opts)
}

func (p *ObjectClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return p.client.Watch(p.ctx, p.ns, opts)
}

func (p *ObjectClient) DeleteCollection(deleteOptions *metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	if deleteOptions == nil {
		deleteOptions = &metav1.DeleteOptions{}
	}
	return p.client.DeleteCollection(p.ctx, p.ns, *deleteOptions, listOptions)
}

func (p *ObjectClient) Patch(name string, o runtime.Object, patchType types.PatchType, data []byte, subresources ...string) (runtime.Object, error) {
	ns := p.ns
	if obj, ok := o.(metav1.Object); ok && obj.GetNamespace() != "" {
		ns = obj.GetNamespace()
	}
	result := p.Factory.Object()
	if len(name) == 0 {
		return result, errors.New("object missing name")
	}
	return result, p.client.Patch(p.ctx, ns, name, patchType, data, result, metav1.PatchOptions{}, subresources...)
}

func (p *ObjectClient) ObjectFactory() ObjectFactory {
	return p.Factory
}
