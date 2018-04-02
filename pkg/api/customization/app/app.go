package app

import (
	"net/http"
	"time"

	"strings"

	"fmt"

	"reflect"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/controllers/management/compose/common"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	projectschema "github.com/rancher/types/apis/project.cattle.io/v3/schema"
	projectv3 "github.com/rancher/types/client/project/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Wrapper struct {
	Clusters              v3.ClusterInterface
	KubeConfigGetter      common.KubeConfigGetter
	TemplateContentClient v3.TemplateContentInterface
}

const appLabel = "io.cattle.field/appId"

func Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "upgrade")
	resource.AddAction(apiContext, "rollback")
	resource.Links["export"] = apiContext.URLBuilder.Link("export", resource)
	resource.Links["revision"] = apiContext.URLBuilder.Link("revision", resource)
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
	store := apiContext.Schema.Store
	switch actionName {
	case "upgrade":
		externalID := actionInput["externalId"]
		answers := actionInput["answers"]
		if answers != nil {
			m, ok := answers.(map[string]interface{})
			if ok {
				if app.Answers == nil {
					app.Answers = make(map[string]string)
				}
				for k, v := range m {
					app.Answers[k] = convert.ToString(v)
				}
			}
		}
		app.ExternalID = convert.ToString(externalID)
		data, err := convert.EncodeToMap(app)
		if err != nil {
			return err
		}
		_, err = store.Update(apiContext, apiContext.Schema, data, apiContext.ID)
		if err != nil {
			return err
		}
		return nil
	case "rollback":
		revision := actionInput["revision"]
		if convert.ToString(revision) == "" {
			return fmt.Errorf("revision is empty")
		}
		var appRevision projectv3.AppRevision
		_, projectID := ref.Parse(app.ProjectId)
		revisionID := fmt.Sprintf("%s:%s", projectID, convert.ToString(revision))
		if err := access.ByID(apiContext, &projectschema.Version, projectv3.AppRevisionType, revisionID, &appRevision); err != nil {
			return err
		}
		app.Answers = appRevision.Status.Answers
		app.ExternalID = appRevision.Status.ExternalID
		data, err := convert.EncodeToMap(app)
		if err != nil {
			return err
		}
		_, err = store.Update(apiContext, apiContext.Schema, data, apiContext.ID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (w Wrapper) LinkHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	switch apiContext.Link {
	case "export":
		var app projectv3.App
		if err := access.ByID(apiContext, &projectschema.Version, projectv3.AppType, apiContext.ID, &app); err != nil {
			return err
		}
		var appRevision projectv3.AppRevision
		_, projectID := ref.Parse(app.ProjectId)
		revisionID := fmt.Sprintf("%s:%s", projectID, app.AppRevisionId)
		if err := access.ByID(apiContext, &projectschema.Version, projectv3.AppRevisionType, revisionID, &appRevision); err != nil {
			return err
		}
		tc, err := w.TemplateContentClient.Get(appRevision.Status.Digest, metav1.GetOptions{})
		if err != nil {
			return err
		}
		reader := strings.NewReader(tc.Data)
		apiContext.Response.Header().Set("Content-Type", "text/plain")
		http.ServeContent(apiContext.Response, apiContext.Request, "readme", time.Now(), reader)
		return nil
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
