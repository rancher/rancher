package nodepool

import (
	"fmt"
	"strings"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	mgmtclient "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	mgmtSchema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
)

type Validator struct {
	NodePoolLister v3.NodePoolLister
}

func (v *Validator) Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	// validate access to nodetemplate
	nodetemplateID, ok := data["nodeTemplateId"].(string)
	if !ok {
		// nodetemplate not passed, nothing to check
		return nil
	}
	if request.ID == "" {
		// creating new pool, confirm access to template
		return checkNodetemplateAccess(request, nodetemplateID)
	}

	// validate request ID is in the right format
	split := strings.SplitN(request.ID, ":", 2)
	if len(split) != 2 {
		return httperror.NewAPIError(httperror.NotFound, fmt.Sprintf("unable to find nodepool [%s]", request.ID))
	}
	cluster, nodepool := split[0], split[1]
	np, err := v.NodePoolLister.Get(cluster, nodepool)
	if err != nil {
		return httperror.NewAPIError(httperror.NotFound, fmt.Sprintf("unable to find nodepool [%s]", request.ID))
	}

	if np.Spec.NodeTemplateName != nodetemplateID {
		// pulling from lister failed, or update attempt to the nodetemplate
		return checkNodetemplateAccess(request, nodetemplateID)
	}

	return nil
}

func checkNodetemplateAccess(request *types.APIContext, nodetemplateID string) error {
	if err := access.ByID(request, &mgmtSchema.Version, mgmtclient.NodeTemplateType, nodetemplateID, nil); err != nil {
		if httperror.IsNotFound(err) || httperror.IsForbidden(err) {
			return httperror.NewAPIError(httperror.NotFound, fmt.Sprintf("unable to find node template [%s]", nodetemplateID))
		}
		return httperror.NewAPIError(httperror.ServerError, err.Error())
	}

	return nil
}
