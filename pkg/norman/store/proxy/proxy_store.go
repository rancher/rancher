package proxy

import (
	"context"
	ejson "encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/objectclient/dynamic"
	"github.com/rancher/norman/pkg/broadcast"
	"github.com/rancher/norman/restwatch"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/convert/merge"
	"github.com/rancher/norman/types/values"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/runtime/serializer/streaming"
	"k8s.io/apimachinery/pkg/util/cache"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	restclientwatch "k8s.io/client-go/rest/watch"
)

var (
	userAuthHeader = "Impersonate-User"
	authHeaders    = []string{
		userAuthHeader,
		"Impersonate-Group",
	}

	// minAccessControlValidity is the minimum period during which the AccessControl data attached to a request must be considered valid.
	// This avoids capping the number of forced expirations to only one every 2s (default)
	// This can be configured via the environment variable "CATTLE_MIN_ACCESSCONTROL_VALIDITY_SECONDS" (values equal or lower than zero disables this mechanism).
	minAccessControlValidityPeriod = 2 * time.Second

	// lastExpiredRequests is a *types.APIContext => struct{} self-expiring cache, which temporarily stores the last requests that were expired
	lastExpiredRequests = cache.NewLRUExpireCache(1000)
)

func init() {
	if v := os.Getenv("CATTLE_MIN_ACCESSCONTROL_VALIDITY_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			minAccessControlValidityPeriod = time.Duration(n) * time.Second
		}
	}
}

type ClientGetter interface {
	UnversionedClient(apiContext *types.APIContext, context types.StorageContext) (rest.Interface, error)
	APIExtClient(apiContext *types.APIContext, context types.StorageContext) (clientset.Interface, error)
}

type simpleClientGetter struct {
	restConfig   rest.Config
	client       rest.Interface
	apiExtClient clientset.Interface
}

func NewClientGetterFromConfig(config rest.Config) (ClientGetter, error) {
	dynamicConfig := config
	if dynamicConfig.NegotiatedSerializer == nil {
		dynamicConfig.NegotiatedSerializer = dynamic.NegotiatedSerializer
	}

	unversionedClient, err := rest.UnversionedRESTClientFor(&dynamicConfig)
	if err != nil {
		return nil, err
	}

	apiExtClient, err := clientset.NewForConfig(&dynamicConfig)
	if err != nil {
		return nil, err
	}

	return &simpleClientGetter{
		restConfig:   config,
		client:       unversionedClient,
		apiExtClient: apiExtClient,
	}, nil
}

func (s *simpleClientGetter) Config(apiContext *types.APIContext, context types.StorageContext) (rest.Config, error) {
	return s.restConfig, nil
}

func (s *simpleClientGetter) UnversionedClient(apiContext *types.APIContext, context types.StorageContext) (rest.Interface, error) {
	return s.client, nil
}

func (s *simpleClientGetter) APIExtClient(apiContext *types.APIContext, context types.StorageContext) (clientset.Interface, error) {
	return s.apiExtClient, nil
}

type StoreTyper interface {
	runtime.ObjectConvertor
	runtime.ObjectCreater
}

type Store struct {
	sync.Mutex

	clientGetter   ClientGetter
	storageContext types.StorageContext
	prefix         []string
	group          string
	version        string
	kind           string
	resourcePlural string
	authContext    map[string]string
	close          context.Context
	broadcasters   map[rest.Interface]*broadcast.Broadcaster
	typer          StoreTyper
}

func NewProxyStore(ctx context.Context, clientGetter ClientGetter, storageContext types.StorageContext, typer StoreTyper,
	prefix []string, group, version, kind, resourcePlural string) types.Store {

	// Default to an empty scheme, all types will default to generic
	if typer == nil {
		typer = runtime.NewScheme()
	}

	return &errorStore{
		Store: &Store{
			clientGetter:   clientGetter,
			storageContext: storageContext,
			prefix:         prefix,
			group:          group,
			version:        version,
			kind:           kind,
			resourcePlural: resourcePlural,
			authContext: map[string]string{
				"apiGroup": group,
				"resource": resourcePlural,
			},
			close:        ctx,
			broadcasters: map[rest.Interface]*broadcast.Broadcaster{},
			typer:        typer,
		},
	}
}

func (s *Store) getUser(apiContext *types.APIContext) string {
	return apiContext.Request.Header.Get(userAuthHeader)
}

func (s *Store) doAuthed(apiContext *types.APIContext, request *rest.Request) rest.Result {
	start := time.Now()
	defer func() {
		logrus.Tracef("GET: %v, %v", time.Since(start), s.resourcePlural)
	}()

	for _, header := range authHeaders {
		request.SetHeader(header, apiContext.Request.Header[http.CanonicalHeaderKey(header)]...)
	}

	//set extra info headers
	for header := range apiContext.Request.Header {
		if strings.HasPrefix(header, "Impersonate-Extra-") {
			request.SetHeader(header, apiContext.Request.Header[http.CanonicalHeaderKey(header)]...)
		}
	}

	return request.Do(apiContext.Request.Context())
}

func (s *Store) k8sClient(apiContext *types.APIContext) (rest.Interface, error) {
	return s.clientGetter.UnversionedClient(apiContext, s.storageContext)
}

func (s *Store) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	_, result, err := s.byID(apiContext, schema, id, true)
	return result, err
}

func (s *Store) byID(apiContext *types.APIContext, schema *types.Schema, id string, retry bool) (string, map[string]interface{}, error) {
	splitted := strings.Split(strings.TrimSpace(id), ":")
	validID := false
	namespaced := schema.Scope == types.NamespaceScope
	if namespaced {
		validID = len(splitted) == 2 && len(strings.TrimSpace(splitted[0])) > 0 && len(strings.TrimSpace(splitted[1])) > 0
	} else {
		validID = len(splitted) == 1 && len(strings.TrimSpace(splitted[0])) > 0
	}
	if !validID {
		return "", nil, httperror.NewAPIError(httperror.NotFound, "failed to find resource by id")
	}

	namespace, id := splitID(id)

	k8sClient, err := s.k8sClient(apiContext)
	if err != nil {
		return "", nil, err
	}

	req := s.common(namespace, k8sClient.Get()).Name(id)
	if !retry {
		return s.singleResult(apiContext, schema, req)
	}

	var version string
	var data map[string]interface{}
	for i := 0; i < 3; i++ {
		req = s.common(namespace, k8sClient.Get()).Name(id)
		version, data, err = s.singleResult(apiContext, schema, req)
		if err != nil {
			if i < 2 && strings.Contains(err.Error(), "Client.Timeout exceeded") {
				logrus.Warnf("Retrying GET. Error: %v", err)
				continue
			}
			return version, data, err
		}
		return version, data, err
	}
	return version, data, err
}

func (s *Store) Context() types.StorageContext {
	return s.storageContext
}

func (s *Store) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	result := []map[string]interface{}{}

	// if there are no namespaces field in options, a single request is made
	if opt == nil || opt.Namespaces == nil {
		ns := getNamespace(apiContext, opt)
		resultList := s.getListStruct()

		err := s.retryList(ns, apiContext, resultList)
		if err != nil {
			return nil, err
		}

		collectionResults, _ := s.collectionFromInternal(resultList, apiContext, schema)
		result = append(result, collectionResults...)
	} else {
		var (
			errGroup errgroup.Group
			mux      sync.Mutex
		)

		allNS := opt.Namespaces
		for _, ns := range allNS {
			nsCopy := ns
			errGroup.Go(func() error {
				resultList := s.getListStruct()

				err := s.retryList(nsCopy, apiContext, resultList)
				if err != nil {
					return err
				}

				mux.Lock()
				collectionResults, _ := s.collectionFromInternal(resultList, apiContext, schema)
				result = append(result, collectionResults...)
				mux.Unlock()

				return nil
			})
		}
		if err := errGroup.Wait(); err != nil {
			return nil, err
		}
	}

	return apiContext.AccessControl.FilterList(apiContext, schema, result, s.authContext), nil
}

func (s *Store) retryList(namespace string, apiContext *types.APIContext, resultList runtime.Object) error {
	k8sClient, err := s.k8sClient(apiContext)
	if err != nil {
		return err
	}

	for i := 0; i < 3; i++ {
		req := s.common(namespace, k8sClient.Get())
		start := time.Now()
		err = req.Do(apiContext.Request.Context()).Into(resultList)
		logrus.Tracef("LIST: %v, %v", time.Since(start), s.resourcePlural)
		if err != nil {
			if i < 2 && strings.Contains(err.Error(), "Client.Timeout exceeded") {
				logrus.Infof("Error on LIST %v: %v. Attempt: %v. Retrying", s.resourcePlural, err, i+1)
				continue
			}
			return err
		}
		return err
	}
	return err
}

func shouldExpireAccessControl(apiContext *types.APIContext) bool {
	// Allow disabling this mechanism, configuring zero or negative values
	if minAccessControlValidityPeriod <= 0 {
		return true
	}

	// Skip expirations if last occurrence happened closer than the configured period
	// This uses a self-expiring cache, so it omits the results for already expired entries.
	// By storing an empty value and relying on its own expiration mechanism, we avoid the redundancy of storing the time itself
	if _, ok := lastExpiredRequests.Get(apiContext); ok {
		return false
	}
	lastExpiredRequests.Add(apiContext, struct{}{}, minAccessControlValidityPeriod)
	return true
}

func (s *Store) Watch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	c, err := s.shareWatch(apiContext, schema, opt)
	if err != nil {
		return nil, err
	}

	return convert.Chan(c, func(data map[string]interface{}) map[string]interface{} {
		if shouldExpireAccessControl(apiContext) {
			apiContext.ExpireAccessControl(schema)
		}
		return apiContext.AccessControl.Filter(apiContext, schema, data, s.authContext)
	}), nil
}

func (s *Store) realWatch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	namespace := getNamespace(apiContext, opt)

	k8sClient, err := s.k8sClient(apiContext)
	if err != nil {
		return nil, err
	}

	if watchClient, ok := k8sClient.(restwatch.WatchClient); ok {
		k8sClient = watchClient.WatchClient()
	}

	timeout := int64(60 * 30)
	req := s.common(namespace, k8sClient.Get())
	req.VersionedParams(&metav1.ListOptions{
		Watch:           true,
		TimeoutSeconds:  &timeout,
		ResourceVersion: "0",
	}, metav1.ParameterCodec)

	body, err := req.Stream(s.close)
	if err != nil {
		return nil, err
	}

	framer := json.Framer.NewFrameReader(body)
	decoder := streaming.NewDecoder(framer, &unstructuredDecoder{})
	watcher := watch.NewStreamWatcher(restclientwatch.NewDecoder(decoder, &unstructuredDecoder{}), &errorReporter{})

	watchingContext, cancelWatchingContext := context.WithCancel(s.close)
	go func() {
		<-watchingContext.Done()
		logrus.Tracef("stopping watcher for %s", schema.ID)
		watcher.Stop()
	}()

	result := make(chan map[string]interface{})
	go func() {
		for event := range watcher.ResultChan() {
			if data, ok := event.Object.(*metav1.Status); ok {
				// just logging it, keeping the same behavior as before
				logrus.Tracef("watcher status for %s: %s", schema.ID, data.Message)
			} else {
				data := event.Object.(*unstructured.Unstructured)
				s.fromInternal(apiContext, schema, data.Object)
				if event.Type == watch.Deleted && data.Object != nil {
					data.Object[".removed"] = true
				}
				result <- data.Object
			}
		}
		logrus.Tracef("closing watcher for %s", schema.ID)
		close(result)
		cancelWatchingContext()
	}()

	return result, nil
}

type unstructuredDecoder struct {
}

type errorReporter struct {
}

func (e *errorReporter) AsObject(err error) runtime.Object {
	return &metav1.Status{Message: err.Error(), Code: http.StatusInternalServerError, Reason: "ClientWatchDecoding"}
}

func (d *unstructuredDecoder) Decode(data []byte, defaults *schema.GroupVersionKind, into runtime.Object) (runtime.Object, *schema.GroupVersionKind, error) {
	if into == nil {
		into = &unstructured.Unstructured{}
	}
	return into, defaults, ejson.Unmarshal(data, &into)
}

func getNamespace(apiContext *types.APIContext, opt *types.QueryOptions) string {
	if val, ok := apiContext.SubContext["namespaces"]; ok {
		return convert.ToString(val)
	}

	for _, condition := range opt.Conditions {
		mod := condition.ToCondition().Modifier
		if condition.Field == "namespaceId" && condition.Value != "" && mod == types.ModifierEQ {
			return condition.Value
		}
		if condition.Field == "namespace" && condition.Value != "" && mod == types.ModifierEQ {
			return condition.Value
		}
	}

	return ""
}

func (s *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	if err := s.toInternal(schema.Mapper, data); err != nil {
		return nil, err
	}

	namespace, _ := values.GetValueN(data, "metadata", "namespace").(string)

	values.PutValue(data, s.getUser(apiContext), "metadata", "annotations", "field.cattle.io/creatorId")
	values.PutValue(data, "norman", "metadata", "labels", "cattle.io/creator")

	name, _ := values.GetValueN(data, "metadata", "name").(string)
	if name == "" {
		generated, _ := values.GetValueN(data, "metadata", "generateName").(string)
		if generated == "" {
			values.PutValue(data, types.GenerateTypePrefix(schema.ID), "metadata", "generateName")
		}
	}

	k8sClient, err := s.k8sClient(apiContext)
	if err != nil {
		return nil, err
	}

	req := s.common(namespace, k8sClient.Post()).
		Body(&unstructured.Unstructured{
			Object: data,
		})

	_, result, err := s.singleResult(apiContext, schema, req)
	return result, err
}

func (s *Store) toInternal(mapper types.Mapper, data map[string]interface{}) error {
	if mapper != nil {
		if err := mapper.ToInternal(data); err != nil {
			return err
		}
	}

	if s.group == "" {
		data["apiVersion"] = s.version
	} else {
		data["apiVersion"] = s.group + "/" + s.version
	}
	data["kind"] = s.kind
	return nil
}

func (s *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	var (
		result map[string]interface{}
		err    error
	)

	k8sClient, err := s.k8sClient(apiContext)
	if err != nil {
		return nil, err
	}

	namespace, id := splitID(id)
	if err := s.toInternal(schema.Mapper, data); err != nil {
		return nil, err
	}

	for i := 0; i < 5; i++ {
		req := s.common(namespace, k8sClient.Get()).
			Name(id)

		resourceVersion, existing, rawErr := s.singleResultRaw(apiContext, schema, req)
		if rawErr != nil {
			return nil, rawErr
		}

		existing = merge.APIUpdateMerge(schema.InternalSchema, apiContext.Schemas, existing, data, apiContext.Option("replace") == "true")

		values.PutValue(existing, resourceVersion, "metadata", "resourceVersion")
		values.PutValue(existing, namespace, "metadata", "namespace")
		values.PutValue(existing, id, "metadata", "name")

		req = s.common(namespace, k8sClient.Put()).
			Body(&unstructured.Unstructured{
				Object: existing,
			}).
			Name(id)

		_, result, err = s.singleResult(apiContext, schema, req)
		if errors.IsConflict(err) {
			continue
		}
		return result, err
	}

	return result, err
}

func (s *Store) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	k8sClient, err := s.k8sClient(apiContext)
	if err != nil {
		return nil, err
	}

	namespace, name := splitID(id)
	options, err := getDeleteOption(apiContext.Request)
	if err != nil {
		return nil, err
	}
	req := s.common(namespace, k8sClient.Delete()).
		Body(options).
		Name(name)

	err = s.doAuthed(apiContext, req).Error()
	if err != nil {
		return nil, err
	}

	_, obj, err := s.byID(apiContext, schema, id, false)
	if err != nil {
		return nil, nil
	}
	return obj, nil
}

func (s *Store) singleResult(apiContext *types.APIContext, schema *types.Schema, req *rest.Request) (string, map[string]interface{}, error) {
	version, data, err := s.singleResultRaw(apiContext, schema, req)
	if err != nil {
		return "", nil, err
	}
	s.fromInternal(apiContext, schema, data)
	return version, data, nil
}

func (s *Store) singleResultRaw(apiContext *types.APIContext, schema *types.Schema, req *rest.Request) (string, map[string]interface{}, error) {
	result := &unstructured.Unstructured{}
	err := s.doAuthed(apiContext, req).Into(result)
	if err != nil {
		return "", nil, err
	}

	return result.GetResourceVersion(), result.Object, nil
}

func splitID(id string) (string, string) {
	namespace := ""
	parts := strings.SplitN(id, ":", 2)
	if len(parts) == 2 {
		namespace = parts[0]
		id = parts[1]
	}

	return namespace, id
}

func getDeleteOption(req *http.Request) (*metav1.DeleteOptions, error) {
	options := &metav1.DeleteOptions{}
	values := req.URL.Query()
	if err := metav1.Convert_url_Values_To_v1_DeleteOptions(&values, options, nil); err != nil {
		return nil, err
	}
	prop := metav1.DeletePropagationBackground
	if options.PropagationPolicy == nil {
		options.PropagationPolicy = &prop
	}
	return options, nil
}

func (s *Store) common(namespace string, req *rest.Request) *rest.Request {
	prefix := append([]string{}, s.prefix...)
	if s.group != "" {
		prefix = append(prefix, s.group)
	}
	prefix = append(prefix, s.version)
	req.Prefix(prefix...).
		Resource(s.resourcePlural)

	if namespace != "" {
		req.Namespace(namespace)
	}

	return req
}

func (s *Store) fromInternal(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) map[string]interface{} {
	if apiContext.Option("export") == "true" {
		delete(data, "status")
	}
	if schema.Mapper != nil {
		schema.Mapper.FromInternal(data)
	}

	return data
}

// getListStruct returns a runtime object for storing results from list requests.  If the Store's scheme does not return
// a type for the resource associated with the store, a generic type will be used.
func (s *Store) getListStruct() runtime.Object {
	// try to find the list type for this store
	obj, err := s.typer.New(schema.GroupVersionKind{
		Group:   s.group,
		Version: s.version,
		Kind:    s.kind + "List",
	})
	// if we cannot get the specific type default to a generic parser
	if err != nil {
		logrus.Debugf("Falling back to generic list type for [%s]: %v", s.kind, err)
		return new(unstructured.UnstructuredList)
	}

	return obj
}

// collectionFromInternal maps a collection runtime object to an array of maps.
func (s *Store) collectionFromInternal(list runtime.Object, apiContext *types.APIContext, schema *types.Schema) ([]map[string]interface{}, error) {
	var ul unstructured.UnstructuredList

	err := s.typer.Convert(list, &ul, nil)
	if err != nil {
		return nil, err
	}

	results := make([]map[string]interface{}, len(ul.Items))
	for i := range ul.Items {
		results[i] = s.fromInternal(apiContext, schema, ul.Items[i].Object)
	}

	return results, nil
}
