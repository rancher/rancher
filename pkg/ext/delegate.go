package ext

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"

	"agones.dev/agones/pkg/util/https"
	agonesRuntime "agones.dev/agones/pkg/util/runtime"
	"github.com/emicklei/go-restful/v3"
	"github.com/rancher/rancher/pkg/ext/resources/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metainternalversionscheme "k8s.io/apimachinery/pkg/apis/meta/internalversion/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"

	"k8s.io/apiserver/pkg/authentication/user"
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
	GroupVersion       schema.GroupVersion
	requestInfoFactory request.RequestInfoFactory
}

func NewStoreDelegate[
	T Ptr[DerefT],
	DerefT any,
	TList runtime.Object,
](store types.Store[T, TList], groupVersion schema.GroupVersion) StoreDelegate[T, DerefT, TList] {
	return StoreDelegate[T, DerefT, TList]{
		Store:              store,
		GroupVersion:       groupVersion,
		requestInfoFactory: request.RequestInfoFactory{APIPrefixes: sets.NewString("apis", "api"), GrouplessAPIPrefixes: sets.NewString("api")},
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
	ws.Path(fmt.Sprintf("/apis/%s/%s", s.GroupVersion.Group, s.GroupVersion.Version))
	// TODO: Missing deletecollection
	ws.Route(
		ws.GET(path).
			To(noop).
			Operation("list").
			AddExtension("x-kubernetes-action", "list").
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
			AddExtension("x-kubernetes-action", "post").
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
			AddExtension("x-kubernetes-action", "get").
			Doc(fmt.Sprintf("get objects of kind %T", t)).
			Consumes(restful.MIME_JSON).
			Produces(restful.MIME_JSON).
			Returns(200, "OK", t),
	)
	ws.Route(
		ws.PUT(pathWithNameParam).
			To(noop).
			Operation("replace").
			AddExtension("x-kubernetes-action", "put").
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
			AddExtension("x-kubernetes-action", "delete").
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
			AddExtension("x-kubernetes-action", "patch").
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

	// TODO: Remove once we do auth
	ctx := request.WithUser(req.Context(), &user.DefaultInfo{
		Name:   "admin",
		Groups: []string{"system:masters"},
	})

	logger := agonesRuntime.NewLoggerWithType(namespace)
	https.LogRequest(logger, req).Info("RancherTokens")
	// other middleware is expected to filter out unauthenticated requests
	userInfo, _ := request.UserFrom(ctx)

	switch req.Method {
	case http.MethodDelete:
		name, _, err := s.resourceNameAndNamespace(req)
		if err != nil {
			return err
		}
		err = s.Store.Delete(userInfo, name)
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
			resource, err := s.Store.Get(userInfo, name)
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

		if req.URL.Query().Get("watch") == "true" {
			resultCh, err := s.Store.Watch(userInfo, listOptions)
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

		resources, err := s.Store.List(userInfo)
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
		accessor := meta.NewAccessor()
		name, err := accessor.Name(resource)
		if err != nil {
			return err
		}
		var retResource T
		_, err = s.Store.Get(userInfo, name)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
			retResource, err = s.Store.Create(userInfo, resource)
		} else {
			retResource, err = s.Store.Update(userInfo, resource)
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

		retResource, err := s.Store.Create(userInfo, resource)
		if err != nil {
			return err
		}
		return s.writeOkResponse(w, req, retResource)
	case http.MethodPatch:
		if req.Header.Get("Content-Type") != "application/merge-patch+json" {
			return fmt.Errorf("unsupported patch")
		}

		resource, err := s.readObjectFromRequest(req)
		if err != nil {
			return err
		}

		retResource, err := s.Store.Update(userInfo, resource)
		if err != nil {
			return err
		}
		return s.writeOkResponse(w, req, retResource)
	default:
		return fmt.Errorf("unsupported request")
	}
}

func (s *StoreDelegate[T, DerefT, TList]) writeOkResponse(w http.ResponseWriter, req *http.Request, obj runtime.Object) error {
	info, err := AcceptedSerializer(req, Codecs)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", info.MediaType)
	w.WriteHeader(http.StatusOK)
	if obj != nil {
		err = Codecs.EncoderForVersion(info.Serializer, s.GroupVersion).Encode(obj, w)
	}
	if err != nil {
		return err
	}
	return nil
}

func (s *StoreDelegate[T, DerefT, TList]) readObjectFromRequest(req *http.Request) (T, error) {
	var resource T = new(DerefT)

	bytes, err := io.ReadAll(req.Body)
	if err != nil {
		return resource, err
	}

	_, _, err = Codecs.UniversalDecoder(s.GroupVersion).Decode(bytes, nil, resource)
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
