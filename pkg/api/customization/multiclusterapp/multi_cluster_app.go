package multiclusterapp

import (
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	gaccess "github.com/rancher/rancher/pkg/api/customization/globalnamespaceaccess"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	pv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/client/management/v3"
)

type Wrapper struct {
	MultiClusterApps              v3.MultiClusterAppInterface
	MultiClusterAppLister         v3.MultiClusterAppLister
	MultiClusterAppRevisionLister v3.MultiClusterAppRevisionLister
	Users                         v3.UserInterface
	PrtbLister                    v3.ProjectRoleTemplateBindingLister
	RoleTemplateLister            v3.RoleTemplateLister
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
		sort.SliceStable(filtered, func(i, j int) bool {
			val1, err := time.Parse(time.RFC3339, convert.ToString(values.GetValueN(filtered[i], "created")))
			if err != nil {
				logrus.Infof("error parsing time %v", err)
			}
			val2, err := time.Parse(time.RFC3339, convert.ToString(values.GetValueN(filtered[j], "created")))
			if err != nil {
				logrus.Infof("error parsing time %v", err)
			}
			return val1.After(val2)
		})
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
		id := input.RevisionID
		splitID := strings.Split(input.RevisionID, ":")
		if len(splitID) == 2 {
			id = splitID[1]
		}
		revision, err := w.MultiClusterAppRevisionLister.Get(namespace.GlobalNamespace, id)
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

func (w Wrapper) Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	if request.Method != http.MethodPut && request.Method != http.MethodPost {
		return nil
	}

	var targetProjects []string
	// check if the creator of multiclusterapp and all members can access all target projects
	targetMapSlice, found := values.GetSlice(data, client.MultiClusterAppFieldTargets)
	if !found {
		return fmt.Errorf("no target projects provided")
	}
	for _, t := range targetMapSlice {
		projectID, ok := t[client.TargetFieldProjectID].(string)
		if !ok {
			continue
		}
		targetProjects = append(targetProjects, projectID)
	}

	ma := gaccess.MemberAccess{
		Users:              w.Users,
		PrtbLister:         w.PrtbLister,
		RoleTemplateLister: w.RoleTemplateLister,
	}
	return ma.CheckCreatorAndMembersAccessToTargets(request, targetProjects, data, client.MultiClusterAppType, pv3.AppGroupVersionKind.Group, pv3.AppResource.Name)
}
