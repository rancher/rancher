package globaldns

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	gaccess "github.com/rancher/rancher/pkg/api/customization/globalnamespaceaccess"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"

	"k8s.io/apimachinery/pkg/api/meta"
)

const (
	addProjectsAction    = "addProjects"
	removeProjectsAction = "removeProjects"
)

func (w *Wrapper) Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, addProjectsAction)
	resource.AddAction(apiContext, removeProjectsAction)
}

func (w *Wrapper) ActionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	if err := access.ByID(request, &managementschema.Version, client.GlobalDNSType, request.ID, &client.GlobalDNS{}); err != nil {
		return err
	}
	split := strings.SplitN(request.ID, ":", 2)
	if len(split) != 2 {
		return fmt.Errorf("incorrect global DNS ID")
	}
	existingProjects := make(map[string]bool)
	gDNS, err := w.GlobalDNSes.Controller().Lister().Get(split[0], split[1])
	if err != nil {
		return err
	}
	// ensure that caller is not a readonly member of globaldns, else abort
	callerID := request.Request.Header.Get(gaccess.ImpersonateUserHeader)
	metaAccessor, err := meta.Accessor(gDNS)
	if err != nil {
		return err
	}
	creatorID, ok := metaAccessor.GetAnnotations()[creatorIDAnn]
	if !ok {
		return fmt.Errorf("GlobalDNS %v has no creatorId annotation", metaAccessor.GetName())
	}
	ma := gaccess.MemberAccess{
		Users: w.Users,
	}
	accessType, err := ma.GetAccessTypeOfCaller(callerID, creatorID, gDNS.Name, gDNS.Spec.Members)
	if err != nil {
		return err
	}
	if accessType != gaccess.OwnerAccess {
		return fmt.Errorf("only owners can modify global DNS target projects")
	}

	actionInput, err := parse.ReadBody(request.Request)
	if err != nil {
		return err
	}
	inputProjects := convert.ToStringSlice(actionInput[client.UpdateGlobalDNSTargetsInputFieldProjectIDs])
	for _, p := range gDNS.Spec.ProjectNames {
		existingProjects[p] = true
	}

	switch actionName {
	case addProjectsAction:
		return w.addProjects(gDNS, request, inputProjects, existingProjects)
	case removeProjectsAction:
		return w.removeProjects(gDNS, request, existingProjects, inputProjects)
	default:
		return fmt.Errorf("bad action for global dns %v", actionName)
	}
}

func (w *Wrapper) addProjects(gDNS *v3.GlobalDNS, request *types.APIContext, inputProjects []string, existingProjects map[string]bool) error {
	if gDNS.Spec.MultiClusterAppName != "" {
		return fmt.Errorf("cannot add projects to globaldns as targets if multiclusterappId is set")
	}
	ma := gaccess.MemberAccess{
		Users: w.Users,
	}
	if err := ma.CheckCallerAccessToTargets(request, inputProjects, client.ProjectType, &client.Project{}); err != nil {
		return err
	}
	gDNSToUpdate := gDNS.DeepCopy()

	for _, p := range inputProjects {
		if !existingProjects[p] {
			gDNSToUpdate.Spec.ProjectNames = append(gDNSToUpdate.Spec.ProjectNames, p)
		}
	}
	return w.updateGDNS(gDNSToUpdate, request, "addedProjects")
}

func (w *Wrapper) removeProjects(gDNS *v3.GlobalDNS, request *types.APIContext, existingProjects map[string]bool, inputProjects []string) error {
	gDNSToUpdate := gDNS.DeepCopy()
	toRemoveProjects := make(map[string]bool)
	var finalProjects []string
	for _, p := range inputProjects {
		toRemoveProjects[p] = true
	}
	for _, p := range gDNS.Spec.ProjectNames {
		if !toRemoveProjects[p] {
			finalProjects = append(finalProjects, p)
		}
	}
	gDNSToUpdate.Spec.ProjectNames = finalProjects
	return w.updateGDNS(gDNSToUpdate, request, "removedProjects")
}

func (w Wrapper) updateGDNS(gDNSToUpdate *v3.GlobalDNS, request *types.APIContext, message string) error {
	if _, err := w.GlobalDNSes.Update(gDNSToUpdate); err != nil {
		return err
	}
	op := map[string]interface{}{
		"message": message,
	}
	request.WriteResponse(http.StatusOK, op)
	return nil
}
