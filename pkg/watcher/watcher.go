package watcher

import (
	"context"
	"time"

	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	defaultRetryPeriod = 5 * time.Second
)

type watchOptions struct {
	handlerName   string
	namespace     string
	fieldSelector fields.Selector
	retryPeriod   time.Duration
	watchTimeout  *int64
}

func (o watchOptions) getLogger() logrus.FieldLogger {
	if o.handlerName != "" {
		return logrus.WithField("handler", o.handlerName)
	}
	return logrus.StandardLogger()
}

func loadWatchOptions(opts ...WatchOption) watchOptions {
	options := watchOptions{
		fieldSelector: fields.Everything(),
		retryPeriod:   defaultRetryPeriod,
	}
	for _, o := range opts {
		o(&options)
	}
	return options
}

// WatchOption allows configuring how watches are performed
type WatchOption func(*watchOptions)

// WithHandlerName configures the handler name, used for logging
func WithHandlerName(name string) WatchOption {
	return func(o *watchOptions) {
		o.handlerName = name
	}
}

// WithNamespace allows watching resources in a single namespace
func WithNamespace(namespace string) WatchOption {
	return func(o *watchOptions) {
		o.namespace = namespace
	}
}

// WithFieldSelector will configure a field selector during Watch. E.g. setting "metadata.name=some-name" allows watching a specific resource
func WithFieldSelector(m map[string]string) WatchOption {
	return func(o *watchOptions) {
		o.fieldSelector = fields.SelectorFromSet(m)
	}
}

// WithRetryPeriod will set a retry period between failed watch attempts
func WithRetryPeriod(retryPeriod time.Duration) WatchOption {
	return func(o *watchOptions) {
		o.retryPeriod = retryPeriod
	}
}

// WithWatchTimeout will set a timeout for the every Watch operation
func WithWatchTimeout(watchTimeout time.Duration) WatchOption {
	return func(o *watchOptions) {
		if watchTimeout >= time.Second {
			seconds := int64(watchTimeout.Seconds())
			o.watchTimeout = &seconds
		}
	}
}

type watchCallback[T generic.RuntimeMetaObject] func(T, watch.EventType)

// OnChange configures a handler to be executed based on a dedicated watcher, not involving informers or existing caches
// Despite accepting a generic.ObjectHandler, and unlike wrangler/lasso controllers, error-handling must be implemented on the handler directly
func OnChange[T generic.RuntimeMetaObject, TList runtime.Object](client generic.ClientInterface[T, TList], ctx context.Context, name string, handler generic.ObjectHandler[T], opts ...WatchOption) {
	opts = append(opts, WithHandlerName(name))
	options := loadWatchOptions(opts...)
	logger := options.getLogger()
	callback := func(obj T, _ watch.EventType) {
		key := objKey(obj)
		if _, err := handler(key, obj); err != nil {
			logger.Errorf("couldn't process %q object: %v", key, err)
		}
	}
	go resumableWatch(ctx, client, callback, opts...)
}

// resumableWatch creates a watcher from client based on the provided watchOptions, then execute the callback with every event received. It is context-aware and will resume watching in case it is interrupted before the context is canceled
func resumableWatch[T generic.RuntimeMetaObject, TList runtime.Object](ctx context.Context, client generic.ClientInterface[T, TList], callback watchCallback[T], opts ...WatchOption) {
	options := loadWatchOptions(opts...)
	logger := options.getLogger()

	var watcher watch.Interface
	var lastSeen string
	for {
		if watcher == nil {
			var err error
			watcher, err = client.Watch(options.namespace, metav1.ListOptions{
				FieldSelector:   options.fieldSelector.String(),
				TimeoutSeconds:  options.watchTimeout,
				ResourceVersion: lastSeen, // if empty, it will produce an "Added" event with the initial state of the resources
			})
			if err != nil {
				logger.WithError(err).Warnf("creating watch")
				time.Sleep(options.retryPeriod)
				continue
			}
		}

		select {
		case <-ctx.Done():
			watcher.Stop()
			return
		case event, ok := <-watcher.ResultChan():
			// Special case for watcher closed by the producer, e.g., due to an error or timeout
			if !ok || event.Type == watch.Error {
				watcher.Stop()
				watcher = nil
				continue
			}

			switch event.Type {
			case watch.Added, watch.Modified, watch.Deleted:
				obj, ok := event.Object.(T)
				if !ok {
					logger.Warnf("unexpected type %T, skipping...", event.Object)
					continue
				}
				lastSeen = obj.GetResourceVersion()

				callback(obj, event.Type)
			}
		}
	}
}

func objKey(obj metav1.Object) string {
	if obj == nil {
		return ""
	}
	ns, name := obj.GetNamespace(), obj.GetName()
	if ns == "" {
		return name
	}
	return ns + "/" + name
}
