package navlinks

import (
	"fmt"

	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/v2/pkg/schemas/validation"
)

type store struct {
	types.Store
}

func (e *store) ByID(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	result, err := e.Store.ByID(apiOp, schema, id)
	if err != nil {
		return result, err
	}
	if !hasAccess(apiOp, result) {
		return types.APIObject{}, validation.NotFound
	}
	return result, err
}

func hasAccess(apiOp *types.APIRequest, result types.APIObject) bool {
	data := result.Data().Map("spec", "toService")
	if len(data) == 0 {
		return true
	}

	scheme := data.String("scheme")
	if scheme == "" {
		scheme = "http"
	}
	port := data.String("port")
	if port == "" {
		port = "80"
	}

	serviceNamespace := data.String("namespace")
	serviceName := fmt.Sprintf("%s:%s:%s", scheme, data.String("name"), port)
	return apiOp.AccessControl.CanDo(apiOp, "/services/proxy", "get", serviceNamespace, serviceName) == nil
}

func (e *store) List(apiOp *types.APIRequest, schema *types.APISchema) (types.APIObjectList, error) {
	result, err := e.Store.List(apiOp, schema)
	if err != nil {
		return result, err
	}
	filtered := result
	filtered.Objects = make([]types.APIObject, 0, len(filtered.Objects))
	for _, obj := range result.Objects {
		if hasAccess(apiOp, obj) {
			filtered.Objects = append(filtered.Objects, obj)
		}
	}
	return filtered, nil
}

func (e *store) Watch(apiOp *types.APIRequest, schema *types.APISchema, wr types.WatchRequest) (chan types.APIEvent, error) {
	result, err := e.Store.Watch(apiOp, schema, wr)
	if err != nil {
		return result, err
	}

	newResult := make(chan types.APIEvent, 1)
	go func() {
		defer close(newResult)
		for event := range result {
			if hasAccess(apiOp, event.Object) {
				newResult <- event
			}
		}
	}()

	return newResult, nil
}
