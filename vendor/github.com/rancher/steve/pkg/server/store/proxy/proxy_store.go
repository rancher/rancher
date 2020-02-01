package proxy

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"regexp"

	"github.com/pkg/errors"
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
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
)

var (
	lowerChars = regexp.MustCompile("[a-z]+")
)

type ClientGetter interface {
	Client(ctx *types.APIRequest, schema *types.APISchema, namespace string) (dynamic.ResourceInterface, error)
	ClientForWatch(ctx *types.APIRequest, schema *types.APISchema, namespace string) (dynamic.ResourceInterface, error)
}

type Store struct {
	clientGetter ClientGetter
}

func NewProxyStore(clientGetter ClientGetter) types.Store {
	return &errorStore{
		Store: &Store{
			clientGetter: clientGetter,
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
	if obj == nil {
		return types.APIObject{}
	}

	gvr := attributes.GVR(schema)

	t := fmt.Sprintf("%s/%s/%s", gvr.Group, gvr.Version, gvr.Resource)
	apiObject := types.APIObject{
		Type:   t,
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
	k8sClient, err := s.clientGetter.Client(apiOp, schema, apiOp.Namespace)
	if err != nil {
		return nil, err
	}

	opts := metav1.GetOptions{}
	if err := decodeParams(apiOp, &opts); err != nil {
		return nil, err
	}

	return k8sClient.Get(id, opts)
}

func (s *Store) List(apiOp *types.APIRequest, schema *types.APISchema) (types.APIObjectList, error) {
	k8sClient, err := s.clientGetter.Client(apiOp, schema, apiOp.Namespace)
	if err != nil {
		return types.APIObjectList{}, err
	}

	opts := metav1.ListOptions{}
	if err := decodeParams(apiOp, &opts); err != nil {
		return types.APIObjectList{}, nil
	}

	resultList, err := k8sClient.List(opts)
	if err != nil {
		return types.APIObjectList{}, err
	}

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
		list, err := k8sClient.List(metav1.ListOptions{
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
	watcher, err := k8sClient.Watch(metav1.ListOptions{
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

func (s *Store) Watch(apiOp *types.APIRequest, schema *types.APISchema, w types.WatchRequest) (chan types.APIEvent, error) {
	k8sClient, err := s.clientGetter.ClientForWatch(apiOp, schema, apiOp.Namespace)
	if err != nil {
		return nil, err
	}

	result := make(chan types.APIEvent)
	go func() {
		s.listAndWatch(apiOp, k8sClient, schema, w, result)
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

	gvk := attributes.GVK(schema)
	input["apiVersion"], input["kind"] = gvk.ToAPIVersionAndKind()

	k8sClient, err := s.clientGetter.Client(apiOp, schema, ns)
	if err != nil {
		return types.APIObject{}, err
	}

	opts := metav1.CreateOptions{}
	if err := decodeParams(apiOp, &opts); err != nil {
		return types.APIObject{}, err
	}

	resp, err = k8sClient.Create(&unstructured.Unstructured{Object: input}, opts)
	return toAPI(schema, resp), err
}

func (s *Store) Update(apiOp *types.APIRequest, schema *types.APISchema, params types.APIObject, id string) (types.APIObject, error) {
	var (
		err   error
		input = params.Data()
	)

	ns := types.Namespace(input)
	k8sClient, err := s.clientGetter.Client(apiOp, schema, ns)
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

		resp, err := k8sClient.Patch(id, pType, bytes, opts)
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

	resp, err := k8sClient.Update(&unstructured.Unstructured{Object: input}, metav1.UpdateOptions{})
	if err != nil {
		return types.APIObject{}, err
	}

	return toAPI(schema, resp), nil
}

func (s *Store) Delete(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	opts := metav1.DeleteOptions{}
	if err := decodeParams(apiOp, &opts); err != nil {
		return types.APIObject{}, nil
	}

	k8sClient, err := s.clientGetter.Client(apiOp, schema, apiOp.Namespace)
	if err != nil {
		return types.APIObject{}, err
	}

	if err := k8sClient.Delete(id, &opts); err != nil {
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
