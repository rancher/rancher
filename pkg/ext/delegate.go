package ext

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"

	"github.com/emicklei/go-restful/v3"
	"github.com/rancher/rancher/pkg/ext/resources/types"
	jsonpatch "gopkg.in/evanphx/json-patch.v4"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metainternalversionscheme "k8s.io/apimachinery/pkg/apis/meta/internalversion/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/endpoints/handlers/negotiation"
	"k8s.io/apiserver/pkg/endpoints/request"
)

// Ptr[U] acts as a type constraint such that
//
//	T Ptr[U]
//
// means that T is a pointer to U and a runtime.Object.
type Ptr[U any] interface {
	*U
	runtime.Object
}

// Note: We need both T and DerefT. T because that's the object we're interested
//       in (runtime.Object). DerefT because we want to instantiate T objects.

type StoreDelegate[
	T Ptr[DerefT],
	DerefT any,
	TList runtime.Object,
] struct {
	Store              types.Store[T, TList]
	GroupVersionKind   schema.GroupVersionKind
	requestInfoFactory request.RequestInfoFactory

	codecFactory serializer.CodecFactory
}

func NewStoreDelegate[
	T Ptr[DerefT],
	DerefT any,
	TList runtime.Object,
](store types.Store[T, TList], gvk schema.GroupVersionKind) StoreDelegate[T, DerefT, TList] {
	return StoreDelegate[T, DerefT, TList]{
		Store:              store,
		GroupVersionKind:   gvk,
		requestInfoFactory: request.RequestInfoFactory{APIPrefixes: sets.NewString("apis", "api"), GrouplessAPIPrefixes: sets.NewString("api")},

		codecFactory: Codecs,
	}
}

func (s *StoreDelegate[T, DerefT, TList]) WebService(resource string, isNamespaced bool) *restful.WebService {
	// WebService builder absolutely want a function with .To()
	noop := func(*restful.Request, *restful.Response) {}

	var t T
	var tList TList

	path := fmt.Sprintf("/%s", resource)
	pathWithNameParam := fmt.Sprintf("/%s/{name}", resource)
	if isNamespaced {
		path = fmt.Sprintf("/namespaces/{namespace}/%s", resource)
		pathWithNameParam = fmt.Sprintf("/namespaces/{namespace}/%s/{name}", resource)
	}

	ws := &restful.WebService{}
	ws.Path(fmt.Sprintf("/apis/%s/%s", s.GroupVersionKind.Group, s.GroupVersionKind.Version))
	// TODO: Missing deletecollection
	ws.Route(
		ws.GET(path).
			To(noop).
			Operation("list").
			Metadata("x-kubernetes-action", "list").
			Metadata("x-kubernetes-group-version-kind", metav1.GroupVersionKind{
				Group:   s.GroupVersionKind.Group,
				Version: s.GroupVersionKind.Version,
				Kind:    s.GroupVersionKind.Kind,
			}).
			Doc(fmt.Sprintf("list or watch objects of kind %T", t)).
			Consumes(restful.MIME_JSON).
			Produces(restful.MIME_JSON).
			Returns(200, "OK", tList),
	)
	ws.Route(
		ws.POST(path).
			To(noop).
			Operation("create").
			Reads(t).
			Metadata("x-kubernetes-action", "post").
			Doc(fmt.Sprintf("create a %T", t)).
			Consumes(restful.MIME_JSON).
			Produces(restful.MIME_JSON).
			Returns(200, "OK", tList).
			Returns(201, "Created", tList).
			Returns(202, "Accepted", tList),
	)
	ws.Route(
		ws.GET(pathWithNameParam).
			To(noop).
			Operation("get").
			Metadata("x-kubernetes-action", "get").
			Metadata("x-kubernetes-group-version-kind", metav1.GroupVersionKind{
				Group:   s.GroupVersionKind.Group,
				Version: s.GroupVersionKind.Version,
				Kind:    s.GroupVersionKind.Kind,
			}).
			Doc(fmt.Sprintf("get objects of kind %T", t)).
			Consumes(restful.MIME_JSON).
			Produces(restful.MIME_JSON).
			Returns(200, "OK", t),
	)
	ws.Route(
		ws.PUT(pathWithNameParam).
			To(noop).
			Operation("replace").
			Metadata("x-kubernetes-action", "put").
			Reads(t).
			Doc(fmt.Sprintf("replace the specified %T", t)).
			Consumes(restful.MIME_JSON).
			Produces(restful.MIME_JSON).
			Returns(200, "OK", t).
			Returns(201, "Created", t),
	)
	ws.Route(
		ws.DELETE(pathWithNameParam).
			To(noop).
			Operation("delete").
			Metadata("x-kubernetes-action", "delete").
			Doc(fmt.Sprintf("delete a %T", t)).
			Produces(restful.MIME_JSON).
			// FIXME: Should be Status
			Returns(200, "OK", tList).
			// FIXME: Should be Status
			Returns(202, "Accepted", tList),
	)
	ws.Route(
		ws.PATCH(pathWithNameParam).
			To(noop).
			Operation("patch").
			Metadata("x-kubernetes-action", "patch").
			Doc(fmt.Sprintf("delete a %T", t)).
			Consumes("application/merge-patch+json").
			Produces(restful.MIME_JSON).
			Returns(200, "OK", t).
			Returns(201, "Created", t),
	)
	return ws
}

func (s *StoreDelegate[T, DerefT, TList]) Delegate(w http.ResponseWriter, req *http.Request, namespace string) error {
	defer func() {
		// XXX: Until https://github.com/rancher/dynamiclistener/pull/118 is fixed
		if r := recover(); r != nil {
			fmt.Println(r)
			fmt.Println("stacktrace from panic: \n" + string(debug.Stack()))
		}
	}()

	ctx := req.Context()

	// other middleware is expected to filter out unauthenticated requests
	userInfo, _ := request.UserFrom(ctx)

	switch req.Method {
	case http.MethodDelete:
		name, _, err := s.resourceNameAndNamespace(req)
		if err != nil {
			return err
		}

		deleteOptions := &metav1.DeleteOptions{}
		err = metainternalversionscheme.ParameterCodec.DecodeParameters(req.URL.Query(), metav1.SchemeGroupVersion, deleteOptions)
		if err != nil {
			return err
		}

		err = s.Store.Delete(ctx, userInfo, name, deleteOptions)
		if err != nil {
			return err
		}

		status := &metav1.Status{
			Status: "Success",
		}
		return s.writeOkResponse(w, req, status)
	case http.MethodGet:
		// XXX: StoreDelegate doesn't support namespaced resources
		name, _, err := s.resourceNameAndNamespace(req)
		if err != nil {
			return err
		}
		if name != "" {
			getOptions := &metav1.GetOptions{}
			err = metainternalversionscheme.ParameterCodec.DecodeParameters(req.URL.Query(), metav1.SchemeGroupVersion, getOptions)
			if err != nil {
				return err
			}
			resource, err := s.Store.Get(ctx, userInfo, name, getOptions)
			if err != nil {
				return err
			}
			return s.writeOkResponse(w, req, resource)
		}

		listOptions := &metav1.ListOptions{}
		err = metainternalversionscheme.ParameterCodec.DecodeParameters(req.URL.Query(), metav1.SchemeGroupVersion, listOptions)
		if err != nil {
			return err
		}

		if listOptions.Watch {
			resultCh, err := s.Store.Watch(ctx, userInfo, listOptions)
			if err != nil {
				return fmt.Errorf("unable to watch: %w", err)
			}

			flusher, ok := w.(http.Flusher)
			if !ok {
				return fmt.Errorf("unable to start watch - can't get http.Flusher: %#v", w)
			}

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Transfer-Encoding", "chunked")
			w.WriteHeader(http.StatusOK)
			flusher.Flush()

			for event := range resultCh {
				objBuffer := runtime.NewSpliceBuffer()
				eventBuffer := runtime.NewSpliceBuffer()
				err = json.NewEncoder(objBuffer).Encode(event.Object)
				if err != nil {
					return fmt.Errorf("unable to encode RancherToken: %w", err)
				}

				outEvent := &metav1.WatchEvent{
					Type:   string(event.Event),
					Object: runtime.RawExtension{Raw: objBuffer.Bytes()},
				}

				err = json.NewEncoder(eventBuffer).Encode(outEvent)
				if err != nil {
					return fmt.Errorf("unable to encode WatchEvent: %w", err)
				}

				_, err = w.Write(eventBuffer.Bytes())
				if err != nil {
					return err
				}
				flusher.Flush()
			}

			return nil
		}

		resources, err := s.Store.List(ctx, userInfo, listOptions)
		if err != nil {
			return err
		}

		return s.writeOkResponse(w, req, resources)
	case http.MethodPut:
		// like k8s, internally we classify PUT as a create or update based on the existence of the object
		resource, err := s.readObjectFromRequest(req)
		if err != nil {
			return err
		}

		updateOptions := &metav1.UpdateOptions{}
		err = metainternalversionscheme.ParameterCodec.DecodeParameters(req.URL.Query(), metav1.SchemeGroupVersion, updateOptions)
		if err != nil {
			return err
		}

		accessor := meta.NewAccessor()
		name, err := accessor.Name(resource)
		if err != nil {
			return err
		}

		var retResource T
		// XXX: What GetOptions to give here?
		_, err = s.Store.Get(ctx, userInfo, name, &metav1.GetOptions{})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
			retResource, err = s.Store.Create(ctx, userInfo, resource, newCreateOptionsFromUpdateOptions(updateOptions))
		} else {
			retResource, err = s.Store.Update(ctx, userInfo, resource, updateOptions)
		}
		if err != nil {
			return err
		}
		return s.writeOkResponse(w, req, retResource)
	case http.MethodPost:
		resource, err := s.readObjectFromRequest(req)
		if err != nil {
			return err
		}

		createOptions := &metav1.CreateOptions{}
		err = metainternalversionscheme.ParameterCodec.DecodeParameters(req.URL.Query(), metav1.SchemeGroupVersion, createOptions)
		if err != nil {
			return err
		}

		retResource, err := s.Store.Create(ctx, userInfo, resource, createOptions)
		if err != nil {
			return err
		}
		return s.writeOkResponse(w, req, retResource)
	case http.MethodPatch:
		if req.Header.Get("Content-Type") != "application/merge-patch+json" {
			return fmt.Errorf("unsupported patch")
		}

		name, _, err := s.resourceNameAndNamespace(req)
		if err != nil {
			return err
		}

		patchBytes, err := io.ReadAll(req.Body)
		if err != nil {
			return err
		}

		resource, err := s.Store.Get(ctx, userInfo, name, &metav1.GetOptions{})
		if err != nil {
			return err
		}

		versionedObj, err := json.Marshal(resource)
		if err != nil {
			return err
		}

		patchedObj, err := jsonpatch.MergePatch(versionedObj, patchBytes)
		if err != nil {
			return err
		}

		var newResource DerefT
		err = json.Unmarshal(patchedObj, &newResource)
		if err != nil {
			return err
		}

		// XXX: Should this be metav1.PatchOptions? Looking at upstream, it seems it maps PatchOptions to UpdateOptions or to CreateOptions
		updateOptions := &metav1.UpdateOptions{}
		err = metainternalversionscheme.ParameterCodec.DecodeParameters(req.URL.Query(), metav1.SchemeGroupVersion, updateOptions)
		if err != nil {
			return err
		}

		retResource, err := s.Store.Update(ctx, userInfo, &newResource, updateOptions)
		if err != nil {
			return err
		}
		return s.writeOkResponse(w, req, retResource)
	default:
		return fmt.Errorf("unsupported request")
	}
}

// TODO: Move to responsewriters.WriteObjectNegotiated
func (s *StoreDelegate[T, DerefT, TList]) writeOkResponse(w http.ResponseWriter, req *http.Request, obj runtime.Object) error {
	opts, info, err := negotiation.NegotiateOutputMediaType(req, s.codecFactory, s)
	if err != nil {
		return err
	}

	w.Header().Set(ContentTypeHeader, info.MediaType)
	w.WriteHeader(http.StatusOK)
	if obj != nil {
		result, err := s.applyConversion(req, obj, opts.Convert)
		if err != nil {
			return err
		}

		err = info.Serializer.Encode(result, w)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *StoreDelegate[T, DerefT, TList]) applyConversion(req *http.Request, obj runtime.Object, target *schema.GroupVersionKind) (runtime.Object, error) {
	objList, ok := obj.(TList)
	if !ok {
		return obj, nil
	}

	obj.GetObjectKind().SetGroupVersionKind(s.GroupVersionKind)
	if target == nil {
		return obj, nil
	}

	convertor, ok := s.Store.(types.TableConvertor[TList])
	if !ok {
		// Shouldn't happen
		return obj, nil
	}

	opts := &metav1.TableOptions{}
	if err := metainternalversionscheme.ParameterCodec.DecodeParameters(req.URL.Query(), metav1.SchemeGroupVersion, opts); err != nil {
		return nil, err
	}
	return convertor.ConvertToTable(objList, opts), nil
}

func (s *StoreDelegate[T, DerefT, TList]) readObjectFromRequest(req *http.Request) (T, error) {
	info, err := negotiation.NegotiateInputSerializer(req, false, s.codecFactory)
	if err != nil {
		return nil, err
	}

	var resource T = new(DerefT)

	bytes, err := io.ReadAll(req.Body)
	if err != nil {
		return resource, err
	}

	_, _, err = info.Serializer.Decode(bytes, &s.GroupVersionKind, resource)
	return resource, err
}

// resourceNameAndNamespace returns the name and namespace of a resource (in that order) according to the
// url path
func (s *StoreDelegate[T, DerefT, TList]) resourceNameAndNamespace(req *http.Request) (string, string, error) {
	info, err := s.requestInfoFactory.NewRequestInfo(req)
	if err != nil {
		return "", "", err
	}
	return info.Name, info.Namespace, nil
}

// AllowsMediaTypeTransform returns true if the endpoint allows either the requested mime type
// or the requested transformation. If false, the caller should ignore this mime type. If the
// target is nil, the client is not requesting a transformation.
//
// Implements negotiation.EndpointRestrictions
func (s *StoreDelegate[T, DerefT, TList]) AllowsMediaTypeTransform(mimeType, mimeSubType string, target *schema.GroupVersionKind) bool {
	if mimeSubType != "json" && mimeSubType != "yaml" {
		return false
	}

	if target != nil {
		gvk := *target
		if gvk != metav1.SchemeGroupVersion.WithKind("Table") {
			return false
		}

		if _, ok := s.Store.(types.TableConvertor[TList]); !ok {
			return false
		}
	}

	return true
}

// AllowsServerVersion should return true if the specified version is valid
// for the server group.
//
// Implements negotiation.EndpointRestrictions
func (s *StoreDelegate[T, DerefT, TList]) AllowsServerVersion(version string) bool {
	return true
}

// AllowsStreamSchema should return true if the specified stream schema is
// valid for the server group.
//
// Implements negotiation.EndpointRestrictions
func (s *StoreDelegate[T, DerefT, TList]) AllowsStreamSchema(schema string) bool {
	return true
}

// copied from k8s.io/apiserver/pkg/registry/generic/registry/store.go
func newCreateOptionsFromUpdateOptions(in *metav1.UpdateOptions) *metav1.CreateOptions {
	co := &metav1.CreateOptions{
		DryRun:          in.DryRun,
		FieldManager:    in.FieldManager,
		FieldValidation: in.FieldValidation,
	}
	co.TypeMeta.SetGroupVersionKind(metav1.SchemeGroupVersion.WithKind("CreateOptions"))
	return co
}
