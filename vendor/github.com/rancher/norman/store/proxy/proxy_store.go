package proxy

import (
	ejson "encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/restwatch"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/convert/merge"
	"github.com/rancher/norman/types/values"
	"github.com/sirupsen/logrus"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/runtime/serializer/streaming"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	restclientwatch "k8s.io/client-go/rest/watch"
)

var (
	userAuthHeader = "Impersonate-User"
	authHeaders    = []string{
		userAuthHeader,
		"Impersonate-Group",
	}
)

type ClientGetter interface {
	Config(apiContext *types.APIContext, context types.StorageContext) (rest.Config, error)
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
		configConfig := dynamic.ContentConfig()
		dynamicConfig.NegotiatedSerializer = configConfig.NegotiatedSerializer
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

type Store struct {
	clientGetter   ClientGetter
	storageContext types.StorageContext
	prefix         []string
	group          string
	version        string
	kind           string
	resourcePlural string
	authContext    map[string]string
}

func NewProxyStore(clientGetter ClientGetter, storageContext types.StorageContext,
	prefix []string, group, version, kind, resourcePlural string) types.Store {
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
		},
	}
}

func (p *Store) getUser(apiContext *types.APIContext) string {
	return apiContext.Request.Header.Get(userAuthHeader)
}

func (p *Store) doAuthed(apiContext *types.APIContext, request *rest.Request) rest.Result {
	start := time.Now()
	defer func() {
		logrus.Debug("GET: ", time.Now().Sub(start), p.resourcePlural)
	}()

	for _, header := range authHeaders {
		request.SetHeader(header, apiContext.Request.Header[http.CanonicalHeaderKey(header)]...)
	}
	return request.Do()
}

func (p *Store) k8sClient(apiContext *types.APIContext) (rest.Interface, error) {
	return p.clientGetter.UnversionedClient(apiContext, p.storageContext)
}

func (p *Store) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	splitted := strings.Split(strings.TrimSpace(id), ":")
	validID := false
	namespaced := schema.Scope == types.NamespaceScope
	if namespaced {
		validID = len(splitted) == 2 && len(strings.TrimSpace(splitted[0])) > 0 && len(strings.TrimSpace(splitted[1])) > 0
	} else {
		validID = len(splitted) == 1 && len(strings.TrimSpace(splitted[0])) > 0
	}
	if !validID {
		return nil, httperror.NewAPIError(httperror.NotFound, "failed to find resource by id")
	}

	_, result, err := p.byID(apiContext, schema, id)
	return result, err
}

func (p *Store) byID(apiContext *types.APIContext, schema *types.Schema, id string) (string, map[string]interface{}, error) {
	namespace, id := splitID(id)

	k8sClient, err := p.k8sClient(apiContext)
	if err != nil {
		return "", nil, err
	}

	req := p.common(namespace, k8sClient.Get()).
		Name(id)

	return p.singleResult(apiContext, schema, req)
}

func (p *Store) Context() types.StorageContext {
	return p.storageContext
}

func (p *Store) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	namespace := getNamespace(apiContext, opt)

	k8sClient, err := p.k8sClient(apiContext)
	if err != nil {
		return nil, err
	}

	req := p.common(namespace, k8sClient.Get())

	resultList := &unstructured.UnstructuredList{}
	start := time.Now()
	err = req.Do().Into(resultList)
	logrus.Debug("LIST: ", time.Now().Sub(start), p.resourcePlural)
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}

	for _, obj := range resultList.Items {
		result = append(result, p.fromInternal(schema, obj.Object))
	}

	return apiContext.AccessControl.FilterList(apiContext, schema, result, p.authContext), nil
}

func (p *Store) Watch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	namespace := getNamespace(apiContext, opt)

	k8sClient, err := p.k8sClient(apiContext)
	if err != nil {
		return nil, err
	}

	if watchClient, ok := k8sClient.(restwatch.WatchClient); ok {
		k8sClient = watchClient.WatchClient()
	}

	timeout := int64(60 * 60)
	req := p.common(namespace, k8sClient.Get())
	req.VersionedParams(&metav1.ListOptions{
		Watch:           true,
		TimeoutSeconds:  &timeout,
		ResourceVersion: "0",
	}, dynamic.VersionedParameterEncoderWithV1Fallback)

	body, err := req.Stream()
	if err != nil {
		return nil, err
	}

	framer := json.Framer.NewFrameReader(body)
	decoder := streaming.NewDecoder(framer, &unstructuredDecoder{})
	watcher := watch.NewStreamWatcher(restclientwatch.NewDecoder(decoder, &unstructuredDecoder{}))

	go func() {
		<-apiContext.Request.Context().Done()
		logrus.Debugf("stopping watcher for %s", schema.ID)
		watcher.Stop()
	}()

	result := make(chan map[string]interface{})
	go func() {
		for event := range watcher.ResultChan() {
			data := event.Object.(*unstructured.Unstructured)
			p.fromInternal(schema, data.Object)
			if event.Type == watch.Deleted && data.Object != nil {
				data.Object[".removed"] = true
			}
			result <- apiContext.AccessControl.Filter(apiContext, schema, data.Object, p.authContext)
		}
		logrus.Debugf("closing watcher for %s", schema.ID)
		close(result)
	}()

	return result, nil
}

type unstructuredDecoder struct {
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
		if condition.Field == "namespaceId" && condition.Value != "" {
			return condition.Value
		}
	}

	return ""
}

func (p *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	namespace, _ := data["namespaceId"].(string)
	p.toInternal(schema.Mapper, data)

	values.PutValue(data, p.getUser(apiContext), "metadata", "annotations", "field.cattle.io/creatorId")

	name, _ := values.GetValueN(data, "metadata", "name").(string)
	if name == "" {
		generated, _ := values.GetValueN(data, "metadata", "generateName").(string)
		if generated == "" {
			values.PutValue(data, types.GenerateName(schema.ID), "metadata", "name")
		}
	}

	k8sClient, err := p.k8sClient(apiContext)
	if err != nil {
		return nil, err
	}

	req := p.common(namespace, k8sClient.Post()).
		Body(&unstructured.Unstructured{
			Object: data,
		})

	_, result, err := p.singleResult(apiContext, schema, req)
	return result, err
}

func (p *Store) toInternal(mapper types.Mapper, data map[string]interface{}) {
	if mapper != nil {
		mapper.ToInternal(data)
	}

	if p.group == "" {
		data["apiVersion"] = p.version
	} else {
		data["apiVersion"] = p.group + "/" + p.version
	}
	data["kind"] = p.kind
}

func (p *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	k8sClient, err := p.k8sClient(apiContext)
	if err != nil {
		return nil, err
	}

	namespace, id := splitID(id)
	req := p.common(namespace, k8sClient.Get()).
		Name(id)

	resourceVersion, existing, err := p.singleResultRaw(apiContext, schema, req)
	if err != nil {
		return data, nil
	}

	p.toInternal(schema.Mapper, data)
	existing = merge.APIUpdateMerge(schema.InternalSchema, apiContext.Schemas, existing, data, apiContext.Query.Get("_replace") == "true")

	values.PutValue(existing, resourceVersion, "metadata", "resourceVersion")
	values.PutValue(existing, namespace, "metadata", "namespace")
	values.PutValue(existing, id, "metadata", "name")

	req = p.common(namespace, k8sClient.Put()).
		Body(&unstructured.Unstructured{
			Object: existing,
		}).
		Name(id)

	_, result, err := p.singleResult(apiContext, schema, req)
	return result, err
}

func (p *Store) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	k8sClient, err := p.k8sClient(apiContext)
	if err != nil {
		return nil, err
	}

	namespace, name := splitID(id)

	prop := metav1.DeletePropagationForeground
	req := p.common(namespace, k8sClient.Delete()).
		Body(&metav1.DeleteOptions{
			PropagationPolicy: &prop,
		}).
		Name(name)

	err = p.doAuthed(apiContext, req).Error()
	if err != nil {
		return nil, err
	}

	obj, err := p.ByID(apiContext, schema, id)
	if err != nil {
		return nil, nil
	}
	return obj, nil
}

func (p *Store) singleResult(apiContext *types.APIContext, schema *types.Schema, req *rest.Request) (string, map[string]interface{}, error) {
	version, data, err := p.singleResultRaw(apiContext, schema, req)
	if err != nil {
		return "", nil, err
	}
	p.fromInternal(schema, data)
	return version, data, nil
}

func (p *Store) singleResultRaw(apiContext *types.APIContext, schema *types.Schema, req *rest.Request) (string, map[string]interface{}, error) {
	result := &unstructured.Unstructured{}
	err := p.doAuthed(apiContext, req).Into(result)
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

func (p *Store) common(namespace string, req *rest.Request) *rest.Request {
	prefix := append([]string{}, p.prefix...)
	if p.group != "" {
		prefix = append(prefix, p.group)
	}
	prefix = append(prefix, p.version)
	req.Prefix(prefix...).
		Resource(p.resourcePlural)

	if namespace != "" {
		req.Namespace(namespace)
	}

	return req
}

func (p *Store) fromInternal(schema *types.Schema, data map[string]interface{}) map[string]interface{} {
	if schema.Mapper != nil {
		schema.Mapper.FromInternal(data)
	}

	return data
}
