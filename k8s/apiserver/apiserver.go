/*
Copyright 2017 The Kubernetes Authors.

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

package apiserver

// A lot of this code was just copied from k8s directly so we will just copyright it to k8s
// to be safe

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"strings"

	"github.com/golang/glog"
	"github.com/rancher/norman/signal"
	"github.com/rancher/rancher/k8s/apiserver/auth"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsapiserver "k8s.io/apiextensions-apiserver/pkg/apiserver"
	apiextensionsinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/internalversion"
	apiextensionscmd "k8s.io/apiextensions-apiserver/pkg/cmd/server"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/healthz"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kube-aggregator/pkg/apis/apiregistration"
	aggregatorv1beta1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"
	aggregatorapiserver "k8s.io/kube-aggregator/pkg/apiserver"
	apiregistrationclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/internalclientset/typed/apiregistration/internalversion"
	internalapireginformers "k8s.io/kube-aggregator/pkg/client/informers/internalversion/apiregistration/internalversion"
	"k8s.io/kube-aggregator/pkg/controllers/autoregister"
	"k8s.io/kubernetes/cmd/kube-apiserver/app"
	"k8s.io/kubernetes/cmd/kube-apiserver/app/options"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/controller/garbagecollector"
	"k8s.io/kubernetes/pkg/controller/namespace"
	"k8s.io/kubernetes/pkg/kubeapiserver/authorizer/modes"
	"k8s.io/kubernetes/pkg/master/controller/crdregistration"
)

var (
	ignoredResources = map[schema.GroupResource]struct{}{
		{Group: "extensions", Resource: "replicationcontrollers"}:              {},
		{Group: "", Resource: "bindings"}:                                      {},
		{Group: "", Resource: "componentstatuses"}:                             {},
		{Group: "", Resource: "events"}:                                        {},
		{Group: "authentication.k8s.io", Resource: "tokenreviews"}:             {},
		{Group: "authorization.k8s.io", Resource: "subjectaccessreviews"}:      {},
		{Group: "authorization.k8s.io", Resource: "selfsubjectaccessreviews"}:  {},
		{Group: "authorization.k8s.io", Resource: "localsubjectaccessreviews"}: {},
		{Group: "authorization.k8s.io", Resource: "selfsubjectrulesreviews"}:   {},
		{Group: "apiregistration.k8s.io", Resource: "apiservices"}:             {},
		{Group: "apiextensions.k8s.io", Resource: "customresourcedefinitions"}: {},
	}

	apiVersionPriorities = map[schema.GroupVersion]priority{
		{Group: "", Version: "v1"}: {group: 18000, version: 1},
		// extensions is above the rest for CLI compatibility, though the level of unqalified resource compatibility we
		// can reasonably expect seems questionable.
		{Group: "extensions", Version: "v1beta1"}: {group: 17900, version: 1},
		// to my knowledge, nothing below here collides
		{Group: "apps", Version: "v1beta1"}:                          {group: 17800, version: 1},
		{Group: "apps", Version: "v1beta2"}:                          {group: 17800, version: 1},
		{Group: "authentication.k8s.io", Version: "v1"}:              {group: 17700, version: 15},
		{Group: "authentication.k8s.io", Version: "v1beta1"}:         {group: 17700, version: 9},
		{Group: "authorization.k8s.io", Version: "v1"}:               {group: 17600, version: 15},
		{Group: "authorization.k8s.io", Version: "v1beta1"}:          {group: 17600, version: 9},
		{Group: "autoscaling", Version: "v1"}:                        {group: 17500, version: 15},
		{Group: "autoscaling", Version: "v2beta1"}:                   {group: 17500, version: 9},
		{Group: "batch", Version: "v1"}:                              {group: 17400, version: 15},
		{Group: "batch", Version: "v1beta1"}:                         {group: 17400, version: 9},
		{Group: "batch", Version: "v2alpha1"}:                        {group: 17400, version: 9},
		{Group: "certificates.k8s.io", Version: "v1beta1"}:           {group: 17300, version: 9},
		{Group: "networking.k8s.io", Version: "v1"}:                  {group: 17200, version: 15},
		{Group: "policy", Version: "v1beta1"}:                        {group: 17100, version: 9},
		{Group: "rbac.authorization.k8s.io", Version: "v1"}:          {group: 17000, version: 15},
		{Group: "rbac.authorization.k8s.io", Version: "v1beta1"}:     {group: 17000, version: 12},
		{Group: "rbac.authorization.k8s.io", Version: "v1alpha1"}:    {group: 17000, version: 9},
		{Group: "settings.k8s.io", Version: "v1alpha1"}:              {group: 16900, version: 9},
		{Group: "storage.k8s.io", Version: "v1"}:                     {group: 16800, version: 15},
		{Group: "storage.k8s.io", Version: "v1beta1"}:                {group: 16800, version: 9},
		{Group: "apiextensions.k8s.io", Version: "v1beta1"}:          {group: 16700, version: 9},
		{Group: "admissionregistration.k8s.io", Version: "v1alpha1"}: {group: 16700, version: 9},
		{Group: "scheduling.k8s.io", Version: "v1alpha1"}:            {group: 16600, version: 9},
	}
)

type priority struct {
	group   int32
	version int32
}

func APIServer() {
	ctx := signal.SigTermCancelContext(context.Background())
	if err := RunAPIServer(ctx, NewAPIServerOptions()); err != nil {
		panic(err)
	}
}

type Options struct {
	ListenPort int
}

func NewAPIServerOptions() *Options {
	return &Options{
		ListenPort: 8081,
	}
}

func RunAPIServer(ctx context.Context, apiServer *Options) error {
	runOptions := options.NewServerRunOptions()
	runOptions.Authentication.ServiceAccounts.Lookup = false
	runOptions.Authorization.Mode = modes.ModeRBAC
	runOptions.InsecureServing.BindPort = apiServer.ListenPort
	runOptions.SecureServing.BindPort = 0
	runOptions.Etcd.StorageConfig.ServerList = []string{"http://127.0.0.1:2381"}
	runOptions.EnableAggregatorRouting = false

	_, ipNet, err := net.ParseCIDR("10.0.0.0/24")
	if err != nil {
		return err
	}
	runOptions.ServiceClusterIPRange = *ipNet

	kubeAPIServerConfig, sharedInformers, versionedInformers, _, serviceResolver, err := app.CreateKubeAPIServerConfig(runOptions, nil, nil)
	if err != nil {
		return err
	}
	kubeAPIServerConfig.ExtraConfig.EnableCoreControllers = false
	kubeAPIServerConfig.GenericConfig.Authenticator = auth.NewAuthentication()
	kubeAPIServerConfig.GenericConfig.Authorizer = auth.NewAuthorizer(kubeAPIServerConfig.GenericConfig.Authorizer)

	apiExtensionsConfig, err := createAPIExtensionsConfig(*kubeAPIServerConfig.GenericConfig, versionedInformers, runOptions)
	if err != nil {
		return err
	}

	apiExtensionsServer, err := apiExtensionsConfig.Complete().New(genericapiserver.EmptyDelegate)
	if err != nil {
		return err
	}

	kubeAPIServer, err := app.CreateKubeAPIServer(kubeAPIServerConfig, apiExtensionsServer.GenericAPIServer, sharedInformers, versionedInformers)
	if err != nil {
		return err
	}

	kubeAPIServer.GenericAPIServer.PrepareRun()
	apiExtensionsServer.GenericAPIServer.PrepareRun()

	aggregatorConfig, err := createAggregatorConfig(*kubeAPIServerConfig.GenericConfig, runOptions, versionedInformers, serviceResolver, nil)
	if err != nil {
		return err
	}

	aggregatorConfig.ExtraConfig.ServiceResolver = serviceResolver
	aggregatorServer, err := createAggregatorServer(aggregatorConfig, kubeAPIServer.GenericAPIServer, apiExtensionsServer.Informers)
	if err != nil {
		return err
	}

	aggregatorServer.GenericAPIServer.PrepareRun()

	done := make(chan error)
	server := http.Server{Addr: fmt.Sprintf("localhost:%d", apiServer.ListenPort), Handler: aggregatorServer.GenericAPIServer.Handler}

	go func() {
		done <- server.ListenAndServe()
	}()

	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()

	aggregatorServer.GenericAPIServer.AddPostStartHookOrDie("controllers", runControllers)
	aggregatorServer.GenericAPIServer.RunPostStartHooks(ctx.Done())

	return <-done
}

func runControllers(hookContext genericapiserver.PostStartHookContext) error {
	nsKubeconfig := *hookContext.LoopbackClientConfig
	nsKubeconfig.QPS *= 10
	nsKubeconfig.Burst *= 10
	k8sClient := kubernetes.NewForConfigOrDie(&nsKubeconfig)

	for {
		_, err := k8sClient.Discovery().ServerVersion()
		if err == nil {
			break
		}
		logrus.Info("Waiting for API server")
		time.Sleep(time.Second)
	}

	informerFactory := informers.NewSharedInformerFactory(k8sClient, 2*time.Hour)
	informersStarted := make(chan struct{})

	if err := startNamespaceController(hookContext, informerFactory); err != nil {
		return err
	}

	if err := startGarbageCollectorController(hookContext, informerFactory, informersStarted); err != nil {
		return err
	}

	informerFactory.Start(hookContext.StopCh)
	close(informersStarted)

	return nil
}

func startNamespaceController(hookContext genericapiserver.PostStartHookContext, informerFactory informers.SharedInformerFactory) error {
	restMapper := api.Registry.RESTMapper()

	namespaceKubeClient := kubernetes.NewForConfigOrDie(hookContext.LoopbackClientConfig)
	namespaceClientPool := dynamic.NewClientPool(hookContext.LoopbackClientConfig, restMapper, dynamic.LegacyAPIPathResolverFunc)

	discoverResourcesFn := namespaceKubeClient.Discovery().ServerPreferredNamespacedResources

	namespaceController := namespace.NewNamespaceController(
		namespaceKubeClient,
		namespaceClientPool,
		discoverResourcesFn,
		informerFactory.Core().V1().Namespaces(),
		5*time.Minute,
		v1.FinalizerKubernetes,
	)
	go namespaceController.Run(5, hookContext.StopCh)

	return nil
}

func startGarbageCollectorController(hookContext genericapiserver.PostStartHookContext, informerFactory informers.SharedInformerFactory, informersStarted <-chan struct{}) error {
	gcClientset := kubernetes.NewForConfigOrDie(hookContext.LoopbackClientConfig)

	// Use a discovery client capable of being refreshed.
	discoveryClient := cached.NewMemCacheClient(gcClientset.Discovery())
	restMapper := discovery.NewDeferredDiscoveryRESTMapper(discoveryClient, meta.InterfacesForUnstructured)
	restMapper.Reset()

	metaOnlyClientPool := dynamic.NewClientPool(hookContext.LoopbackClientConfig, restMapper, dynamic.LegacyAPIPathResolverFunc)
	clientPool := dynamic.NewClientPool(hookContext.LoopbackClientConfig, restMapper, dynamic.LegacyAPIPathResolverFunc)

	// Get an initial set of deletable resources to prime the garbage collector.
	deletableResources := garbagecollector.GetDeletableResources(discoveryClient)
	ignoredResources := make(map[schema.GroupResource]struct{})
	for r := range ignoredResources {
		ignoredResources[schema.GroupResource{Group: r.Group, Resource: r.Resource}] = struct{}{}
	}
	garbageCollector, err := garbagecollector.NewGarbageCollector(
		metaOnlyClientPool,
		clientPool,
		restMapper,
		deletableResources,
		ignoredResources,
		informerFactory,
		informersStarted,
	)
	if err != nil {
		return fmt.Errorf("Failed to start the generic garbage collector: %v", err)
	}

	// Start the garbage collector.
	go garbageCollector.Run(20, hookContext.StopCh)

	// Periodically refresh the RESTMapper with new discovery information and sync
	// the garbage collector.
	go garbageCollector.Sync(gcClientset.Discovery(), 30*time.Second, hookContext.StopCh)

	return nil
}

func createAPIExtensionsConfig(kubeAPIServerConfig genericapiserver.Config, externalInformers informers.SharedInformerFactory, commandOptions *options.ServerRunOptions) (*apiextensionsapiserver.Config, error) {
	genericConfig := kubeAPIServerConfig
	etcdOptions := *commandOptions.Etcd
	etcdOptions.StorageConfig.Codec = apiextensionsapiserver.Codecs.LegacyCodec(v1beta1.SchemeGroupVersion)
	etcdOptions.StorageConfig.Copier = apiextensionsapiserver.Scheme
	genericConfig.RESTOptionsGetter = &genericoptions.SimpleRestOptionsFactory{Options: etcdOptions}

	apiextensionsConfig := &apiextensionsapiserver.Config{
		GenericConfig: &genericapiserver.RecommendedConfig{
			Config:                genericConfig,
			SharedInformerFactory: externalInformers,
		},
		ExtraConfig: apiextensionsapiserver.ExtraConfig{
			CRDRESTOptionsGetter: apiextensionscmd.NewCRDRESTOptionsGetter(etcdOptions),
		},
	}

	return apiextensionsConfig, nil
}

func createAggregatorConfig(kubeAPIServerConfig genericapiserver.Config, commandOptions *options.ServerRunOptions, externalInformers informers.SharedInformerFactory, serviceResolver aggregatorapiserver.ServiceResolver, proxyTransport *http.Transport) (*aggregatorapiserver.Config, error) {
	// make a shallow copy to let us twiddle a few things
	// most of the config actually remains the same.  We only need to mess with a couple items related to the particulars of the aggregator
	genericConfig := kubeAPIServerConfig

	// the aggregator doesn't wire these up.  It just delegates them to the kubeapiserver
	genericConfig.EnableSwaggerUI = false
	genericConfig.SwaggerConfig = nil

	// copy the etcd options so we don't mutate originals.
	etcdOptions := *commandOptions.Etcd
	etcdOptions.StorageConfig.Codec = aggregatorapiserver.Codecs.LegacyCodec(aggregatorv1beta1.SchemeGroupVersion)
	etcdOptions.StorageConfig.Copier = aggregatorapiserver.Scheme
	genericConfig.RESTOptionsGetter = &genericoptions.SimpleRestOptionsFactory{Options: etcdOptions}

	aggregatorConfig := &aggregatorapiserver.Config{
		GenericConfig: &genericapiserver.RecommendedConfig{
			Config:                genericConfig,
			SharedInformerFactory: externalInformers,
		},
		ExtraConfig: aggregatorapiserver.ExtraConfig{
			ServiceResolver: serviceResolver,
			ProxyTransport:  proxyTransport,
		},
	}

	return aggregatorConfig, nil
}

func createAggregatorServer(aggregatorConfig *aggregatorapiserver.Config, delegateAPIServer genericapiserver.DelegationTarget, apiExtensionInformers apiextensionsinformers.SharedInformerFactory) (*aggregatorapiserver.APIAggregator, error) {
	aggregatorServer, err := aggregatorConfig.Complete().NewWithDelegate(delegateAPIServer)
	if err != nil {
		return nil, err
	}

	// create controllers for auto-registration
	apiRegistrationClient, err := apiregistrationclient.NewForConfig(aggregatorConfig.GenericConfig.LoopbackClientConfig)
	if err != nil {
		return nil, err
	}
	autoRegistrationController := autoregister.NewAutoRegisterController(aggregatorServer.APIRegistrationInformers.Apiregistration().InternalVersion().APIServices(), apiRegistrationClient)
	apiServices := apiServicesToRegister(delegateAPIServer, autoRegistrationController)
	crdRegistrationController := crdregistration.NewAutoRegistrationController(
		apiExtensionInformers.Apiextensions().InternalVersion().CustomResourceDefinitions(),
		autoRegistrationController)

	aggregatorServer.GenericAPIServer.AddPostStartHook("kube-apiserver-autoregistration", func(context genericapiserver.PostStartHookContext) error {
		go crdRegistrationController.Run(5, context.StopCh)
		go func() {
			// let the CRD controller process the initial set of CRDs before starting the autoregistration controller.
			// this prevents the autoregistration controller's initial sync from deleting APIServices for CRDs that still exist.
			crdRegistrationController.WaitForInitialSync()
			autoRegistrationController.Run(5, context.StopCh)
		}()
		return nil
	})

	aggregatorServer.GenericAPIServer.AddHealthzChecks(
		makeAPIServiceAvailableHealthzCheck(
			"autoregister-completion",
			apiServices,
			aggregatorServer.APIRegistrationInformers.Apiregistration().InternalVersion().APIServices(),
		),
	)

	return aggregatorServer, nil
}

func apiServicesToRegister(delegateAPIServer genericapiserver.DelegationTarget, registration autoregister.AutoAPIServiceRegistration) []*apiregistration.APIService {
	var apiServices []*apiregistration.APIService

	for _, curr := range delegateAPIServer.ListedPaths() {
		if curr == "/api/v1" {
			apiService := makeAPIService(schema.GroupVersion{Group: "", Version: "v1"})
			registration.AddAPIServiceToSyncOnStart(apiService)
			apiServices = append(apiServices, apiService)
			continue
		}

		if !strings.HasPrefix(curr, "/apis/") {
			continue
		}
		// this comes back in a list that looks like /apis/rbac.authorization.k8s.io/v1alpha1
		tokens := strings.Split(curr, "/")
		if len(tokens) != 4 {
			continue
		}

		apiService := makeAPIService(schema.GroupVersion{Group: tokens[2], Version: tokens[3]})
		if apiService == nil {
			continue
		}
		registration.AddAPIServiceToSyncOnStart(apiService)
		apiServices = append(apiServices, apiService)
	}

	return apiServices
}

func makeAPIService(gv schema.GroupVersion) *apiregistration.APIService {
	apiServicePriority, ok := apiVersionPriorities[gv]
	if !ok {
		// if we aren't found, then we shouldn't register ourselves because it could result in a CRD group version
		// being permanently stuck in the APIServices list.
		glog.Infof("Skipping APIService creation for %v", gv)
		return nil
	}
	return &apiregistration.APIService{
		ObjectMeta: metav1.ObjectMeta{Name: gv.Version + "." + gv.Group},
		Spec: apiregistration.APIServiceSpec{
			Group:                gv.Group,
			Version:              gv.Version,
			GroupPriorityMinimum: apiServicePriority.group,
			VersionPriority:      apiServicePriority.version,
		},
	}
}

// makeAPIServiceAvailableHealthzCheck returns a healthz check that returns healthy
// once all of the specified services have been observed to be available at least once.
func makeAPIServiceAvailableHealthzCheck(name string, apiServices []*apiregistration.APIService, apiServiceInformer internalapireginformers.APIServiceInformer) healthz.HealthzChecker {
	// Track the auto-registered API services that have not been observed to be available yet
	pendingServiceNamesLock := &sync.RWMutex{}
	pendingServiceNames := sets.NewString()
	for _, service := range apiServices {
		pendingServiceNames.Insert(service.Name)
	}

	// When an APIService in the list is seen as available, remove it from the pending list
	handleAPIServiceChange := func(service *apiregistration.APIService) {
		pendingServiceNamesLock.Lock()
		defer pendingServiceNamesLock.Unlock()
		if !pendingServiceNames.Has(service.Name) {
			return
		}
		if apiregistration.IsAPIServiceConditionTrue(service, apiregistration.Available) {
			pendingServiceNames.Delete(service.Name)
		}
	}

	// Watch add/update events for APIServices
	apiServiceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { handleAPIServiceChange(obj.(*apiregistration.APIService)) },
		UpdateFunc: func(old, new interface{}) { handleAPIServiceChange(new.(*apiregistration.APIService)) },
	})

	// Don't return healthy until the pending list is empty
	return healthz.NamedCheck(name, func(r *http.Request) error {
		pendingServiceNamesLock.RLock()
		defer pendingServiceNamesLock.RUnlock()
		if pendingServiceNames.Len() > 0 {
			return fmt.Errorf("missing APIService: %v", pendingServiceNames.List())
		}
		return nil
	})
}
