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

package lister

import (
	"github.com/rancher/wrangler/pkg/summary"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

var _ Lister = &summaryLister{}
var _ NamespaceLister = &summaryNamespaceLister{}

// summaryLister implements the Lister interface.
type summaryLister struct {
	indexer cache.Indexer
	gvr     schema.GroupVersionResource
}

// New returns a new Lister.
func New(indexer cache.Indexer, gvr schema.GroupVersionResource) Lister {
	return &summaryLister{indexer: indexer, gvr: gvr}
}

// List lists all resources in the indexer.
func (l *summaryLister) List(selector labels.Selector) (ret []*summary.SummarizedObject, err error) {
	err = cache.ListAll(l.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*summary.SummarizedObject))
	})
	return ret, err
}

// Get retrieves a resource from the indexer with the given name
func (l *summaryLister) Get(name string) (*summary.SummarizedObject, error) {
	obj, exists, err := l.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(l.gvr.GroupResource(), name)
	}
	return obj.(*summary.SummarizedObject), nil
}

// Namespace returns an object that can list and get resources from a given namespace.
func (l *summaryLister) Namespace(namespace string) NamespaceLister {
	return &summaryNamespaceLister{indexer: l.indexer, namespace: namespace, gvr: l.gvr}
}

// summaryNamespaceLister implements the NamespaceLister interface.
type summaryNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
	gvr       schema.GroupVersionResource
}

// List lists all resources in the indexer for a given namespace.
func (l *summaryNamespaceLister) List(selector labels.Selector) (ret []*summary.SummarizedObject, err error) {
	err = cache.ListAllByNamespace(l.indexer, l.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*summary.SummarizedObject))
	})
	return ret, err
}

// Get retrieves a resource from the indexer for a given namespace and name.
func (l *summaryNamespaceLister) Get(name string) (*summary.SummarizedObject, error) {
	obj, exists, err := l.indexer.GetByKey(l.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(l.gvr.GroupResource(), name)
	}
	return obj.(*summary.SummarizedObject), nil
}
