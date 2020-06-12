package parse

import (
	"fmt"
	"net/http"

	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/pkg/schemas/validation"
)

var (
	supportedMethods = map[string]bool{
		http.MethodPost:   true,
		http.MethodGet:    true,
		http.MethodPut:    true,
		http.MethodPatch:  true,
		http.MethodDelete: true,
	}
)

func ValidateMethod(request *types.APIRequest) error {
	if request.Action != "" && request.Method == http.MethodPost {
		return nil
	}

	if !supportedMethods[request.Method] {
		return apierror.NewAPIError(validation.MethodNotAllowed, fmt.Sprintf("Invalid method %s not supported", request.Method))
	}

	if request.Type == "" || request.Schema == nil || request.Link != "" {
		return nil
	}

	allowed := request.Schema.ResourceMethods
	if request.Name == "" {
		allowed = request.Schema.CollectionMethods
	}

	for _, method := range allowed {
		if method == request.Method {
			return nil
		}
	}

	return apierror.NewAPIError(validation.MethodNotAllowed, fmt.Sprintf("Method %s not supported", request.Method))
}
