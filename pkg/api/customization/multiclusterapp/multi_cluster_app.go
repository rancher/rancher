package multiclusterapp

import (
	"encoding/json"
	"github.com/pkg/errors"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/namespace"
	"io/ioutil"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"
	"net/http"
	"reflect"
)

type Wrapper struct {
	MultiClusterApps              v3.MultiClusterAppInterface
	MultiClusterAppLister         v3.MultiClusterAppLister
	MultiClusterAppRevisionLister v3.MultiClusterAppRevisionLister
}

const (
	mcAppLabel = "io.cattle.field/multiClusterAppId"
)

func (w Wrapper) LinkHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	switch apiContext.Link {
	case "revisions":
		var app client.MultiClusterApp
		if err := access.ByID(apiContext, &managementschema.Version, client.MultiClusterAppType, apiContext.ID, &app); err != nil {
			return err
		}
		var appRevisions, filtered []map[string]interface{}
		if err := access.List(apiContext, &managementschema.Version, client.MultiClusterAppRevisionType, &types.QueryOptions{}, &appRevisions); err != nil {
			return err
		}
		for _, revision := range appRevisions {
			labels := convert.ToMapInterface(revision["labels"])
			if reflect.DeepEqual(labels[mcAppLabel], app.Name) {
				filtered = append(filtered, revision)
			}
		}
		apiContext.Type = client.MultiClusterAppRevisionType
		apiContext.WriteResponse(http.StatusOK, filtered)
		return nil
	}
	return nil
}

func (w Wrapper) Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "rollback")
	resource.Links["revisions"] = apiContext.URLBuilder.Link("revisions", resource)
}

func (w Wrapper) ActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	var mcApp client.MultiClusterApp
	if err := access.ByID(apiContext, &managementschema.Version, client.MultiClusterAppType, apiContext.ID, &mcApp); err != nil {
		return err
	}
	data, err := ioutil.ReadAll(apiContext.Request.Body)
	if err != nil {
		return errors.Wrap(err, "reading request body error")
	}
	switch actionName {
	case "rollback":
		input := client.MultiClusterAppRollbackInput{}
		if err = json.Unmarshal(data, &input); err != nil {
			return errors.Wrap(err, "unmarshal input error")
		}
		revision, err := w.MultiClusterAppRevisionLister.Get(namespace.GlobalNamespace, input.RevisionID)
		if err != nil {
			return err
		}
		obj, err := w.MultiClusterAppLister.Get(namespace.GlobalNamespace, mcApp.Name)
		if err != nil {
			return err
		}
		if obj.Status.RevisionName == revision.Name {
			return nil
		}
		toUpdate := obj.DeepCopy()
		toUpdate.Spec.TemplateVersionName = revision.TemplateVersionName
		toUpdate.Spec.Answers = revision.Answers
		_, err = w.MultiClusterApps.Update(toUpdate)
		return err
	}
	return nil
}
