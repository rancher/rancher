package client

import (
	"fmt"
	"sync"

	"github.com/rancher/lasso/pkg/mapper"
	"github.com/rancher/lasso/pkg/scheme"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
)

type SharedClientFactoryOptions struct {
	Mapper meta.RESTMapper
	Scheme *runtime.Scheme
}

type SharedClientFactory interface {
	ForKind(gvk schema.GroupVersionKind) (*Client, error)
	ForResource(gvr schema.GroupVersionResource) (*Client, error)
	NewObjects(gvk schema.GroupVersionKind) (runtime.Object, runtime.Object, error)
	GVK(obj runtime.Object) (schema.GroupVersionKind, error)
}

type sharedClientFactory struct {
	createLock sync.RWMutex
	clients    map[schema.GroupVersionResource]*Client
	rest       rest.Interface

	Mapper meta.RESTMapper
	Scheme *runtime.Scheme
}

func NewSharedClientFactoryForConfig(config *rest.Config) (SharedClientFactory, error) {
	return NewSharedClientFactory(config, nil)
}

func NewSharedClientFactory(config *rest.Config, opts *SharedClientFactoryOptions) (_ SharedClientFactory, err error) {
	opts, err = applyDefaults(config, opts)
	if err != nil {
		return nil, err
	}

	rest, err := rest.UnversionedRESTClientFor(populateConfig(opts.Scheme, config))
	if err != nil {
		return nil, err
	}

	return &sharedClientFactory{
		clients: map[schema.GroupVersionResource]*Client{},
		Scheme:  opts.Scheme,
		Mapper:  opts.Mapper,
		rest:    rest,
	}, nil
}

func applyDefaults(config *rest.Config, opts *SharedClientFactoryOptions) (*SharedClientFactoryOptions, error) {
	var newOpts SharedClientFactoryOptions
	if opts != nil {
		newOpts = *opts
	}

	if newOpts.Scheme == nil {
		newOpts.Scheme = scheme.All
	}

	if newOpts.Mapper == nil {
		mapperOpt, err := mapper.New(config)
		if err != nil {
			return nil, err
		}
		newOpts.Mapper = mapperOpt
	}

	return &newOpts, nil
}

func (s *sharedClientFactory) GVK(obj runtime.Object) (schema.GroupVersionKind, error) {
	gvks, _, err := s.Scheme.ObjectKinds(obj)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}
	if len(gvks) == 0 {
		return schema.GroupVersionKind{}, fmt.Errorf("failed to find schema.GroupVersionKind for %T", obj)
	}
	return gvks[0], nil
}

func (s *sharedClientFactory) NewObjects(gvk schema.GroupVersionKind) (runtime.Object, runtime.Object, error) {
	obj, err := s.Scheme.New(gvk)
	if err != nil {
		return nil, nil, err
	}

	objList, err := s.Scheme.New(schema.GroupVersionKind{
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    gvk.Kind + "List",
	})
	return obj, objList, err
}

func (s *sharedClientFactory) ForKind(gvk schema.GroupVersionKind) (*Client, error) {
	mapping, err := s.Mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}

	return s.ForResource(mapping.Resource)
}

func (s *sharedClientFactory) ForResource(gvr schema.GroupVersionResource) (*Client, error) {
	client := s.getClient(gvr)
	if client != nil {
		return client, nil
	}

	s.createLock.Lock()
	defer s.createLock.Unlock()

	client = s.clients[gvr]
	if client != nil {
		return client, nil
	}

	client, err := NewClient(gvr, s.Mapper, s.rest)
	if err != nil {
		return nil, err
	}

	s.clients[gvr] = client
	return client, nil
}

func (s *sharedClientFactory) getClient(gvr schema.GroupVersionResource) *Client {
	s.createLock.RLock()
	defer s.createLock.RUnlock()
	return s.clients[gvr]
}

func populateConfig(scheme *runtime.Scheme, config *rest.Config) *rest.Config {
	config = rest.CopyConfig(config)
	config.NegotiatedSerializer = serializer.NewCodecFactory(scheme).WithoutConversion()
	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}
	return config
}
