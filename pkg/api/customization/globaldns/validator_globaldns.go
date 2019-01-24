package globaldns

import (
	"net/http"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	access "github.com/rancher/rancher/pkg/api/customization/globalnamespaceaccess"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/client/management/v3"
)

type Wrapper struct {
	GlobalDNSLister v3.GlobalDNSLister
	Users           v3.UserInterface
	PrtbLister      v3.ProjectRoleTemplateBindingLister
}

func (w Wrapper) Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	if request.Method != http.MethodPut && request.Method != http.MethodPost {
		return nil
	}

	ma := access.MemberAccess{
		Users:      w.Users,
		PrtbLister: w.PrtbLister,
	}
	// check access to target projects if given
	targetProjects := convert.ToStringSlice(data[client.GlobalDNSFieldProjectIDs])
	if len(targetProjects) > 0 {
		if err := ma.CheckCreatorAndMembersAccessToTargets(request, targetProjects, data, client.GlobalDNSType, v3.ProjectGroupVersionKind.Group, v3.ProjectResource.Name); err != nil {
			return err
		}
	}

	// check access to multiclusterapp if given
	mcappID := convert.ToString(data[client.GlobalDNSFieldMultiClusterAppID])
	if mcappID != "" {
		return ma.CheckCreatorAndMembersAccessToTargets(request, []string{mcappID}, data, client.GlobalDNSType, v3.MultiClusterAppGroupVersionKind.Group, v3.MultiClusterAppResource.Name)
	}
	return nil
}
