package generator

var controllerTemplate = `package {{.schema.Version.Version}}

import (
	"context"
	"time"

	{{.importPackage}}
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

var (
	{{.schema.CodeName}}GroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "{{.schema.CodeName}}",
	}
	{{.schema.CodeName}}Resource = metav1.APIResource{
		Name:         "{{.schema.PluralName | toLower}}",
		SingularName: "{{.schema.CodeName | toLower}}",
{{- if eq .schema.Scope "namespace" }}
		Namespaced:   true,
{{ else }}
		Namespaced:   false,
{{- end }}
		Kind:         {{.schema.CodeName}}GroupVersionKind.Kind,
	}

	{{.schema.CodeName}}GroupVersionResource = schema.GroupVersionResource{
		Group:     GroupName,
		Version:   Version,
		Resource:  "{{.schema.PluralName | toLower}}",
	}
)

func init() {
	resource.Put({{.schema.CodeName}}GroupVersionResource)
}

// Deprecated use {{.prefix}}{{.schema.CodeName}} instead
type {{.schema.CodeName}} = {{.prefix}}{{.schema.CodeName}}

func New{{.schema.CodeName}}(namespace, name string, obj {{.prefix}}{{.schema.CodeName}}) *{{.prefix}}{{.schema.CodeName}} {
	obj.APIVersion, obj.Kind = {{.schema.CodeName}}GroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

{{ if eq .prefix "" }}
type {{.schema.CodeName}}List struct {
	metav1.TypeMeta   %BACK%json:",inline"%BACK%
	metav1.ListMeta   %BACK%json:"metadata,omitempty"%BACK%
	Items             []{{.prefix}}{{.schema.CodeName}} %BACK%json:"items"%BACK%
}
{{- end }}

type {{.schema.CodeName}}HandlerFunc func(key string, obj *{{.prefix}}{{.schema.CodeName}}) (runtime.Object, error)

type {{.schema.CodeName}}ChangeHandlerFunc func(obj *{{.prefix}}{{.schema.CodeName}}) (runtime.Object, error)

type {{.schema.CodeName}}Lister interface {
	List(namespace string, selector labels.Selector) (ret []*{{.prefix}}{{.schema.CodeName}}, err error)
	Get(namespace, name string) (*{{.prefix}}{{.schema.CodeName}}, error)
}

type {{.schema.CodeName}}Controller interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() {{.schema.CodeName}}Lister
	AddHandler(ctx context.Context, name string, handler {{.schema.CodeName}}HandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync {{.schema.CodeName}}HandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler {{.schema.CodeName}}HandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler {{.schema.CodeName}}HandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type {{.schema.CodeName}}Interface interface {
    ObjectClient() *objectclient.ObjectClient
	Create(*{{.prefix}}{{.schema.CodeName}}) (*{{.prefix}}{{.schema.CodeName}}, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*{{.prefix}}{{.schema.CodeName}}, error)
	Get(name string, opts metav1.GetOptions) (*{{.prefix}}{{.schema.CodeName}}, error)
	Update(*{{.prefix}}{{.schema.CodeName}}) (*{{.prefix}}{{.schema.CodeName}}, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*{{.prefix}}{{.schema.CodeName}}List, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*{{.prefix}}{{.schema.CodeName}}List, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() {{.schema.CodeName}}Controller
	AddHandler(ctx context.Context, name string, sync {{.schema.CodeName}}HandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync {{.schema.CodeName}}HandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle {{.schema.CodeName}}Lifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle {{.schema.CodeName}}Lifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync {{.schema.CodeName}}HandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync {{.schema.CodeName}}HandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle {{.schema.CodeName}}Lifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle {{.schema.CodeName}}Lifecycle)
}

type {{.schema.ID}}Lister struct {
	controller *{{.schema.ID}}Controller
}

func (l *{{.schema.ID}}Lister) List(namespace string, selector labels.Selector) (ret []*{{.prefix}}{{.schema.CodeName}}, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*{{.prefix}}{{.schema.CodeName}}))
	})
	return
}

func (l *{{.schema.ID}}Lister) Get(namespace, name string) (*{{.prefix}}{{.schema.CodeName}}, error) {
	var key string
	if namespace != "" {
		key = namespace + "/" + name
	} else {
		key = name
	}
	obj, exists, err := l.controller.Informer().GetIndexer().GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(schema.GroupResource{
			Group: {{.schema.CodeName}}GroupVersionKind.Group,
			Resource: {{.schema.CodeName}}GroupVersionResource.Resource,
		}, key)
	}
	return obj.(*{{.prefix}}{{.schema.CodeName}}), nil
}

type {{.schema.ID}}Controller struct {
	controller.GenericController
}

func (c *{{.schema.ID}}Controller) Generic() controller.GenericController {
	return c.GenericController
}

func (c *{{.schema.ID}}Controller) Lister() {{.schema.CodeName}}Lister {
	return &{{.schema.ID}}Lister{
		controller: c,
	}
}


func (c *{{.schema.ID}}Controller) AddHandler(ctx context.Context, name string, handler {{.schema.CodeName}}HandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*{{.prefix}}{{.schema.CodeName}}); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *{{.schema.ID}}Controller) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler {{.schema.CodeName}}HandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*{{.prefix}}{{.schema.CodeName}}); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *{{.schema.ID}}Controller) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler {{.schema.CodeName}}HandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*{{.prefix}}{{.schema.CodeName}}); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *{{.schema.ID}}Controller) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler {{.schema.CodeName}}HandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*{{.prefix}}{{.schema.CodeName}}); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type {{.schema.ID}}Factory struct {
}

func (c {{.schema.ID}}Factory) Object() runtime.Object {
	return &{{.prefix}}{{.schema.CodeName}}{}
}

func (c {{.schema.ID}}Factory) List() runtime.Object {
	return &{{.prefix}}{{.schema.CodeName}}List{}
}

func (s *{{.schema.ID}}Client) Controller() {{.schema.CodeName}}Controller {
	genericController := controller.NewGenericController({{.schema.CodeName}}GroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind({{.schema.CodeName}}GroupVersionResource, {{.schema.CodeName}}GroupVersionKind.Kind, {{.schema | namespaced}}))

	return &{{.schema.ID}}Controller{
		GenericController: genericController,
	}
}

type {{.schema.ID}}Client struct {
	client *Client
	ns string
	objectClient *objectclient.ObjectClient
	controller   {{.schema.CodeName}}Controller
}

func (s *{{.schema.ID}}Client) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *{{.schema.ID}}Client) Create(o *{{.prefix}}{{.schema.CodeName}}) (*{{.prefix}}{{.schema.CodeName}}, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*{{.prefix}}{{.schema.CodeName}}), err
}

func (s *{{.schema.ID}}Client) Get(name string, opts metav1.GetOptions) (*{{.prefix}}{{.schema.CodeName}}, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*{{.prefix}}{{.schema.CodeName}}), err
}

func (s *{{.schema.ID}}Client) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*{{.prefix}}{{.schema.CodeName}}, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*{{.prefix}}{{.schema.CodeName}}), err
}

func (s *{{.schema.ID}}Client) Update(o *{{.prefix}}{{.schema.CodeName}}) (*{{.prefix}}{{.schema.CodeName}}, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*{{.prefix}}{{.schema.CodeName}}), err
}

func (s *{{.schema.ID}}Client) UpdateStatus(o *{{.prefix}}{{.schema.CodeName}}) (*{{.prefix}}{{.schema.CodeName}}, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*{{.prefix}}{{.schema.CodeName}}), err
}

func (s *{{.schema.ID}}Client) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *{{.schema.ID}}Client) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *{{.schema.ID}}Client) List(opts metav1.ListOptions) (*{{.prefix}}{{.schema.CodeName}}List, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*{{.prefix}}{{.schema.CodeName}}List), err
}

func (s *{{.schema.ID}}Client) ListNamespaced(namespace string, opts metav1.ListOptions) (*{{.prefix}}{{.schema.CodeName}}List, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*{{.prefix}}{{.schema.CodeName}}List), err
}

func (s *{{.schema.ID}}Client) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *{{.schema.ID}}Client) Patch(o *{{.prefix}}{{.schema.CodeName}}, patchType types.PatchType, data []byte, subresources ...string) (*{{.prefix}}{{.schema.CodeName}}, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*{{.prefix}}{{.schema.CodeName}}), err
}

func (s *{{.schema.ID}}Client) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *{{.schema.ID}}Client) AddHandler(ctx context.Context, name string, sync {{.schema.CodeName}}HandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *{{.schema.ID}}Client) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync {{.schema.CodeName}}HandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *{{.schema.ID}}Client) AddLifecycle(ctx context.Context, name string, lifecycle {{.schema.CodeName}}Lifecycle) {
	sync := New{{.schema.CodeName}}LifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *{{.schema.ID}}Client) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle {{.schema.CodeName}}Lifecycle) {
	sync := New{{.schema.CodeName}}LifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *{{.schema.ID}}Client) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync {{.schema.CodeName}}HandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *{{.schema.ID}}Client) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync {{.schema.CodeName}}HandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *{{.schema.ID}}Client) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle {{.schema.CodeName}}Lifecycle) {
	sync := New{{.schema.CodeName}}LifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *{{.schema.ID}}Client) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle {{.schema.CodeName}}Lifecycle) {
	sync := New{{.schema.CodeName}}LifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
`
