package norman

import (
	"errors"
	"net/http"
	"strings"

	normanapi "github.com/rancher/norman/api"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/httperror/handler"
	"github.com/rancher/norman/types"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/settings"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewServer(schemas *types.Schemas) (*normanapi.Server, error) {
	server := normanapi.NewAPIServer()
	server.Defaults.ErrorHandler = writeAPIError
	if err := server.AddSchemas(schemas); err != nil {
		return nil, err
	}
	ConfigureAPIUI(server)
	return server, nil
}

func writeAPIError(request *types.APIContext, err error) {
	if apiErr, ok := errors.AsType[*httperror.APIError](err); ok &&
		apiErr.Code.Status == http.StatusUnprocessableEntity &&
		apiErr.Code.Code == string(metav1.StatusReasonInvalid) &&
		request.Schema != nil {
		switch request.Schema.ID {
		case client.ProjectRoleTemplateBindingType, client.ClusterRoleTemplateBindingType:
			// Legacy Rancher webhook responses encode RBAC escalation denials as Invalid.
			prefix := `admission webhook "rancher.cattle.io.` + strings.ToLower(request.Schema.ID) + `s.management.cattle.io" denied the request: user "`
			if strings.HasPrefix(apiErr.Message, prefix) &&
				strings.Contains(apiErr.Message, ") is attempting to grant RBAC permissions not currently held:\n") {
				forbidden := *apiErr
				forbidden.Code = httperror.ErrorCode{Status: http.StatusForbidden, Code: string(metav1.StatusReasonForbidden)}
				err = &forbidden
			}
		}
	}

	handler.ErrorHandler(request, err)
}

func ConfigureAPIUI(server *normanapi.Server) {
	server.CustomAPIUIResponseWriter(nil, nil, settings.APIUIVersion.Get)
}
