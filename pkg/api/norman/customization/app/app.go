package app

import (
	"fmt"
	"net/http"
	"reflect"

	v32 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	catUtil "github.com/rancher/rancher/pkg/catalog/utils"
	clusterv3 "github.com/rancher/rancher/pkg/client/generated/cluster/v3"
	projectv3 "github.com/rancher/rancher/pkg/client/generated/project/v3"
	"github.com/rancher/rancher/pkg/controllers/management/compose/common"
	hcommon "github.com/rancher/rancher/pkg/controllers/managementuser/helm/common"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	pv3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/ref"
	clusterschema "github.com/rancher/rancher/pkg/schemas/cluster.cattle.io/v3"
	projectschema "github.com/rancher/rancher/pkg/schemas/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/user"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Wrapper struct {
	Clusters              v3.ClusterInterface
	TemplateVersionClient v3.CatalogTemplateVersionInterface
	TemplateVersionLister v3.CatalogTemplateVersionLister
	KubeConfigGetter      common.KubeConfigGetter
	AppGetter             pv3.AppsGetter
	UserLister            v3.UserLister
	UserManager           user.Manager
}

const (
	appLabel      = "io.cattle.field/appId"
	MCappLabel    = "mcapp"
	creatorIDAnno = "field.cattle.io/creatorId"
)

func Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	mcappLabel := convert.ToString(values.GetValueN(resource.Values, "labels", MCappLabel))
	if mcappLabel == "" {
		resource.AddAction(apiContext, "upgrade")
		resource.AddAction(apiContext, "rollback")
	} else {
		delete(resource.Links, "remove")
	}
	resource.Links["revision"] = apiContext.URLBuilder.Link("revision", resource)
	if _, ok := resource.Values["status"]; ok {
		if status, ok := resource.Values["status"].(map[string]interface{}); ok {
			delete(status, "lastAppliedTemplate")
		}
	}
	delete(resource.Values, "appliedFiles")
	delete(resource.Values, "files")
}

func (w Wrapper) Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	externalID := convert.ToString(data["externalId"])
	if externalID == "" {
		return nil
	}
	templateVersionID, templateVersionNamespace, err := hcommon.ParseExternalID(externalID)
	if err != nil {
		return err
	}
	templateVersion, err := w.TemplateVersionClient.GetNamespaced(templateVersionNamespace, templateVersionID, metav1.GetOptions{})
	if err != nil {
		return err
	}
	targetNamespace := convert.ToString(data["targetNamespace"])
	if templateVersion.Spec.RequiredNamespace != "" && templateVersion.Spec.RequiredNamespace != targetNamespace {
		return httperror.NewAPIError(httperror.InvalidType, "template's requiredNamespace doesn't match catalog app's target namespace")
	}

	// in here access.ByID will only find namespace that is assigned to the current project
	var ns clusterv3.Namespace
	if err := access.ByID(request, &clusterschema.Version, clusterv3.NamespaceType, targetNamespace, &ns); err != nil {
		return err
	}
	if ns.Name == "" {
		return httperror.NewAPIError(httperror.InvalidReference, fmt.Sprintf("target namespace %v is not assigned to the current project %v", targetNamespace, data["projectId"]))
	}

	return nil
}

func (w Wrapper) ActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	var app projectv3.App
	if err := access.ByID(apiContext, &projectschema.Version, projectv3.AppType, apiContext.ID, &app); err != nil {
		return err
	}

	var appMap map[string]interface{}
	if err := access.ByID(apiContext, &projectschema.Version, projectv3.AppType, apiContext.ID, &appMap); err != nil {
		return httperror.NewAPIError(httperror.NotFound, fmt.Sprintf("unable to access app by id: %v", err))
	}

	if err := apiContext.AccessControl.CanDo(pv3.AppGroupVersionKind.Group, pv3.AppResource.Name, "update", apiContext, appMap, apiContext.Schema); err != nil {
		return httperror.NewAPIError(httperror.PermissionDenied, fmt.Sprintf("user does not have permission to update for action %s", actionName))
	}

	creatorNotFound := false
	if _, err := w.UserLister.Get("", app.CreatorID); err != nil && apierrors.IsNotFound(err) {
		creatorNotFound = true
	}

	actionInput, err := parse.ReadBody(apiContext.Request)
	if err != nil {
		return err
	}
	switch actionName {
	case "upgrade":
		externalID := convert.ToString(actionInput["externalId"])

		err := w.validateRancherVersion(externalID)
		if err != nil {
			return err
		}

		answers := actionInput["answers"]
		forceUpgrade := actionInput["forceUpgrade"]
		files := actionInput["files"]
		valuesYaml := actionInput["valuesYaml"]
		_, namespace := ref.Parse(app.ProjectID)
		obj, err := w.AppGetter.Apps(namespace).Get(app.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if answers != nil {
			m, ok := answers.(map[string]interface{})
			if ok {
				obj.Spec.Answers = make(map[string]string)
				for k, v := range m {
					obj.Spec.Answers[k] = convert.ToString(v)
				}
			}
		} else {
			obj.Spec.Answers = make(map[string]string)
		}
		obj.Spec.ExternalID = externalID
		if convert.ToBool(forceUpgrade) {
			v32.AppConditionForceUpgrade.Unknown(obj)
		}
		if creatorNotFound {
			obj.Annotations[creatorIDAnno] = w.UserManager.GetUser(apiContext)
		}
		if files != nil {
			inputFiles := convert.ToMapInterface(files)
			if len(inputFiles) != 0 {
				targetFiles := make(map[string]string)
				for k, v := range inputFiles {
					targetFiles[k] = convert.ToString(v)
				}

				obj.Spec.Files = targetFiles
				obj.Spec.ExternalID = "" // ignore externalID
			}
		}
		if valuesYaml != nil {
			obj.Spec.ValuesYaml = convert.ToString(valuesYaml)
		} else {
			obj.Spec.ValuesYaml = ""
		}
		// indicate this a user driven action
		v32.AppConditionUserTriggeredAction.True(obj)
		if _, err := w.AppGetter.Apps(namespace).Update(obj); err != nil {
			return err
		}
		apiContext.WriteResponse(http.StatusNoContent, map[string]interface{}{})
		return nil
	case "rollback":
		forceUpgrade := actionInput["forceUpgrade"]
		revisionName := convert.ToString(actionInput["revisionId"])
		if revisionName == "" {
			return fmt.Errorf("revision is empty")
		}
		_, projectID := ref.Parse(app.ProjectID)
		revisionID := fmt.Sprintf("%s:%s", projectID, revisionName)
		var appRevision projectv3.AppRevision
		if err := access.ByID(apiContext, &projectschema.Version, projectv3.AppRevisionType, revisionID, &appRevision); err != nil {
			return err
		}

		err := w.validateRancherVersion(appRevision.Status.ExternalID)
		if err != nil {
			return err
		}

		_, namespace := ref.Parse(app.ProjectID)
		obj, err := w.AppGetter.Apps(namespace).Get(app.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		obj.Spec.Answers = appRevision.Status.Answers
		obj.Spec.ExternalID = appRevision.Status.ExternalID
		obj.Spec.ValuesYaml = appRevision.Status.ValuesYaml
		if convert.ToBool(forceUpgrade) {
			v32.AppConditionForceUpgrade.Unknown(obj)
		}
		if creatorNotFound {
			obj.Annotations[creatorIDAnno] = w.UserManager.GetUser(apiContext)
		}
		obj.Spec.Files = appRevision.Status.Files
		// indicate this a user driven action
		v32.AppConditionUserTriggeredAction.True(obj)
		if _, err := w.AppGetter.Apps(namespace).Update(obj); err != nil {
			return err
		}
		apiContext.WriteResponse(http.StatusNoContent, map[string]interface{}{})
		return nil
	}
	return nil
}

func (w Wrapper) LinkHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	switch apiContext.Link {
	case "revision":
		var app projectv3.App
		if err := access.ByID(apiContext, &projectschema.Version, projectv3.AppType, apiContext.ID, &app); err != nil {
			return err
		}
		var appRevisions, filtered []map[string]interface{}
		if err := access.List(apiContext, &projectschema.Version, projectv3.AppRevisionType, &types.QueryOptions{}, &appRevisions); err != nil {
			return err
		}
		for _, re := range appRevisions {
			labels := convert.ToMapInterface(re["labels"])
			if reflect.DeepEqual(labels[appLabel], app.Name) {
				filtered = append(filtered, re)
			}
		}
		apiContext.Type = projectv3.AppRevisionType
		apiContext.WriteResponse(http.StatusOK, filtered)
		return nil
	}
	return nil
}

func (w Wrapper) validateRancherVersion(externalID string) error {
	if externalID == "" {
		return nil
	}
	templateVersionID, namespace, err := hcommon.ParseExternalID(externalID)
	if err != nil {
		return err
	}
	template, err := w.TemplateVersionLister.Get(namespace, templateVersionID)
	if err != nil {
		return err
	}
	return catUtil.ValidateRancherVersion(template)
}
