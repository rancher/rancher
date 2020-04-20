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

package manager

import (
	"fmt"
	"net"
	"time"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	internalrecorder "sigs.k8s.io/controller-runtime/pkg/internal/recorder"
	"sigs.k8s.io/controller-runtime/pkg/leaderelection"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/recorder"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// Manager initializes shared dependencies such as Caches and Clients, and provides them to Runnables.
// A Manager is required to create Controllers.
type Manager interface {
	// Add will set requested dependencies on the component, and cause the component to be
	// started when Start is called.  Add will inject any dependencies for which the argument
	// implements the inject interface - e.g. inject.Client.
	// Depending on if a Runnable implements LeaderElectionRunnable interface, a Runnable can be run in either
	// non-leaderelection mode (always running) or leader election mode (managed by leader election if enabled).
	Add(Runnable) error

	// SetFields will set any dependencies on an object for which the object has implemented the inject
	// interface - e.g. inject.Client.
	SetFields(interface{}) error

	// AddHealthzCheck allows you to add Healthz checker
	AddHealthzCheck(name string, check healthz.Checker) error

	// AddReadyzCheck allows you to add Readyz checker
	AddReadyzCheck(name string, check healthz.Checker) error

	// Start starts all registered Controllers and blocks until the Stop channel is closed.
	// Returns an error if there is an error starting any controller.
	Start(<-chan struct{}) error

	// GetConfig returns an initialized Config
	GetConfig() *rest.Config

	// GetScheme returns an initialized Scheme
	GetScheme() *runtime.Scheme

	// GetClient returns a client configured with the Config. This client may
	// not be a fully "direct" client -- it may read from a cache, for
	// instance.  See Options.NewClient for more information on how the default
	// implementation works.
	GetClient() client.Client

	// GetFieldIndexer returns a client.FieldIndexer configured with the client
	GetFieldIndexer() client.FieldIndexer

	// GetCache returns a cache.Cache
	GetCache() cache.Cache

	// GetEventRecorderFor returns a new EventRecorder for the provided name
	GetEventRecorderFor(name string) record.EventRecorder

	// GetRESTMapper returns a RESTMapper
	GetRESTMapper() meta.RESTMapper

	// GetAPIReader returns a reader that will be configured to use the API server.
	// This should be used sparingly and only when the client does not fit your
	// use case.
	GetAPIReader() client.Reader

	// GetWebhookServer returns a webhook.Server
	GetWebhookServer() *webhook.Server
}

// Options are the arguments for creating a new Manager
type Options struct {
	// Scheme is the scheme used to resolve runtime.Objects to GroupVersionKinds / Resources
	// Defaults to the kubernetes/client-go scheme.Scheme, but it's almost always better
	// idea to pass your own scheme in.  See the documentation in pkg/scheme for more information.
	Scheme *runtime.Scheme

	// MapperProvider provides the rest mapper used to map go types to Kubernetes APIs
	MapperProvider func(c *rest.Config) (meta.RESTMapper, error)

	// SyncPeriod determines the minimum frequency at which watched resources are
	// reconciled. A lower period will correct entropy more quickly, but reduce
	// responsiveness to change if there are many watched resources. Change this
	// value only if you know what you are doing. Defaults to 10 hours if unset.
	// there will a 10 percent jitter between the SyncPeriod of all controllers
	// so that all controllers will not send list requests simultaneously.
	SyncPeriod *time.Duration

	// LeaderElection determines whether or not to use leader election when
	// starting the manager.
	LeaderElection bool

	// LeaderElectionNamespace determines the namespace in which the leader
	// election configmap will be created.
	LeaderElectionNamespace string

	// LeaderElectionID determines the name of the configmap that leader election
	// will use for holding the leader lock.
	LeaderElectionID string

	// LeaseDuration is the duration that non-leader candidates will
	// wait to force acquire leadership. This is measured against time of
	// last observed ack. Default is 15 seconds.
	LeaseDuration *time.Duration
	// RenewDeadline is the duration that the acting master will retry
	// refreshing leadership before giving up. Default is 10 seconds.
	RenewDeadline *time.Duration
	// RetryPeriod is the duration the LeaderElector clients should wait
	// between tries of actions. Default is 2 seconds.
	RetryPeriod *time.Duration

	// Namespace if specified restricts the manager's cache to watch objects in
	// the desired namespace Defaults to all namespaces
	//
	// Note: If a namespace is specified, controllers can still Watch for a
	// cluster-scoped resource (e.g Node).  For namespaced resources the cache
	// will only hold objects from the desired namespace.
	Namespace string

	// MetricsBindAddress is the TCP address that the controller should bind to
	// for serving prometheus metrics.
	// It can be set to "0" to disable the metrics serving.
	MetricsBindAddress string

	// HealthProbeBindAddress is the TCP address that the controller should bind to
	// for serving health probes
	HealthProbeBindAddress string

	// Readiness probe endpoint name, defaults to "readyz"
	ReadinessEndpointName string

	// Liveness probe endpoint name, defaults to "healthz"
	LivenessEndpointName string

	// Port is the port that the webhook server serves at.
	// It is used to set webhook.Server.Port.
	Port int
	// Host is the hostname that the webhook server binds to.
	// It is used to set webhook.Server.Host.
	Host string

	// CertDir is the directory that contains the server key and certificate.
	// if not set, webhook server would look up the server key and certificate in
	// {TempDir}/k8s-webhook-server/serving-certs. The server key and certificate
	// must be named tls.key and tls.crt, respectively.
	CertDir string
	// Functions to all for a user to customize the values that will be injected.

	// NewCache is the function that will create the cache to be used
	// by the manager. If not set this will use the default new cache function.
	NewCache cache.NewCacheFunc

	// NewClient will create the client to be used by the manager.
	// If not set this will create the default DelegatingClient that will
	// use the cache for reads and the client for writes.
	NewClient NewClientFunc

	// EventBroadcaster records Events emitted by the manager and sends them to the Kubernetes API
	// Use this to customize the event correlator and spam filter
	EventBroadcaster record.EventBroadcaster

	// Dependency injection for testing
	newRecorderProvider    func(config *rest.Config, scheme *runtime.Scheme, logger logr.Logger, broadcaster record.EventBroadcaster) (recorder.Provider, error)
	newResourceLock        func(config *rest.Config, recorderProvider recorder.Provider, options leaderelection.Options) (resourcelock.Interface, error)
	newMetricsListener     func(addr string) (net.Listener, error)
	newHealthProbeListener func(addr string) (net.Listener, error)
}

// NewClientFunc allows a user to define how to create a client
type NewClientFunc func(cache cache.Cache, config *rest.Config, options client.Options) (client.Client, error)

// Runnable allows a component to be started.
// It's very important that Start blocks until
// it's done running.
type Runnable interface {
	// Start starts running the component.  The component will stop running
	// when the channel is closed.  Start blocks until the channel is closed or
	// an error occurs.
	Start(<-chan struct{}) error
}

// RunnableFunc implements Runnable using a function.
// It's very important that the given function block
// until it's done running.
type RunnableFunc func(<-chan struct{}) error

// Start implements Runnable
func (r RunnableFunc) Start(s <-chan struct{}) error {
	return r(s)
}

// LeaderElectionRunnable knows if a Runnable needs to be run in the leader election mode.
type LeaderElectionRunnable interface {
	// NeedLeaderElection returns true if the Runnable needs to be run in the leader election mode.
	// e.g. controllers need to be run in leader election mode, while webhook server doesn't.
	NeedLeaderElection() bool
}

// New returns a new Manager for creating Controllers.
func New(config *rest.Config, options Options) (Manager, error) {
	// Initialize a rest.config if none was specified
	if config == nil {
		return nil, fmt.Errorf("must specify Config")
	}

	// Set default values for options fields
	options = setOptionsDefaults(options)

	// Create the mapper provider
	mapper, err := options.MapperProvider(config)
	if err != nil {
		log.Error(err, "Failed to get API Group-Resources")
		return nil, err
	}

	// Create the cache for the cached read client and registering informers
	cache, err := options.NewCache(config, cache.Options{Scheme: options.Scheme, Mapper: mapper, Resync: options.SyncPeriod, Namespace: options.Namespace})
	if err != nil {
		return nil, err
	}

	apiReader, err := client.New(config, client.Options{Scheme: options.Scheme, Mapper: mapper})
	if err != nil {
		return nil, err
	}

	writeObj, err := options.NewClient(cache, config, client.Options{Scheme: options.Scheme, Mapper: mapper})
	if err != nil {
		return nil, err
	}
	// Create the recorder provider to inject event recorders for the components.
	// TODO(directxman12): the log for the event provider should have a context (name, tags, etc) specific
	// to the particular controller that it's being injected into, rather than a generic one like is here.
	recorderProvider, err := options.newRecorderProvider(config, options.Scheme, log.WithName("events"), options.EventBroadcaster)
	if err != nil {
		return nil, err
	}

	// Create the resource lock to enable leader election)
	resourceLock, err := options.newResourceLock(config, recorderProvider, leaderelection.Options{
		LeaderElection:          options.LeaderElection,
		LeaderElectionID:        options.LeaderElectionID,
		LeaderElectionNamespace: options.LeaderElectionNamespace,
	})
	if err != nil {
		return nil, err
	}

	// Create the metrics listener. This will throw an error if the metrics bind
	// address is invalid or already in use.
	metricsListener, err := options.newMetricsListener(options.MetricsBindAddress)
	if err != nil {
		return nil, err
	}

	// Create health probes listener. This will throw an error if the bind
	// address is invalid or already in use.
	healthProbeListener, err := options.newHealthProbeListener(options.HealthProbeBindAddress)
	if err != nil {
		return nil, err
	}

	stop := make(chan struct{})

	return &controllerManager{
		config:                config,
		scheme:                options.Scheme,
		cache:                 cache,
		fieldIndexes:          cache,
		client:                writeObj,
		apiReader:             apiReader,
		recorderProvider:      recorderProvider,
		resourceLock:          resourceLock,
		mapper:                mapper,
		metricsListener:       metricsListener,
		internalStop:          stop,
		internalStopper:       stop,
		port:                  options.Port,
		host:                  options.Host,
		certDir:               options.CertDir,
		leaseDuration:         *options.LeaseDuration,
		renewDeadline:         *options.RenewDeadline,
		retryPeriod:           *options.RetryPeriod,
		healthProbeListener:   healthProbeListener,
		readinessEndpointName: options.ReadinessEndpointName,
		livenessEndpointName:  options.LivenessEndpointName,
	}, nil
}

// defaultNewClient creates the default caching client
func defaultNewClient(cache cache.Cache, config *rest.Config, options client.Options) (client.Client, error) {
	// Create the Client for Write operations.
	c, err := client.New(config, options)
	if err != nil {
		return nil, err
	}

	return &client.DelegatingClient{
		Reader: &client.DelegatingReader{
			CacheReader:  cache,
			ClientReader: c,
		},
		Writer:       c,
		StatusClient: c,
	}, nil
}

// defaultHealthProbeListener creates the default health probes listener bound to the given address
func defaultHealthProbeListener(addr string) (net.Listener, error) {
	if addr == "" || addr == "0" {
		return nil, nil
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("error listening on %s: %v", addr, err)
	}
	return ln, nil
}

// setOptionsDefaults set default values for Options fields
func setOptionsDefaults(options Options) Options {
	// Use the Kubernetes client-go scheme if none is specified
	if options.Scheme == nil {
		options.Scheme = scheme.Scheme
	}

	if options.MapperProvider == nil {
		options.MapperProvider = func(c *rest.Config) (meta.RESTMapper, error) {
			return apiutil.NewDynamicRESTMapper(c)
		}
	}

	// Allow newClient to be mocked
	if options.NewClient == nil {
		options.NewClient = defaultNewClient
	}

	// Allow newCache to be mocked
	if options.NewCache == nil {
		options.NewCache = cache.New
	}

	// Allow newRecorderProvider to be mocked
	if options.newRecorderProvider == nil {
		options.newRecorderProvider = internalrecorder.NewProvider
	}

	// Allow newResourceLock to be mocked
	if options.newResourceLock == nil {
		options.newResourceLock = leaderelection.NewResourceLock
	}

	if options.newMetricsListener == nil {
		options.newMetricsListener = metrics.NewListener
	}
	leaseDuration, renewDeadline, retryPeriod := defaultLeaseDuration, defaultRenewDeadline, defaultRetryPeriod
	if options.LeaseDuration == nil {
		options.LeaseDuration = &leaseDuration
	}

	if options.RenewDeadline == nil {
		options.RenewDeadline = &renewDeadline
	}

	if options.RetryPeriod == nil {
		options.RetryPeriod = &retryPeriod
	}

	if options.EventBroadcaster == nil {
		options.EventBroadcaster = record.NewBroadcaster()
	}

	if options.ReadinessEndpointName == "" {
		options.ReadinessEndpointName = defaultReadinessEndpoint
	}

	if options.LivenessEndpointName == "" {
		options.LivenessEndpointName = defaultLivenessEndpoint
	}

	if options.newHealthProbeListener == nil {
		options.newHealthProbeListener = defaultHealthProbeListener
	}

	return options
}
