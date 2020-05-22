package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"regexp"

	"github.com/pkg/errors"
	"github.com/rancher/steve/pkg/accesscontrol"
	"github.com/rancher/steve/pkg/attributes"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"github.com/rancher/wrangler/pkg/data"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
)

var (
	lowerChars = regexp.MustCompile("[a-z]+")
)

type ClientGetter interface {
	Client(ctx *types.APIRequest, schema *types.APISchema, namespace string) (dynamic.ResourceInterface, error)
	AdminClient(ctx *types.APIRequest, schema *types.APISchema, namespace string) (dynamic.ResourceInterface, error)
	TableClient(ctx *types.APIRequest, schema *types.APISchema, namespace string) (dynamic.ResourceInterface, error)
	TableAdminClient(ctx *types.APIRequest, schema *types.APISchema, namespace string) (dynamic.ResourceInterface, error)
	TableClientForWatch(ctx *types.APIRequest, schema *types.APISchema, namespace string) (dynamic.ResourceInterface, error)
	TableAdminClientForWatch(ctx *types.APIRequest, schema *types.APISchema, namespace string) (dynamic.ResourceInterface, error)
}

type Store struct {
	clientGetter ClientGetter
}

func NewProxyStore(clientGetter ClientGetter, lookup accesscontrol.AccessSetLookup) types.Store {
	return &errorStore{
		Store: &WatchRefresh{
			Store: &RBACStore{
				Store: &Store{
					clientGetter: clientGetter,
				},
			},
			asl: lookup,
		},
	}
}

func (s *Store) ByID(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	result, err := s.byID(apiOp, schema, id)
	return toAPI(schema, result), err
}

func decodeParams(apiOp *types.APIRequest, target runtime.Object) error {
	return metav1.ParameterCodec.DecodeParameters(apiOp.Request.URL.Query(), metav1.SchemeGroupVersion, target)
}

func toAPI(schema *types.APISchema, obj runtime.Object) types.APIObject {
	if obj == nil || reflect.ValueOf(obj).IsNil() {
		return types.APIObject{}
	}

	if unstr, ok := obj.(*unstructured.Unstructured); ok {
		obj = moveToUnderscore(unstr)
	}

	apiObject := types.APIObject{
		Type:   schema.ID,
		Object: obj,
	}

	m, err := meta.Accessor(obj)
	if err != nil {
		return apiObject
	}

	id := m.GetName()
	ns := m.GetNamespace()
	if ns != "" {
		id = fmt.Sprintf("%s/%s", ns, id)
	}

	apiObject.ID = id
	return apiObject
}

func (s *Store) byID(apiOp *types.APIRequest, schema *types.APISchema, id string) (*unstructured.Unstructured, error) {
	k8sClient, err := s.clientGetter.TableClient(apiOp, schema, apiOp.Namespace)
	if err != nil {
		return nil, err
	}

	opts := metav1.GetOptions{}
	if err := decodeParams(apiOp, &opts); err != nil {
		return nil, err
	}

	obj, err := k8sClient.Get(apiOp.Context(), id, opts)
	rowToObject(obj)
	return obj, err
}

func moveFromUnderscore(obj map[string]interface{}) map[string]interface{} {
	if obj == nil {
		return nil
	}
	for k := range types.ReservedFields {
		v, ok := obj["_"+k]
		delete(obj, "_"+k)
		delete(obj, k)
		if ok {
			obj[k] = v
		}
	}
	return obj
}

func moveToUnderscore(obj *unstructured.Unstructured) *unstructured.Unstructured {
	if obj == nil {
		return nil
	}

	for k := range types.ReservedFields {
		v, ok := obj.Object[k]
		if ok {
			delete(obj.Object, k)
			obj.Object["_"+k] = v
		}
	}

	return obj
}

func rowToObject(obj *unstructured.Unstructured) {
	if obj == nil {
		return
	}
	if obj.Object["kind"] != "Table" ||
		obj.Object["apiVersion"] != "meta.k8s.io/v1" {
		return
	}

	items := tableToObjects(obj.Object)
	if len(items) == 1 {
		obj.Object = items[0].Object
	}
}

func tableToList(obj *unstructured.UnstructuredList) {
	if obj.Object["kind"] != "Table" ||
		obj.Object["apiVersion"] != "meta.k8s.io/v1" {
		return
	}

	obj.Items = tableToObjects(obj.Object)
}

func tableToObjects(obj map[string]interface{}) []unstructured.Unstructured {
	var result []unstructured.Unstructured

	rows, _ := obj["rows"].([]interface{})
	for _, row := range rows {
		m, ok := row.(map[string]interface{})
		if !ok {
			continue
		}
		cells := m["cells"]
		object, ok := m["object"].(map[string]interface{})
		if !ok {
			continue
		}

		data.PutValue(object, cells, "metadata", "fields")
		result = append(result, unstructured.Unstructured{
			Object: object,
		})
	}

	return result
}

func (s *Store) ByNames(apiOp *types.APIRequest, schema *types.APISchema, names sets.String) (types.APIObjectList, error) {
	adminClient, err := s.clientGetter.TableAdminClient(apiOp, schema, apiOp.Namespace)
	if err != nil {
		return types.APIObjectList{}, err
	}

	objs, err := s.list(apiOp, schema, adminClient)
	if err != nil {
		return types.APIObjectList{}, err
	}

	var filtered []types.APIObject
	for _, obj := range objs.Objects {
		if names.Has(obj.Name()) {
			filtered = append(filtered, obj)
		}
	}

	objs.Objects = filtered
	return objs, nil
}

func (s *Store) List(apiOp *types.APIRequest, schema *types.APISchema) (types.APIObjectList, error) {
	client, err := s.clientGetter.TableClient(apiOp, schema, apiOp.Namespace)
	if err != nil {
		return types.APIObjectList{}, err
	}
	return s.list(apiOp, schema, client)
}

func (s *Store) list(apiOp *types.APIRequest, schema *types.APISchema, client dynamic.ResourceInterface) (types.APIObjectList, error) {
	opts := metav1.ListOptions{}
	if err := decodeParams(apiOp, &opts); err != nil {
		return types.APIObjectList{}, nil
	}

	resultList, err := client.List(apiOp.Context(), opts)
	if err != nil {
		return types.APIObjectList{}, err
	}

	tableToList(resultList)

	result := types.APIObjectList{
		Revision: resultList.GetResourceVersion(),
		Continue: resultList.GetContinue(),
	}

	for i := range resultList.Items {
		result.Objects = append(result.Objects, toAPI(schema, &resultList.Items[i]))
	}

	return result, nil
}

func returnErr(err error, c chan types.APIEvent) {
	c <- types.APIEvent{
		Name:  "resource.error",
		Error: err,
	}
}

func (s *Store) listAndWatch(apiOp *types.APIRequest, k8sClient dynamic.ResourceInterface, schema *types.APISchema, w types.WatchRequest, result chan types.APIEvent) {
	rev := w.Revision
	if rev == "" {
		list, err := k8sClient.List(apiOp.Context(), metav1.ListOptions{
			Limit: 1,
		})
		if err != nil {
			returnErr(errors.Wrapf(err, "failed to list %s", schema.ID), result)
			return
		}
		rev = list.GetResourceVersion()
	} else if rev == "-1" {
		rev = ""
	}

	timeout := int64(60 * 30)
	watcher, err := k8sClient.Watch(apiOp.Context(), metav1.ListOptions{
		Watch:           true,
		TimeoutSeconds:  &timeout,
		ResourceVersion: rev,
	})
	if err != nil {
		returnErr(errors.Wrapf(err, "stopping watch for %s: %v", schema.ID, err), result)
		return
	}
	defer watcher.Stop()
	logrus.Debugf("opening watcher for %s", schema.ID)

	go func() {
		<-apiOp.Request.Context().Done()
		watcher.Stop()
	}()

	for event := range watcher.ResultChan() {
		if event.Type == watch.Error {
			continue
		}
		result <- s.toAPIEvent(apiOp, schema, event.Type, event.Object)
	}
}

func (s *Store) WatchNames(apiOp *types.APIRequest, schema *types.APISchema, w types.WatchRequest, names sets.String) (chan types.APIEvent, error) {
	adminClient, err := s.clientGetter.TableAdminClientForWatch(apiOp, schema, apiOp.Namespace)
	if err != nil {
		return nil, err
	}
	c, err := s.watch(apiOp, schema, w, adminClient)
	if err != nil {
		return nil, err
	}

	result := make(chan types.APIEvent)
	go func() {
		defer close(result)
		for item := range c {
			if item.Error != nil && names.Has(item.Object.Name()) {
				result <- item
			}
		}
	}()

	return result, nil
}

func (s *Store) Watch(apiOp *types.APIRequest, schema *types.APISchema, w types.WatchRequest) (chan types.APIEvent, error) {
	client, err := s.clientGetter.TableClientForWatch(apiOp, schema, apiOp.Namespace)
	if err != nil {
		return nil, err
	}
	return s.watch(apiOp, schema, w, client)
}

func (s *Store) watch(apiOp *types.APIRequest, schema *types.APISchema, w types.WatchRequest, client dynamic.ResourceInterface) (chan types.APIEvent, error) {
	result := make(chan types.APIEvent)
	go func() {
		s.listAndWatch(apiOp, client, schema, w, result)
		logrus.Debugf("closing watcher for %s", schema.ID)
		close(result)
	}()
	return result, nil
}

func (s *Store) toAPIEvent(apiOp *types.APIRequest, schema *types.APISchema, et watch.EventType, obj runtime.Object) types.APIEvent {
	name := types.ChangeAPIEvent
	switch et {
	case watch.Deleted:
		name = types.RemoveAPIEvent
	case watch.Added:
		name = types.CreateAPIEvent
	}

	if unstr, ok := obj.(*unstructured.Unstructured); ok {
		rowToObject(unstr)
	}

	event := types.APIEvent{
		Name:   name,
		Object: toAPI(schema, obj),
	}

	m, err := meta.Accessor(obj)
	if err != nil {
		return event
	}

	event.Revision = m.GetResourceVersion()
	return event
}

func (s *Store) Create(apiOp *types.APIRequest, schema *types.APISchema, params types.APIObject) (types.APIObject, error) {
	var (
		resp *unstructured.Unstructured
	)

	input := params.Data()

	if input == nil {
		input = data.Object{}
	}

	name := types.Name(input)
	ns := types.Namespace(input)
	if name == "" && input.String("metadata", "generateName") == "" {
		input.SetNested(schema.ID[0:1]+"-", "metadata", "generatedName")
	}
	if ns == "" && apiOp.Namespace != "" {
		ns = apiOp.Namespace
		input.SetNested(ns, "metadata", "namespace")
	}

	gvk := attributes.GVK(schema)
	input["apiVersion"], input["kind"] = gvk.ToAPIVersionAndKind()

	k8sClient, err := s.clientGetter.TableClient(apiOp, schema, ns)
	if err != nil {
		return types.APIObject{}, err
	}

	opts := metav1.CreateOptions{}
	if err := decodeParams(apiOp, &opts); err != nil {
		return types.APIObject{}, err
	}

	resp, err = k8sClient.Create(apiOp.Context(), &unstructured.Unstructured{Object: input}, opts)
	rowToObject(resp)
	return toAPI(schema, resp), err
}

func (s *Store) Update(apiOp *types.APIRequest, schema *types.APISchema, params types.APIObject, id string) (types.APIObject, error) {
	var (
		err   error
		input = params.Data()
	)

	ns := types.Namespace(input)
	k8sClient, err := s.clientGetter.TableClient(apiOp, schema, ns)
	if err != nil {
		return types.APIObject{}, err
	}

	if apiOp.Method == http.MethodPatch {
		bytes, err := ioutil.ReadAll(io.LimitReader(apiOp.Request.Body, 2<<20))
		if err != nil {
			return types.APIObject{}, err
		}

		pType := apitypes.StrategicMergePatchType
		if apiOp.Request.Header.Get("content-type") == string(apitypes.JSONPatchType) {
			pType = apitypes.JSONPatchType
		}

		opts := metav1.PatchOptions{}
		if err := decodeParams(apiOp, &opts); err != nil {
			return types.APIObject{}, err
		}

		if pType == apitypes.StrategicMergePatchType {
			data := map[string]interface{}{}
			if err := json.Unmarshal(bytes, &data); err != nil {
				return types.APIObject{}, err
			}
			data = moveFromUnderscore(data)
			bytes, err = json.Marshal(data)
			if err != nil {
				return types.APIObject{}, err
			}
		}

		resp, err := k8sClient.Patch(apiOp.Context(), id, pType, bytes, opts)
		if err != nil {
			return types.APIObject{}, err
		}

		return toAPI(schema, resp), nil
	}

	resourceVersion := input.String("metadata", "resourceVersion")
	if resourceVersion == "" {
		return types.APIObject{}, fmt.Errorf("metadata.resourceVersion is required for update")
	}

	opts := metav1.UpdateOptions{}
	if err := decodeParams(apiOp, &opts); err != nil {
		return types.APIObject{}, err
	}

	resp, err := k8sClient.Update(apiOp.Context(), &unstructured.Unstructured{Object: moveFromUnderscore(input)}, metav1.UpdateOptions{})
	if err != nil {
		return types.APIObject{}, err
	}

	rowToObject(resp)
	return toAPI(schema, resp), nil
}

func (s *Store) Delete(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	opts := metav1.DeleteOptions{}
	if err := decodeParams(apiOp, &opts); err != nil {
		return types.APIObject{}, nil
	}

	k8sClient, err := s.clientGetter.TableClient(apiOp, schema, apiOp.Namespace)
	if err != nil {
		return types.APIObject{}, err
	}

	if err := k8sClient.Delete(apiOp.Context(), id, opts); err != nil {
		return types.APIObject{}, err
	}

	obj, err := s.byID(apiOp, schema, id)
	if err != nil {
		// ignore lookup error
		return types.APIObject{}, validation.ErrorCode{
			Status: http.StatusNoContent,
		}
	}
	return toAPI(schema, obj), nil
}
