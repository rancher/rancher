/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// Options are creation options for a Client
type Options struct {
	// Scheme, if provided, will be used to map go structs to GroupVersionKinds
	Scheme *runtime.Scheme

	// Mapper, if provided, will be used to map GroupVersionKinds to Resources
	Mapper meta.RESTMapper
}

// New returns a new Client using the provided config and Options.
// The returned client reads *and* writes directly from the server
// (it doesn't use object caches).  It understands how to work with
// normal types (both custom resources and aggregated/built-in resources),
// as well as unstructured types.
//
// In the case of normal types, the scheme will be used to look up the
// corresponding group, version, and kind for the given type.  In the
// case of unstructured types, the group, version, and kind will be extracted
// from the corresponding fields on the object.
func New(config *rest.Config, options Options) (Client, error) {
	if config == nil {
		return nil, fmt.Errorf("must provide non-nil rest.Config to client.New")
	}

	// Init a scheme if none provided
	if options.Scheme == nil {
		options.Scheme = scheme.Scheme
	}

	// Init a Mapper if none provided
	if options.Mapper == nil {
		var err error
		options.Mapper, err = apiutil.NewDynamicRESTMapper(config)
		if err != nil {
			return nil, err
		}
	}

	clientcache := &clientCache{
		config:         config,
		scheme:         options.Scheme,
		mapper:         options.Mapper,
		codecs:         serializer.NewCodecFactory(options.Scheme),
		resourceByType: make(map[schema.GroupVersionKind]*resourceMeta),
	}

	c := &client{
		typedClient: typedClient{
			cache:      clientcache,
			paramCodec: runtime.NewParameterCodec(options.Scheme),
		},
		unstructuredClient: unstructuredClient{
			cache:      clientcache,
			paramCodec: noConversionParamCodec{},
		},
	}

	return c, nil
}

var _ Client = &client{}

// client is a client.Client that reads and writes directly from/to an API server.  It lazily initializes
// new clients at the time they are used, and caches the client.
type client struct {
	typedClient        typedClient
	unstructuredClient unstructuredClient
}

// resetGroupVersionKind is a helper function to restore and preserve GroupVersionKind on an object.
// TODO(vincepri): Remove this function and its calls once controller-runtime dependencies are upgraded to 1.16?
func (c *client) resetGroupVersionKind(obj runtime.Object, gvk schema.GroupVersionKind) {
	if gvk != schema.EmptyObjectKind.GroupVersionKind() {
		if v, ok := obj.(schema.ObjectKind); ok {
			v.SetGroupVersionKind(gvk)
		}
	}
}

// Create implements client.Client
func (c *client) Create(ctx context.Context, obj runtime.Object, opts ...CreateOption) error {
	_, ok := obj.(*unstructured.Unstructured)
	if ok {
		return c.unstructuredClient.Create(ctx, obj, opts...)
	}
	return c.typedClient.Create(ctx, obj, opts...)
}

// Update implements client.Client
func (c *client) Update(ctx context.Context, obj runtime.Object, opts ...UpdateOption) error {
	defer c.resetGroupVersionKind(obj, obj.GetObjectKind().GroupVersionKind())
	_, ok := obj.(*unstructured.Unstructured)
	if ok {
		return c.unstructuredClient.Update(ctx, obj, opts...)
	}
	return c.typedClient.Update(ctx, obj, opts...)
}

// Delete implements client.Client
func (c *client) Delete(ctx context.Context, obj runtime.Object, opts ...DeleteOption) error {
	_, ok := obj.(*unstructured.Unstructured)
	if ok {
		return c.unstructuredClient.Delete(ctx, obj, opts...)
	}
	return c.typedClient.Delete(ctx, obj, opts...)
}

// DeleteAllOf implements client.Client
func (c *client) DeleteAllOf(ctx context.Context, obj runtime.Object, opts ...DeleteAllOfOption) error {
	_, ok := obj.(*unstructured.Unstructured)
	if ok {
		return c.unstructuredClient.DeleteAllOf(ctx, obj, opts...)
	}
	return c.typedClient.DeleteAllOf(ctx, obj, opts...)
}

// Patch implements client.Client
func (c *client) Patch(ctx context.Context, obj runtime.Object, patch Patch, opts ...PatchOption) error {
	defer c.resetGroupVersionKind(obj, obj.GetObjectKind().GroupVersionKind())
	_, ok := obj.(*unstructured.Unstructured)
	if ok {
		return c.unstructuredClient.Patch(ctx, obj, patch, opts...)
	}
	return c.typedClient.Patch(ctx, obj, patch, opts...)
}

// Get implements client.Client
func (c *client) Get(ctx context.Context, key ObjectKey, obj runtime.Object) error {
	_, ok := obj.(*unstructured.Unstructured)
	if ok {
		return c.unstructuredClient.Get(ctx, key, obj)
	}
	return c.typedClient.Get(ctx, key, obj)
}

// List implements client.Client
func (c *client) List(ctx context.Context, obj runtime.Object, opts ...ListOption) error {
	_, ok := obj.(*unstructured.UnstructuredList)
	if ok {
		return c.unstructuredClient.List(ctx, obj, opts...)
	}
	return c.typedClient.List(ctx, obj, opts...)
}

// Status implements client.StatusClient
func (c *client) Status() StatusWriter {
	return &statusWriter{client: c}
}

// statusWriter is client.StatusWriter that writes status subresource
type statusWriter struct {
	client *client
}

// ensure statusWriter implements client.StatusWriter
var _ StatusWriter = &statusWriter{}

// Update implements client.StatusWriter
func (sw *statusWriter) Update(ctx context.Context, obj runtime.Object, opts ...UpdateOption) error {
	defer sw.client.resetGroupVersionKind(obj, obj.GetObjectKind().GroupVersionKind())
	_, ok := obj.(*unstructured.Unstructured)
	if ok {
		return sw.client.unstructuredClient.UpdateStatus(ctx, obj, opts...)
	}
	return sw.client.typedClient.UpdateStatus(ctx, obj, opts...)
}

// Patch implements client.Client
func (sw *statusWriter) Patch(ctx context.Context, obj runtime.Object, patch Patch, opts ...PatchOption) error {
	defer sw.client.resetGroupVersionKind(obj, obj.GetObjectKind().GroupVersionKind())
	_, ok := obj.(*unstructured.Unstructured)
	if ok {
		return sw.client.unstructuredClient.PatchStatus(ctx, obj, patch, opts...)
	}
	return sw.client.typedClient.PatchStatus(ctx, obj, patch, opts...)
}
