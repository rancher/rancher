package feature

import (
	"net/http"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	v3client "github.com/rancher/types/client/management/v3"
)

func Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	if request.Method == http.MethodPost {
		return httperror.NewAPIError(httperror.MethodNotAllowed, "cannot create new features")
	}

	id := request.ID
	var feature v3client.Feature

	if err := access.ByID(request, request.Version, v3client.FeatureType, id, &feature); err != nil {
		if !httperror.IsNotFound(err) {
			return err
		}
	}

	newValue, ok := data["value"]
	if !ok {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "must contain value")
	}

	_, ok = newValue.(bool)
	if !ok {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "feature value must be a bool")
	}

	return nil
}

func Formatter(request *types.APIContext, resource *types.RawResource) {
	if request.Method == http.MethodGet {
		if resource.Values["value"] == nil {
			// if value is nil, then this ensure default value will be used
			resource.Values["value"] = resource.Values["default"]
		}
	}
}
