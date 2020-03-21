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

package informer

import (
	"sync"
	"time"

	"github.com/rancher/wrangler/pkg/summary"
	"github.com/rancher/wrangler/pkg/summary/client"
	"github.com/rancher/wrangler/pkg/summary/lister"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

// NewSummarySharedInformerFactory constructs a new instance of summarySharedInformerFactory for all namespaces.
func NewSummarySharedInformerFactory(client client.Interface, defaultResync time.Duration) SummarySharedInformerFactory {
	return NewFilteredSummarySharedInformerFactory(client, defaultResync, metav1.NamespaceAll, nil)
}

// NewFilteredSummarySharedInformerFactory constructs a new instance of summarySharedInformerFactory.
// Listers obtained via this factory will be subject to the same filters as specified here.
func NewFilteredSummarySharedInformerFactory(client client.Interface, defaultResync time.Duration, namespace string, tweakListOptions TweakListOptionsFunc) SummarySharedInformerFactory {
	return &summarySharedInformerFactory{
		client:           client,
		defaultResync:    defaultResync,
		namespace:        namespace,
		informers:        map[schema.GroupVersionResource]informers.GenericInformer{},
		startedInformers: make(map[schema.GroupVersionResource]bool),
		tweakListOptions: tweakListOptions,
	}
}

type summarySharedInformerFactory struct {
	client        client.Interface
	defaultResync time.Duration
	namespace     string

	lock      sync.Mutex
	informers map[schema.GroupVersionResource]informers.GenericInformer
	// startedInformers is used for tracking which informers have been started.
	// This allows Start() to be called multiple times safely.
	startedInformers map[schema.GroupVersionResource]bool
	tweakListOptions TweakListOptionsFunc
}

var _ SummarySharedInformerFactory = &summarySharedInformerFactory{}

func (f *summarySharedInformerFactory) ForResource(gvr schema.GroupVersionResource) informers.GenericInformer {
	f.lock.Lock()
	defer f.lock.Unlock()

	key := gvr
	informer, exists := f.informers[key]
	if exists {
		return informer
	}

	informer = NewFilteredSummaryInformer(f.client, gvr, f.namespace, f.defaultResync, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
	f.informers[key] = informer

	return informer
}

// Start initializes all requested informers.
func (f *summarySharedInformerFactory) Start(stopCh <-chan struct{}) {
	f.lock.Lock()
	defer f.lock.Unlock()

	for informerType, informer := range f.informers {
		if !f.startedInformers[informerType] {
			go informer.Informer().Run(stopCh)
			f.startedInformers[informerType] = true
		}
	}
}

// WaitForCacheSync waits for all started informers' cache were synced.
func (f *summarySharedInformerFactory) WaitForCacheSync(stopCh <-chan struct{}) map[schema.GroupVersionResource]bool {
	informers := func() map[schema.GroupVersionResource]cache.SharedIndexInformer {
		f.lock.Lock()
		defer f.lock.Unlock()

		informers := map[schema.GroupVersionResource]cache.SharedIndexInformer{}
		for informerType, informer := range f.informers {
			if f.startedInformers[informerType] {
				informers[informerType] = informer.Informer()
			}
		}
		return informers
	}()

	res := map[schema.GroupVersionResource]bool{}
	for informType, informer := range informers {
		res[informType] = cache.WaitForCacheSync(stopCh, informer.HasSynced)
	}
	return res
}

// NewFilteredSummaryInformer constructs a new informer for a summary type.
func NewFilteredSummaryInformer(client client.Interface, gvr schema.GroupVersionResource, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions TweakListOptionsFunc) informers.GenericInformer {
	return &summaryInformer{
		gvr: gvr,
		informer: cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					if tweakListOptions != nil {
						tweakListOptions(&options)
					}
					return client.Resource(gvr).Namespace(namespace).List(options)
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					if tweakListOptions != nil {
						tweakListOptions(&options)
					}
					return client.Resource(gvr).Namespace(namespace).Watch(options)
				},
			},
			&summary.SummarizedObject{},
			resyncPeriod,
			indexers,
		),
	}
}

type summaryInformer struct {
	informer cache.SharedIndexInformer
	gvr      schema.GroupVersionResource
}

var _ informers.GenericInformer = &summaryInformer{}

func (d *summaryInformer) Informer() cache.SharedIndexInformer {
	return d.informer
}

func (d *summaryInformer) Lister() cache.GenericLister {
	return lister.NewRuntimeObjectShim(lister.New(d.informer.GetIndexer(), d.gvr))
}
