package secret

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	v1 "github.com/rancher/rancher/pkg/types/apis/core/v1"
)

func Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	unauthedError := httperror.NewAPIError(httperror.PermissionDenied, "unauthorized")

	if request.Method == http.MethodPost {
		id := ""

		// extracting project name from data
		if projectData, ok := data["projectId"].(string); ok {
			if projectParts := strings.Split(projectData, ":"); len(projectParts) == 2 {
				id = fmt.Sprintf("%s:%s", projectParts[1], data["name"])
			}
		}

		if id == "" {
			return unauthedError
		}

		// minimum info needed to use CanDo
		secretState := map[string]interface{}{
			"name":        data["name"],
			"id":          id,
			"namespaceId": data["namespaceId"],
		}

		// update is used here to avoid the application of general user permissions for secrets
		if err := request.AccessControl.CanDo(v1.SecretGroupVersionKind.Group, v1.SecretResource.Name, "update", request, secretState, schema); err != nil {
			return unauthedError
		}
	} else if request.Method == http.MethodPut {
		var secretState map[string]interface{}

		if err := access.ByID(request, request.Version, request.Type, request.ID, &secretState); err != nil {
			if httperror.IsNotFound(err) || isUnauthorized(err) {
				return httperror.NewAPIError(httperror.NotFound, "not found")
			}
			return httperror.NewAPIError(httperror.ServerError, err.Error())
		}
		// this is unused but will be necessary if readonly users are ever given permission to view secrets
		if err := request.AccessControl.CanDo(v1.SecretGroupVersionKind.Group, v1.SecretResource.Name, "update", request, secretState, schema); err != nil {
			return unauthedError
		}
	}
	return nil
}

func isUnauthorized(err interface{}) bool {
	if err, ok := err.(*httperror.APIError); ok {
		return err.Code.Status == 403
	}
	return false
}
