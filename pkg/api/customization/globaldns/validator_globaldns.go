package globaldns

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	gaccess "github.com/rancher/rancher/pkg/api/customization/globalnamespaceaccess"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	client "github.com/rancher/types/client/management/v3"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Wrapper struct {
	GlobalDNSLister       v3.GlobalDnsLister
	GlobalDNSes           v3.GlobalDnsInterface
	PrtbLister            v3.ProjectRoleTemplateBindingLister
	MultiClusterAppLister v3.MultiClusterAppLister
	Users                 v3.UserInterface
	GrbLister             v3.GlobalRoleBindingLister
	GrLister              v3.GlobalRoleLister
}

const (
	creatorIDAnn = "field.cattle.io/creatorId"
)

func (w Wrapper) Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	if request.Method != http.MethodPut && request.Method != http.MethodPost {
		return nil
	}

	var targetProjects []string
	ma := gaccess.MemberAccess{
		Users:     w.Users,
		GrLister:  w.GrLister,
		GrbLister: w.GrbLister,
	}

	callerID := request.Request.Header.Get(gaccess.ImpersonateUserHeader)
	if request.Method == http.MethodPost {
		// create request, caller is owner/creator
		// Request is POST, hence global DNS is being created.
		// if multiclusterapp ID is provided check access to its projects
		mcappID := convert.ToString(data[client.GlobalDnsFieldMultiClusterAppID])
		if mcappID != "" {
			split := strings.SplitN(mcappID, ":", 2)
			if len(split) != 2 {
				return fmt.Errorf("incorrect multiclusterapp id %v provided for global dns %v", mcappID, request.ID)
			}
			mcapp, err := w.MultiClusterAppLister.Get(split[0], split[1])
			if err != nil {
				return err
			}
			for _, t := range mcapp.Spec.Targets {
				targetProjects = append(targetProjects, t.ProjectName)
			}
		} else {
			// if not, check access to all projects provided in the projects list
			targetProjects = convert.ToStringSlice(data[client.GlobalDnsFieldProjectIDs])
		}
		return ma.CheckCallerAccessToTargets(request, targetProjects, client.ProjectType, &client.Project{})
	}
	// edit request, check access type caller has
	split := strings.SplitN(request.ID, ":", 2)
	if len(split) != 2 {
		return fmt.Errorf("incorrect global DNS ID %v", request.ID)
	}

	gDNS, err := w.GlobalDNSes.GetNamespaced(split[0], split[1], v1.GetOptions{})
	if err != nil {
		return err
	}
	metaAccessor, err := meta.Accessor(gDNS)
	if err != nil {
		return err
	}
	creatorID, ok := metaAccessor.GetAnnotations()[creatorIDAnn]
	if !ok {
		return fmt.Errorf("GlobalDNS %v has no creatorId annotation", metaAccessor.GetName())
	}
	accessType, err := ma.GetAccessTypeOfCaller(callerID, creatorID, gDNS.Name, gDNS.Spec.Members)
	if err != nil {
		return err
	}
	if accessType != gaccess.OwnerAccess {
		return fmt.Errorf("invalid access type %v for globaldns member", accessType)
	}
	// only members list, FQDN and multiclusterappID can be edited through PUT, for updating projects, we need to use actions only
	// that's why projects and multiclusterappID field have been made non updatable in rancher/types
	if err := gaccess.CheckAccessToUpdateMembers(gDNS.Spec.Members, data, accessType == gaccess.OwnerAccess); err != nil {
		return err
	}

	originalMultiClusterApp := gDNS.Spec.MultiClusterAppName
	newMultiClusterApp := convert.ToString(data[client.GlobalDnsFieldMultiClusterAppID])
	if newMultiClusterApp != "" && originalMultiClusterApp != newMultiClusterApp {
		// check access to new multiclusterapp
		return ma.CheckCallerAccessToTargets(request, []string{newMultiClusterApp}, client.MultiClusterAppType, &client.MultiClusterApp{})
	}
	return nil
}
