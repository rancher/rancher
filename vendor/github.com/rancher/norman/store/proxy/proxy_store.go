package proxy

import (
	ejson "encoding/json"
	"net/http"
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
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

type Store struct {
	k8sClient      rest.Interface
	prefix         []string
	group          string
	version        string
	kind           string
	resourcePlural string
	authContext    map[string]string
}

func NewProxyStore(k8sClient rest.Interface,
	prefix []string, group, version, kind, resourcePlural string) types.Store {
	return &errorStore{
		Store: &Store{
			k8sClient:      k8sClient,
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

func NewRawProxyStore(k8sClient rest.Interface,
	prefix []string, group, version, kind, resourcePlural string) *Store {
	return &Store{
		k8sClient:      k8sClient,
		prefix:         prefix,
		group:          group,
		version:        version,
		kind:           kind,
		resourcePlural: resourcePlural,
		authContext: map[string]string{
			"apiGroup": group,
			"resource": resourcePlural,
		},
	}
}

func (p *Store) getUser(apiContext *types.APIContext) string {
	return apiContext.Request.Header.Get(userAuthHeader)
}

func (p *Store) doAuthed(apiContext *types.APIContext, request *rest.Request) rest.Result {
	for _, header := range authHeaders {
		request.SetHeader(header, apiContext.Request.Header[http.CanonicalHeaderKey(header)]...)
	}
	return request.Do()
}

func (p *Store) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	_, result, err := p.byID(apiContext, schema, id)
	return result, err
}

func (p *Store) byID(apiContext *types.APIContext, schema *types.Schema, id string) (string, map[string]interface{}, error) {
	namespace, id := splitID(id)

	req := p.common(namespace, p.k8sClient.Get()).
		Name(id)

	return p.singleResult(apiContext, schema, req)
}

func (p *Store) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	namespace := getNamespace(apiContext, opt)

	if !strings.EqualFold(schema.ID, p.kind) {
		opt.Conditions = append(opt.Conditions, types.NewConditionFromString("type", types.ModifierEQ, schema.ID))
	}

	req := p.common(namespace, p.k8sClient.Get())

	resultList := &unstructured.UnstructuredList{}
	err := req.Do().Into(resultList)
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}

	for _, obj := range resultList.Items {
		result = append(result, p.fromInternal(schema, apiContext, obj.Object))
	}

	return apiContext.AccessControl.FilterList(apiContext, result, p.authContext), nil
}

func (p *Store) Watch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	namespace := getNamespace(apiContext, opt)

	if !strings.EqualFold(schema.ID, p.kind) {
		opt.Conditions = append(opt.Conditions, types.NewConditionFromString("type", types.ModifierEQ, schema.ID))
	}

	timeout := int64(60 * 60)
	req := p.common(namespace, p.k8sClient.Get())
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
		watcher.Stop()
	}()

	result := make(chan map[string]interface{})
	go func() {
		for event := range watcher.ResultChan() {
			data := event.Object.(*unstructured.Unstructured)
			p.fromInternal(schema, apiContext, data.Object)
			if event.Type == watch.Deleted && data.Object != nil {
				data.Object[".removed"] = true
			}
			result <- apiContext.AccessControl.Filter(apiContext, data.Object, p.authContext)
		}
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
	p.toInternal(schema.Mapper, schema, data)

	values.PutValue(data, p.getUser(apiContext), "metadata", "annotations", "field.cattle.io/creatorId")

	name, _ := values.GetValueN(data, "metadata", "name").(string)
	if name == "" {
		generated, _ := values.GetValueN(data, "metadata", "generateName").(string)
		if generated == "" {
			values.PutValue(data, strings.ToLower(schema.ID+"-"), "metadata", "generateName")
		}
	}

	req := p.common(namespace, p.k8sClient.Post()).
		Body(&unstructured.Unstructured{
			Object: data,
		})

	_, result, err := p.singleResult(apiContext, schema, req)
	return result, err
}

func (p *Store) toInternal(mapper types.Mapper, schema *types.Schema, data map[string]interface{}) {
	if mapper != nil {
		mapper.ToInternal(data)
	}

	if p.group == "" {
		data["apiVersion"] = p.version
	} else {
		data["apiVersion"] = p.group + "/" + p.version
	}
	data["kind"] = p.kind
	if !strings.EqualFold(schema.ID, p.kind) {
		data["type"] = schema.ID
	}
}

func (p *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	namespace, id := splitID(id)
	req := p.common(namespace, p.k8sClient.Get()).
		Name(id)

	resourceVersion, existing, err := p.singleResultRaw(apiContext, schema, req)
	if err != nil {
		return data, nil
	}

	p.toInternal(schema.Mapper, schema, data)
	existing = convert.APIUpdateMerge(existing, data, apiContext.Query.Get("_replace") == "true")

	values.PutValue(existing, resourceVersion, "metadata", "resourceVersion")
	values.PutValue(existing, namespace, "metadata", "namespace")
	values.PutValue(existing, id, "metadata", "name")

	req = p.common(namespace, p.k8sClient.Put()).
		Body(&unstructured.Unstructured{
			Object: existing,
		}).
		Name(id)

	_, result, err := p.singleResult(apiContext, schema, req)
	return result, err
}

func (p *Store) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	namespace, id := splitID(id)

	prop := metav1.DeletePropagationForeground
	req := p.common(namespace, p.k8sClient.Delete()).
		Body(&metav1.DeleteOptions{
			PropagationPolicy: &prop,
		}).
		Name(id)

	err := p.doAuthed(apiContext, req).Error()
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
	p.fromInternal(schema, apiContext, data)
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

func (p *Store) fromInternal(schema *types.Schema, apiContext *types.APIContext, data map[string]interface{}) map[string]interface{} {
	mapperSchema := schema
	// if type on resource doesn't match schema.Type, the resource is a subtype and the provided schema is for the baseType
	// get the mapper for the actual subtype schema
	if resourceType, ok := data["type"].(string); ok && resourceType != schema.ID {
		if s := apiContext.Schemas.Schema(apiContext.Version, resourceType); s != nil && s.BaseType == schema.ID {
			mapperSchema = s
		}
	}

	if mapperSchema.Mapper != nil {
		mapperSchema.Mapper.FromInternal(data)
	}

	return data
}
