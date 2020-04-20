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

package controller

import (
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/internal/controller/metrics"
	logf "sigs.k8s.io/controller-runtime/pkg/internal/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.RuntimeLog.WithName("controller")

var _ inject.Injector = &Controller{}

// Controller implements controller.Controller
type Controller struct {
	// Name is used to uniquely identify a Controller in tracing, logging and monitoring.  Name is required.
	Name string

	// MaxConcurrentReconciles is the maximum number of concurrent Reconciles which can be run. Defaults to 1.
	MaxConcurrentReconciles int

	// Reconciler is a function that can be called at any time with the Name / Namespace of an object and
	// ensures that the state of the system matches the state specified in the object.
	// Defaults to the DefaultReconcileFunc.
	Do reconcile.Reconciler

	// Client is a lazily initialized Client.  The controllerManager will initialize this when Start is called.
	Client client.Client

	// Scheme is injected by the controllerManager when controllerManager.Start is called
	Scheme *runtime.Scheme

	// informers are injected by the controllerManager when controllerManager.Start is called
	Cache cache.Cache

	// Config is the rest.Config used to talk to the apiserver.  Defaults to one of in-cluster, environment variable
	// specified, or the ~/.kube/Config.
	Config *rest.Config

	// MakeQueue constructs the queue for this controller once the controller is ready to start.
	// This exists because the standard Kubernetes workqueues start themselves immediately, which
	// leads to goroutine leaks if something calls controller.New repeatedly.
	MakeQueue func() workqueue.RateLimitingInterface

	// Queue is an listeningQueue that listens for events from Informers and adds object keys to
	// the Queue for processing
	Queue workqueue.RateLimitingInterface

	// SetFields is used to inject dependencies into other objects such as Sources, EventHandlers and Predicates
	SetFields func(i interface{}) error

	// mu is used to synchronize Controller setup
	mu sync.Mutex

	// JitterPeriod allows tests to reduce the JitterPeriod so they complete faster
	JitterPeriod time.Duration

	// WaitForCacheSync allows tests to mock out the WaitForCacheSync function to return an error
	// defaults to Cache.WaitForCacheSync
	WaitForCacheSync func(stopCh <-chan struct{}) bool

	// Started is true if the Controller has been Started
	Started bool

	// Recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	Recorder record.EventRecorder

	// TODO(community): Consider initializing a logger with the Controller Name as the tag

	// watches maintains a list of sources, handlers, and predicates to start when the controller is started.
	watches []watchDescription
}

// watchDescription contains all the information necessary to start a watch.
type watchDescription struct {
	src        source.Source
	handler    handler.EventHandler
	predicates []predicate.Predicate
}

// Reconcile implements reconcile.Reconciler
func (c *Controller) Reconcile(r reconcile.Request) (reconcile.Result, error) {
	return c.Do.Reconcile(r)
}

// Watch implements controller.Controller
func (c *Controller) Watch(src source.Source, evthdler handler.EventHandler, prct ...predicate.Predicate) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Inject Cache into arguments
	if err := c.SetFields(src); err != nil {
		return err
	}
	if err := c.SetFields(evthdler); err != nil {
		return err
	}
	for _, pr := range prct {
		if err := c.SetFields(pr); err != nil {
			return err
		}
	}

	c.watches = append(c.watches, watchDescription{src: src, handler: evthdler, predicates: prct})
	if c.Started {
		log.Info("Starting EventSource", "controller", c.Name, "source", src)
		return src.Start(evthdler, c.Queue, prct...)
	}

	return nil
}

// Start implements controller.Controller
func (c *Controller) Start(stop <-chan struct{}) error {
	// use an IIFE to get proper lock handling
	// but lock outside to get proper handling of the queue shutdown
	c.mu.Lock()

	c.Queue = c.MakeQueue()
	defer c.Queue.ShutDown() // needs to be outside the iife so that we shutdown after the stop channel is closed

	err := func() error {
		defer c.mu.Unlock()

		// TODO(pwittrock): Reconsider HandleCrash
		defer utilruntime.HandleCrash()

		// NB(directxman12): launch the sources *before* trying to wait for the
		// caches to sync so that they have a chance to register their intendeded
		// caches.
		for _, watch := range c.watches {
			log.Info("Starting EventSource", "controller", c.Name, "source", watch.src)
			if err := watch.src.Start(watch.handler, c.Queue, watch.predicates...); err != nil {
				return err
			}
		}

		// Start the SharedIndexInformer factories to begin populating the SharedIndexInformer caches
		log.Info("Starting Controller", "controller", c.Name)

		// Wait for the caches to be synced before starting workers
		if c.WaitForCacheSync == nil {
			c.WaitForCacheSync = c.Cache.WaitForCacheSync
		}
		if ok := c.WaitForCacheSync(stop); !ok {
			// This code is unreachable right now since WaitForCacheSync will never return an error
			// Leaving it here because that could happen in the future
			err := fmt.Errorf("failed to wait for %s caches to sync", c.Name)
			log.Error(err, "Could not wait for Cache to sync", "controller", c.Name)
			return err
		}

		if c.JitterPeriod == 0 {
			c.JitterPeriod = 1 * time.Second
		}

		// Launch workers to process resources
		log.Info("Starting workers", "controller", c.Name, "worker count", c.MaxConcurrentReconciles)
		for i := 0; i < c.MaxConcurrentReconciles; i++ {
			// Process work items
			go wait.Until(c.worker, c.JitterPeriod, stop)
		}

		c.Started = true
		return nil
	}()
	if err != nil {
		return err
	}

	<-stop
	log.Info("Stopping workers", "controller", c.Name)
	return nil
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the reconcileHandler is never invoked concurrently with the same object.
func (c *Controller) worker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the reconcileHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.Queue.Get()
	if shutdown {
		// Stop working
		return false
	}

	// We call Done here so the workqueue knows we have finished
	// processing this item. We also must remember to call Forget if we
	// do not want this work item being re-queued. For example, we do
	// not call Forget if a transient error occurs, instead the item is
	// put back on the workqueue and attempted again after a back-off
	// period.
	defer c.Queue.Done(obj)

	return c.reconcileHandler(obj)
}

func (c *Controller) reconcileHandler(obj interface{}) bool {
	// Update metrics after processing each item
	reconcileStartTS := time.Now()
	defer func() {
		c.updateMetrics(time.Since(reconcileStartTS))
	}()

	var req reconcile.Request
	var ok bool
	if req, ok = obj.(reconcile.Request); !ok {
		// As the item in the workqueue is actually invalid, we call
		// Forget here else we'd go into a loop of attempting to
		// process a work item that is invalid.
		c.Queue.Forget(obj)
		log.Error(nil, "Queue item was not a Request",
			"controller", c.Name, "type", fmt.Sprintf("%T", obj), "value", obj)
		// Return true, don't take a break
		return true
	}
	// RunInformersAndControllers the syncHandler, passing it the namespace/Name string of the
	// resource to be synced.
	if result, err := c.Do.Reconcile(req); err != nil {
		c.Queue.AddRateLimited(req)
		log.Error(err, "Reconciler error", "controller", c.Name, "request", req)
		ctrlmetrics.ReconcileErrors.WithLabelValues(c.Name).Inc()
		ctrlmetrics.ReconcileTotal.WithLabelValues(c.Name, "error").Inc()
		return false
	} else if result.RequeueAfter > 0 {
		// The result.RequeueAfter request will be lost, if it is returned
		// along with a non-nil error. But this is intended as
		// We need to drive to stable reconcile loops before queuing due
		// to result.RequestAfter
		c.Queue.Forget(obj)
		c.Queue.AddAfter(req, result.RequeueAfter)
		ctrlmetrics.ReconcileTotal.WithLabelValues(c.Name, "requeue_after").Inc()
		return true
	} else if result.Requeue {
		c.Queue.AddRateLimited(req)
		ctrlmetrics.ReconcileTotal.WithLabelValues(c.Name, "requeue").Inc()
		return true
	}

	// Finally, if no error occurs we Forget this item so it does not
	// get queued again until another change happens.
	c.Queue.Forget(obj)

	// TODO(directxman12): What does 1 mean?  Do we want level constants?  Do we want levels at all?
	log.V(1).Info("Successfully Reconciled", "controller", c.Name, "request", req)

	ctrlmetrics.ReconcileTotal.WithLabelValues(c.Name, "success").Inc()
	// Return true, don't take a break
	return true
}

// InjectFunc implement SetFields.Injector
func (c *Controller) InjectFunc(f inject.Func) error {
	c.SetFields = f
	return nil
}

// updateMetrics updates prometheus metrics within the controller
func (c *Controller) updateMetrics(reconcileTime time.Duration) {
	ctrlmetrics.ReconcileTime.WithLabelValues(c.Name).Observe(reconcileTime.Seconds())
}
