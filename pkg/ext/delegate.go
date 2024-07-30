package ext

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"agones.dev/agones/pkg/util/https"
	agonesRuntime "agones.dev/agones/pkg/util/runtime"
	"github.com/rancher/rancher/pkg/ext/resources/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type StoreDelegate[T runtime.Object] struct {
	Store        types.Store[T]
	GroupVersion schema.GroupVersion
}

func (s *StoreDelegate[T]) Delegate(w http.ResponseWriter, req *http.Request, namespace string) error {
	logger := agonesRuntime.NewLoggerWithType(namespace)
	https.LogRequest(logger, req).Info("RancherTokens")

	switch req.Method {
	case http.MethodDelete:
		fields := strings.Split(req.URL.Path, "/")
		resourceName := fields[len(fields)-1]
		err := s.Store.Delete(resourceName)
		if err != nil {
			return err
		}
		status := &metav1.Status{
			Status: "Success",
		}
		return s.writeOkResponse(w, req, status)
	case http.MethodGet:
		fields := strings.Split(req.URL.Path, "/")
		resourceName := fields[len(fields)-1]

		resource, err := s.Store.Get(resourceName)
		if err != nil {
			return err
		}
		return s.writeOkResponse(w, req, resource)
	case http.MethodPost:
		// note that this assumes post == create
		resource, err := s.readObjectFromRequest(req)
		if err != nil {
			return err
		}

		retResource, err := s.Store.Create(resource)

		return s.writeOkResponse(w, req, retResource)
	case http.MethodPatch:
		if req.Header.Get("Content-Type") != "application/merge-patch+json" {
			return fmt.Errorf("unsupported patch")
		}

		resource, err := s.readObjectFromRequest(req)
		if err != nil {
			return err
		}

		retResource, err := s.Store.Update(resource)
		if err != nil {
			return err
		}
		return s.writeOkResponse(w, req, retResource)
	default:
		return fmt.Errorf("unsupported request")
	}
}

func (s *StoreDelegate[T]) writeOkResponse(w http.ResponseWriter, req *http.Request, obj runtime.Object) error {
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

func (s *StoreDelegate[T]) readObjectFromRequest(req *http.Request) (T, error) {
	var resource T
	bytes, err := io.ReadAll(req.Body)
	if err != nil {
		return resource, err
	}

	_, _, err = Codecs.UniversalDecoder(s.GroupVersion).Decode(bytes, nil, resource)
	return resource, err
}
