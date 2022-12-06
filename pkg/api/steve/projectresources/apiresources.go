package projectresources

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	apiextcontrollerv1 "github.com/rancher/wrangler/pkg/generated/controllers/apiextensions.k8s.io/v1"
	apiregcontrollerv1 "github.com/rancher/wrangler/pkg/generated/controllers/apiregistration.k8s.io/v1"
	"github.com/sirupsen/logrus"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apidiscovery "k8s.io/apiserver/pkg/endpoints/discovery"
	"k8s.io/client-go/discovery"
	apiregv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
)

// NOTE(cmurphy): copied with modifications from lasso and steve

var (
	queueRefreshDelay  = 500 * time.Millisecond
	enqueueAfterPeriod = 10 * time.Second
)

// APIResourceWatcher provides access to the Kubernetes schema.
type APIResourceWatcher interface {
	List() []metav1.APIResource
	Get(resource, group string) (metav1.APIResource, bool)
	GetKindForResource(gvr schema.GroupVersionResource) (string, error)
}

type apiResourceWatcher struct {
	sync.RWMutex

	toSync       atomic.Int32
	client       discovery.DiscoveryInterface
	apiResources []metav1.APIResource
	mapper       meta.RESTMapper
	resourceMap  map[string]metav1.APIResource
	crds         apiextcontrollerv1.CustomResourceDefinitionController
}

// WatchAPIResources creates an APIResourceWatcher object and starts watches on CRDs and APIServices,
// which prompts it to run a discovery check to get the most up to date Kubernetes schema.
func WatchAPIResources(ctx context.Context, discovery discovery.DiscoveryInterface, crds apiextcontrollerv1.CustomResourceDefinitionController, apiServices apiregcontrollerv1.APIServiceController, mapper meta.RESTMapper) APIResourceWatcher {
	a := &apiResourceWatcher{
		client:      discovery,
		mapper:      mapper,
		resourceMap: make(map[string]metav1.APIResource),
		crds:        crds,
	}

	crds.OnChange(ctx, "project-resources-api", a.OnChangeCRD)
	apiServices.OnChange(ctx, "project-resources-api", a.OnChangeAPIService)
	return a
}

// OnChangeCRD queues the refresh when a CRD event occurs.
func (a *apiResourceWatcher) OnChangeCRD(_ string, crd *apiextv1.CustomResourceDefinition) (*apiextv1.CustomResourceDefinition, error) {
	if crd != nil {
		logrus.Tracef("[%s] APIResourceWatcher handler triggered by CRD %s", apiServiceName, crd.Name)
	} else {
		logrus.Tracef("[%s] APIResourceWatcher handler triggered by deleted CRD", apiServiceName)
	}
	a.queueRefresh()
	return crd, nil
}

// OnChangeAPIService queues the refresh when an APIService event occurs.
func (a *apiResourceWatcher) OnChangeAPIService(_ string, apiService *apiregv1.APIService) (*apiregv1.APIService, error) {
	if apiService != nil {
		logrus.Tracef("[%s] APIResourceWatcher handler triggered by APIService %s", apiServiceName, apiService.Name)
	} else {
		logrus.Tracef("[%s] APIResourceWatcher handler triggered by deleted APIService", apiServiceName)
	}
	a.queueRefresh()
	return apiService, nil
}

// GetKindForResource returns the resource Kind given its GVR.
func (a *apiResourceWatcher) GetKindForResource(gvr schema.GroupVersionResource) (string, error) {
	a.RLock()
	defer a.RUnlock()
	gvk, err := a.mapper.KindFor(gvr)
	if err != nil {
		return "", fmt.Errorf("failed to look up kind for resource %s, %w", gvr.Resource, err)
	}
	return gvk.Kind, nil
}

// List returns all the APIResources for the project resources API.
func (a *apiResourceWatcher) List() []metav1.APIResource {
	a.RLock()
	defer a.RUnlock()
	list := make([]metav1.APIResource, len(a.apiResources))
	copy(list, a.apiResources)
	return list
}

// Get returns an APIResource and an existence bool given a resource and group.
func (a *apiResourceWatcher) Get(resource, group string) (metav1.APIResource, bool) {
	a.RLock()
	defer a.RUnlock()
	if group == "" {
		val, ok := a.resourceMap[resource]
		return val, ok
	}
	val, ok := a.resourceMap[group+"."+resource]
	return val, ok
}

func (a *apiResourceWatcher) queueRefresh() {
	a.toSync.Store(1)

	go func() {
		time.Sleep(queueRefreshDelay)
		if err := a.refreshAll(); err != nil {
			logrus.Debugf("[%s] failed to sync schemas, will retry: %v", apiServiceName, err)
			a.toSync.Store(1)
			a.crds.EnqueueAfter("", enqueueAfterPeriod)
		}
	}()
}

func (a *apiResourceWatcher) setAPIResources() error {
	resourceList, err := discovery.ServerPreferredNamespacedResources(a.client)
	// discovery often fails because this API isn't ready yet, the rest of the resource list returned is still good
	if err != nil && len(resourceList) == 0 {
		return err
	}
	result := []metav1.APIResource{}
	a.Lock()
	defer a.Unlock()
	a.resourceMap = make(map[string]metav1.APIResource)
	for _, resource := range resourceList {
		if resource.GroupVersion == groupVersion {
			continue
		}
		gv := strings.Split(resource.GroupVersion, "/")
		group := ""
		version := ""
		if len(gv) > 1 {
			group = gv[0]
			version = gv[1]
		} else {
			version = gv[0]
		}
		for _, r := range resource.APIResources {
			name := r.Name
			kind := r.Kind
			if group != "" {
				name = group + "." + name
				kind = group + "." + kind
			}
			resource := metav1.APIResource{
				Name:               name,
				Group:              group,
				Version:            version,
				Kind:               kind,
				Verbs:              []string{"list"},
				Namespaced:         true,
				StorageVersionHash: apidiscovery.StorageVersionHash(group, version, r.Kind),
			}
			result = append(result, resource)
			a.resourceMap[name] = resource
		}
	}
	a.apiResources = result
	return nil
}

func (a *apiResourceWatcher) refreshAll() error {
	if !a.needToSync() {
		logrus.Tracef("[%s] no refresh needed", apiServiceName)
		return nil
	}

	logrus.Infof("[%s] Refreshing all types", apiServiceName)
	var start time.Time
	if logrus.GetLevel() >= logrus.TraceLevel {
		start = time.Now()
	}
	err := a.setAPIResources()
	if logrus.GetLevel() >= logrus.TraceLevel {
		logrus.Tracef("[%s] APIResource refresh completed in %d milliseconds", apiServiceName, time.Since(start).Milliseconds())
	}
	if err != nil {
		return err
	}

	return nil
}

func (a *apiResourceWatcher) needToSync() bool {
	old := a.toSync.Swap(0)
	return old == 1
}
