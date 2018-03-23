package controller

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/juju/ratelimit"
	errors2 "github.com/pkg/errors"
	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/types"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

var (
	resyncPeriod = 2 * time.Hour
)

type HandlerFunc func(key string) error

type GenericController interface {
	Informer() cache.SharedIndexInformer
	AddHandler(name string, handler HandlerFunc)
	HandlerCount() int
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type Backend interface {
	List(opts metav1.ListOptions) (runtime.Object, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	ObjectFactory() clientbase.ObjectFactory
}

type handlerDef struct {
	name    string
	handler HandlerFunc
}

type genericController struct {
	sync.Mutex
	informer cache.SharedIndexInformer
	handlers []handlerDef
	queue    workqueue.RateLimitingInterface
	name     string
	running  bool
	synced   bool
}

func NewGenericController(name string, genericClient Backend) GenericController {
	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc:  genericClient.List,
			WatchFunc: genericClient.Watch,
		},
		genericClient.ObjectFactory().Object(), resyncPeriod, cache.Indexers{})

	rl := workqueue.NewMaxOfRateLimiter(
		workqueue.NewItemExponentialFailureRateLimiter(500*time.Millisecond, 1000*time.Second),
		// 10 qps, 100 bucket size.  This is only for retry speed and its only the overall factor (not per item)
		&workqueue.BucketRateLimiter{Bucket: ratelimit.NewBucketWithRate(float64(10), int64(100))},
	)

	return &genericController{
		informer: informer,
		queue:    workqueue.NewNamedRateLimitingQueue(rl, name),
		name:     name,
	}
}

func (g *genericController) HandlerCount() int {
	return len(g.handlers)
}

func (g *genericController) Informer() cache.SharedIndexInformer {
	return g.informer
}

func (g *genericController) Enqueue(namespace, name string) {
	if namespace == "" {
		g.queue.Add(name)
	} else {
		g.queue.Add(namespace + "/" + name)
	}
}

func (g *genericController) AddHandler(name string, handler HandlerFunc) {
	g.handlers = append(g.handlers, handlerDef{
		name:    name,
		handler: handler,
	})
}

func (g *genericController) Sync(ctx context.Context) error {
	g.Lock()
	defer g.Unlock()

	return g.sync(ctx)
}

func (g *genericController) sync(ctx context.Context) error {
	if g.synced {
		return nil
	}

	defer utilruntime.HandleCrash()

	g.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: g.queueObject,
		UpdateFunc: func(_, obj interface{}) {
			g.queueObject(obj)
		},
		DeleteFunc: g.queueObject,
	})

	logrus.Infof("Syncing %s Controller", g.name)

	go g.informer.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), g.informer.HasSynced) {
		return fmt.Errorf("failed to sync controller %s", g.name)
	}
	logrus.Infof("Syncing %s Controller Done", g.name)

	g.synced = true
	return nil
}

func (g *genericController) Start(ctx context.Context, threadiness int) error {
	g.Lock()
	defer g.Unlock()

	if !g.synced {
		if err := g.sync(ctx); err != nil {
			return err
		}
	}

	if !g.running {
		go g.run(ctx, threadiness)
	}

	g.running = true
	return nil
}

func (g *genericController) queueObject(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err == nil {
		g.queue.Add(key)
	}
}

func (g *genericController) run(ctx context.Context, threadiness int) {
	defer utilruntime.HandleCrash()
	defer g.queue.ShutDown()

	for i := 0; i < threadiness; i++ {
		go wait.Until(g.runWorker, time.Second, ctx.Done())
	}

	<-ctx.Done()
	logrus.Infof("Shutting down %s controller", g.name)
}

func (g *genericController) runWorker() {
	for g.processNextWorkItem() {
	}
}

func (g *genericController) processNextWorkItem() bool {
	key, quit := g.queue.Get()
	if quit {
		return false
	}
	defer g.queue.Done(key)

	// do your work on the key.  This method will contains your "do stuff" logic
	err := g.syncHandler(key.(string))
	checkErr := err
	if handlerErr, ok := checkErr.(*handlerError); ok {
		checkErr = handlerErr.err
	}
	if _, ok := checkErr.(*ForgetError); err == nil || ok {
		if ok {
			logrus.Infof("%v %v completed with dropped err: %v", g.name, key, err)
		}
		g.queue.Forget(key)
		return true
	}

	if err := filterConflictsError(err); err != nil {
		utilruntime.HandleError(fmt.Errorf("%v %v %v", g.name, key, err))
	}

	g.queue.AddRateLimited(key)

	return true
}

func ignoreError(err error, checkString bool) bool {
	err = errors2.Cause(err)
	if errors.IsConflict(err) {
		return true
	}
	if _, ok := err.(*ForgetError); ok {
		return true
	}
	if checkString {
		return strings.HasSuffix(err.Error(), "please apply your changes to the latest version and try again")
	}
	return false
}

func filterConflictsError(err error) error {
	if ignoreError(err, false) {
		return nil
	}

	if errs, ok := errors2.Cause(err).(*types.MultiErrors); ok {
		var newErrors []error
		for _, err := range errs.Errors {
			if !ignoreError(err, true) {
				newErrors = append(newErrors)
			}
		}
		return types.NewErrors(newErrors...)
	}

	return err
}

func (g *genericController) syncHandler(s string) (err error) {
	defer utilruntime.RecoverFromPanic(&err)

	var errs []error
	for _, handler := range g.handlers {
		logrus.Debugf("%s calling handler %s %s", g.name, handler.name, s)
		if err := handler.handler(s); err != nil {
			errs = append(errs, &handlerError{
				name: handler.name,
				err:  err,
			})
		}
	}
	err = types.NewErrors(errs...)
	return
}

type handlerError struct {
	name string
	err  error
}

func (h *handlerError) Error() string {
	return fmt.Sprintf("[%s] failed with : %v", h.name, h.err)
}

func (h *handlerError) Cause() error {
	return h.err
}
