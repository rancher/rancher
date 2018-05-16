package app

import (
	"net/http"

	"fmt"

	"reflect"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/controllers/management/compose/common"
	hcommon "github.com/rancher/rancher/pkg/controllers/user/helm/common"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	pv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	projectschema "github.com/rancher/types/apis/project.cattle.io/v3/schema"
	projectv3 "github.com/rancher/types/client/project/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Wrapper struct {
	Clusters              v3.ClusterInterface
	TemplateVersionClient v3.TemplateVersionInterface
	KubeConfigGetter      common.KubeConfigGetter
	TemplateContentClient v3.TemplateContentInterface
	AppGetter             pv3.AppsGetter
}

const (
	appLabel       = "io.cattle.field/appId"
	activeState    = "active"
	deployingState = "deploying"
)

func Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "upgrade")
	resource.AddAction(apiContext, "rollback")
	resource.Links["revision"] = apiContext.URLBuilder.Link("revision", resource)
	if _, ok := resource.Values["status"]; ok {
		if status, ok := resource.Values["status"].(map[string]interface{}); ok {
			delete(status, "lastAppliedTemplate")
		}
	}
	var workloads []projectv3.Workload
	if err := access.List(apiContext, &projectschema.Version, projectv3.WorkloadType, &types.QueryOptions{}, &workloads); err == nil {
		for _, w := range workloads {
			_, appID := ref.Parse(resource.ID)
			if w.WorkloadLabels[appLabel] == appID && w.State != activeState {
				resource.Values["state"] = deployingState
				resource.Values["transitioning"] = "yes"
				transitionMsg := convert.ToString(resource.Values["transitioningMessage"])
				if transitionMsg != "" {
					transitionMsg += "; "
				}
				resource.Values["transitioningMessage"] = transitionMsg + fmt.Sprintf("Workload %s: %s", w.Name, w.TransitioningMessage)
			}
		}
	}
	delete(resource.Values, "appliedFiles")
	delete(resource.Values, "files")
}

func (w Wrapper) Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	externalID := convert.ToString(data["externalId"])
	templateVersionID, err := hcommon.ParseExternalID(externalID)
	if err != nil {
		return err
	}
	templateVersion, err := w.TemplateVersionClient.Get(templateVersionID, metav1.GetOptions{})
	if err != nil {
		return err
	}
	targetNamespace := convert.ToString(data["targetNamespace"])
	if templateVersion.Spec.RequiredNamespace != "" && templateVersion.Spec.RequiredNamespace != targetNamespace {
		return httperror.NewAPIError(httperror.InvalidType, "template's requiredNamespace doesn't match catalog app's target namespace")
	}
	return nil
}

func (w Wrapper) ActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	var app projectv3.App
	if err := access.ByID(apiContext, &projectschema.Version, projectv3.AppType, apiContext.ID, &app); err != nil {
		return err
	}
	actionInput, err := parse.ReadBody(apiContext.Request)
	if err != nil {
		return err
	}
	switch actionName {
	case "upgrade":
		externalID := actionInput["externalId"]
		answers := actionInput["answers"]
		_, namespace := ref.Parse(app.ProjectId)
		obj, err := w.AppGetter.Apps(namespace).Get(app.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if answers != nil {
			m, ok := answers.(map[string]interface{})
			if ok {
				if obj.Spec.Answers == nil {
					obj.Spec.Answers = make(map[string]string)
				}
				for k, v := range m {
					obj.Spec.Answers[k] = convert.ToString(v)
				}
			}
		}
		obj.Spec.ExternalID = convert.ToString(externalID)
		if _, err := w.AppGetter.Apps(namespace).Update(obj); err != nil {
			return err
		}
		return nil
	case "rollback":
		revision := actionInput["revisionId"]
		if convert.ToString(revision) == "" {
			return fmt.Errorf("revision is empty")
		}
		var appRevision projectv3.AppRevision
		_, projectID := ref.Parse(app.ProjectId)
		revisionID := fmt.Sprintf("%s:%s", projectID, convert.ToString(revision))
		if err := access.ByID(apiContext, &projectschema.Version, projectv3.AppRevisionType, revisionID, &appRevision); err != nil {
			return err
		}
		_, namespace := ref.Parse(app.ProjectId)
		obj, err := w.AppGetter.Apps(namespace).Get(app.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		obj.Spec.Answers = appRevision.Status.Answers
		obj.Spec.ExternalID = appRevision.Status.ExternalID
		if _, err := w.AppGetter.Apps(namespace).Update(obj); err != nil {
			return err
		}
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
