package multiclusterapp

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"github.com/pkg/errors"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	gaccess "github.com/rancher/rancher/pkg/api/customization/globalnamespaceaccess"
	catUtil "github.com/rancher/rancher/pkg/catalog/utils"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	managementschema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/namespace"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	addProjectsAction    = "addProjects"
	removeProjectsAction = "removeProjects"
)

var backoff = wait.Backoff{
	Duration: 100 * time.Millisecond,
	Factor:   2,
	Jitter:   0,
	Steps:    6,
}

func (w Wrapper) Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "rollback")
	resource.AddAction(apiContext, addProjectsAction)
	resource.AddAction(apiContext, removeProjectsAction)
	resource.Links["revisions"] = apiContext.URLBuilder.Link("revisions", resource)
}

func (w Wrapper) ActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	var mcApp client.MultiClusterApp
	if err := access.ByID(apiContext, &managementschema.Version, client.MultiClusterAppType, apiContext.ID, &mcApp); err != nil {
		return err
	}
	switch actionName {
	case "rollback":
		data, err := ioutil.ReadAll(apiContext.Request.Body)
		if err != nil {
			return errors.Wrap(err, "reading request body error")
		}
		input := client.MultiClusterAppRollbackInput{}
		if err = json.Unmarshal(data, &input); err != nil {
			return errors.Wrap(err, "unmarshal input error")
		}
		id := input.RevisionID
		splitID := strings.Split(input.RevisionID, ":")
		if len(splitID) == 2 {
			id = splitID[1]
		}
		revision, err := w.MultiClusterAppRevisionLister.Get(namespace.GlobalNamespace, id)
		if err != nil {
			return err
		}
		obj, err := w.MultiClusterApps.GetNamespaced(namespace.GlobalNamespace, mcApp.Name, v1.GetOptions{})
		if err != nil {
			return err
		}
		if obj.Status.RevisionName == revision.Name {
			return nil
		}
		err = w.validateRancherVersion(revision.TemplateVersionName)
		if err != nil {
			return err
		}
		toUpdate := obj.DeepCopy()
		toUpdate.Spec.TemplateVersionName = revision.TemplateVersionName
		toUpdate.Spec.Answers = revision.Answers
		_, err = w.MultiClusterApps.Update(toUpdate)
		return err
	case addProjectsAction:
		return w.addProjects(apiContext)
	case removeProjectsAction:
		return w.removeProjects(apiContext)
	default:
		return fmt.Errorf("bad action for multiclusterapp %v", actionName)
	}
}

func (w Wrapper) addProjects(request *types.APIContext) error {
	split := strings.SplitN(request.ID, ":", 2)
	if len(split) != 2 {
		return fmt.Errorf("incorrect multi cluster app ID %v", request.ID)
	}
	inputProjects, inputAnswers, err := w.modifyProjects(request, addProjectsAction)
	if err != nil {
		return err
	}

	err = wait.ExponentialBackoff(backoff, func() (bool, error) {
		existingProjects := make(map[string]bool)
		mcapp, err := w.MultiClusterApps.GetNamespaced(split[0], split[1], v1.GetOptions{})
		if err != nil {
			return false, err
		}
		for _, p := range mcapp.Spec.Targets {
			existingProjects[p.ProjectName] = true
		}
		for _, p := range inputProjects {
			if existingProjects[p] {
				return false, httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("duplicate projects in targets %s", p))
			}
			existingProjects[p] = true
		}
		for _, name := range inputProjects {
			mcapp.Spec.Targets = append(mcapp.Spec.Targets, v32.Target{ProjectName: name})
		}
		if len(inputAnswers) > 0 {
			mcapp.Spec.Answers = append(mcapp.Spec.Answers, inputAnswers...)
		}
		_, err = w.MultiClusterApps.Update(mcapp)
		if err != nil {
			if apierrors.IsConflict(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return err
	}
	op := map[string]interface{}{
		"message": "addedProjects",
	}
	request.WriteResponse(http.StatusOK, op)
	return nil
}

func (w Wrapper) removeProjects(request *types.APIContext) error {
	inputProjects, _, err := w.modifyProjects(request, removeProjectsAction)
	if err != nil {
		return err
	}
	split := strings.SplitN(request.ID, ":", 2)
	if len(split) != 2 {
		return fmt.Errorf("incorrect multi cluster app ID %v", request.ID)
	}
	err = wait.ExponentialBackoff(backoff, func() (bool, error) {
		mcapp, err := w.MultiClusterApps.GetNamespaced(split[0], split[1], v1.GetOptions{})
		if err != nil {
			return false, err
		}
		toRemoveProjects := make(map[string]bool)
		var finalTargets []v32.Target
		for _, p := range inputProjects {
			toRemoveProjects[p] = true
		}
		for _, t := range mcapp.Spec.Targets {
			if !toRemoveProjects[t.ProjectName] {
				finalTargets = append(finalTargets, t)
			}
		}
		// after this finalTargets will contain all mcapp targets, that aren't in inputProjects
		mcapp.Spec.Targets = finalTargets
		_, err = w.MultiClusterApps.Update(mcapp)
		if err != nil {
			if apierrors.IsConflict(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return err
	}
	op := map[string]interface{}{
		"message": "removedProjects",
	}
	request.WriteResponse(http.StatusOK, op)
	return nil
}

func (w Wrapper) modifyProjects(request *types.APIContext, actionName string) ([]string, []v32.Answer, error) {
	split := strings.SplitN(request.ID, ":", 2)
	if len(split) != 2 {
		return []string{}, []v32.Answer{}, fmt.Errorf("incorrect multi cluster app ID %v", request.ID)
	}
	var inputProjects []string
	var inputAnswers []v32.Answer
	mcapp, err := w.MultiClusterApps.GetNamespaced(split[0], split[1], v1.GetOptions{})
	if err != nil {
		return inputProjects, inputAnswers, err
	}
	// ensure that caller is not a readonly member of multiclusterapp, else abort
	callerID := request.Request.Header.Get(gaccess.ImpersonateUserHeader)
	metaAccessor, err := meta.Accessor(mcapp)
	if err != nil {
		return inputProjects, inputAnswers, err
	}
	creatorID, ok := metaAccessor.GetAnnotations()[creatorIDAnn]
	if !ok {
		return inputProjects, inputAnswers, fmt.Errorf("multiclusterapp %v has no creatorId annotation", metaAccessor.GetName())
	}
	ma := gaccess.MemberAccess{
		Users:              w.Users,
		PrtbLister:         w.PrtbLister,
		CrtbLister:         w.CrtbLister,
		RoleTemplateLister: w.RoleTemplateLister,
		GrbLister:          w.GrbLister,
		GrLister:           w.GrLister,
		Prtbs:              w.Prtbs,
		Crtbs:              w.Crtbs,
		ProjectLister:      w.ProjectLister,
		ClusterLister:      w.ClusterLister,
	}
	accessType, err := ma.GetAccessTypeOfCaller(callerID, creatorID, mcapp.Name, mcapp.Spec.Members)
	if err != nil {
		return inputProjects, inputAnswers, err
	}
	if accessType != gaccess.OwnerAccess {
		return inputProjects, inputAnswers, fmt.Errorf("only owners can modify projects of multiclusterapp")
	}
	var updateMultiClusterAppTargetsInput client.UpdateMultiClusterAppTargetsInput
	actionInput, err := parse.ReadBody(request.Request)
	if err != nil {
		return inputProjects, inputAnswers, err
	}
	if err = convert.ToObj(actionInput, &updateMultiClusterAppTargetsInput); err != nil {
		return inputProjects, inputAnswers, err
	}
	inputProjects = updateMultiClusterAppTargetsInput.Projects
	if actionName == addProjectsAction {
		if err = ma.EnsureRoleInTargets(inputProjects, mcapp.Spec.Roles, callerID); err != nil {
			return inputProjects, inputAnswers, err
		}
	} else if actionName == removeProjectsAction {
		// we want to remove all roles that the mcapp's sys acc has in these projects being removed
		if err = ma.RemoveRolesFromTargets(inputProjects, []string{}, mcapp.Name, true); err != nil {
			return inputProjects, inputAnswers, err
		}
	}
	for _, a := range updateMultiClusterAppTargetsInput.Answers {
		inputAnswers = append(inputAnswers, v32.Answer{
			ProjectName: a.ProjectID,
			ClusterName: a.ClusterID,
			Values:      a.Values,
		})
	}
	// check if the input includes answers, and if they are only for the input projects
	if len(inputAnswers) > 0 {
		inputProjectsMap := make(map[string]bool)
		for _, p := range inputProjects {
			if !inputProjectsMap[p] {
				inputProjectsMap[p] = true
			}
		}
		for _, a := range inputAnswers {
			if a.ProjectName == "" {
				return inputProjects, inputAnswers, fmt.Errorf("can only provide project-scoped answers for new projects through add/remove projects action")
			}
			if !inputProjectsMap[a.ProjectName] {
				return inputProjects, inputAnswers, fmt.Errorf("the project %v is not among the ones provided in input", a.ProjectName)
			}
		}
	}
	return inputProjects, inputAnswers, nil
}

func (w Wrapper) validateRancherVersion(tempVersion string) error {
	parts := strings.Split(tempVersion, ":")
	if len(parts) != 2 {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "invalid templateVersionId")
	}

	template, err := w.TemplateVersionLister.Get(namespace.GlobalNamespace, parts[1])
	if err != nil {
		return err
	}

	return catUtil.ValidateRancherVersion(template)
}
