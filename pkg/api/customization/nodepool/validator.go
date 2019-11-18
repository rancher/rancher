package nodepool

import (
	"fmt"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	mgmtSchema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	mgmtclient "github.com/rancher/types/client/management/v3"
)

func Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	nodetemplateID, _ := data["nodeTemplateId"].(string)
	if err := access.ByID(request, &mgmtSchema.Version, mgmtclient.NodeTemplateType, nodetemplateID, nil); err != nil {
		if httperror.IsNotFound(err) || httperror.IsForbidden(err) {
			return httperror.NewAPIError(httperror.NotFound, fmt.Sprintf("unable to find node template [%s]", nodetemplateID))
		}
		return httperror.NewAPIError(httperror.ServerError, err.Error())

	}
	return nil
}
