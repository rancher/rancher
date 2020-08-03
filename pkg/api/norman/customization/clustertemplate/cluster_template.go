package clustertemplate

import (
	"net/http"
	"sort"
	"time"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"encoding/json"
	"fmt"

	"strings"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	managementschema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	enableRevisionAction  = "enable"
	disableRevisionAction = "disable"
)

type Wrapper struct {
	ClusterTemplates              v3.ClusterTemplateInterface
	ClusterTemplateLister         v3.ClusterTemplateLister
	ClusterTemplateRevisionLister v3.ClusterTemplateRevisionLister
	ClusterTemplateRevisions      v3.ClusterTemplateRevisionInterface
	ClusterTemplateQuestions      []v32.Question
}

func (w Wrapper) Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.Links["revisions"] = apiContext.URLBuilder.Link("revisions", resource)
}

func (w Wrapper) RevisionFormatter(apiContext *types.APIContext, resource *types.RawResource) {

	if err := apiContext.AccessControl.CanDo(v3.ClusterTemplateRevisionGroupVersionKind.Group, v3.ClusterTemplateRevisionResource.Name, "update", apiContext, resource.Values, apiContext.Schema); err == nil {
		if convert.ToBool(resource.Values["enabled"]) {
			resource.AddAction(apiContext, disableRevisionAction)
		} else {
			resource.AddAction(apiContext, enableRevisionAction)
		}
	}
}

func (w Wrapper) CollectionFormatter(request *types.APIContext, collection *types.GenericCollection) {
	collection.AddAction(request, "listquestions")
}

func (w Wrapper) LinkHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	switch apiContext.Link {
	case "revisions":
		var template client.ClusterTemplate
		if err := access.ByID(apiContext, &managementschema.Version, client.ClusterTemplateType, apiContext.ID, &template); err != nil {
			return err
		}
		conditions := []*types.QueryCondition{
			types.NewConditionFromString(client.ClusterTemplateRevisionFieldClusterTemplateID, types.ModifierEQ, []string{template.ID}...),
		}
		var templateVersions []map[string]interface{}
		if err := access.List(apiContext, &managementschema.Version, client.ClusterTemplateRevisionType, &types.QueryOptions{Conditions: conditions}, &templateVersions); err != nil {
			return err
		}
		sort.SliceStable(templateVersions, func(i, j int) bool {
			val1, err := time.Parse(time.RFC3339, convert.ToString(values.GetValueN(templateVersions[i], "created")))
			if err != nil {
				logrus.Infof("error parsing time %v", err)
			}
			val2, err := time.Parse(time.RFC3339, convert.ToString(values.GetValueN(templateVersions[j], "created")))
			if err != nil {
				logrus.Infof("error parsing time %v", err)
			}
			return val1.After(val2)
		})
		apiContext.Type = client.ClusterTemplateRevisionType
		apiContext.WriteResponse(http.StatusOK, templateVersions)
		return nil
	}
	return nil
}

func (w Wrapper) ClusterTemplateRevisionsActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {

	canUpdateClusterTemplateRevision := func() bool {
		revision := map[string]interface{}{
			"id": apiContext.ID,
		}

		return apiContext.AccessControl.CanDo(v3.ClusterTemplateRevisionGroupVersionKind.Group, v3.ClusterTemplateRevisionResource.Name, "update", apiContext, revision, apiContext.Schema) == nil
	}

	switch actionName {
	case "disable":
		if !canUpdateClusterTemplateRevision() {
			return httperror.NewAPIError(httperror.PermissionDenied, "can not access clusterTemplateRevision")
		}
		return w.updateEnabledFlagOnRevision(apiContext, false)
	case "enable":
		if !canUpdateClusterTemplateRevision() {
			return httperror.NewAPIError(httperror.PermissionDenied, "can not access clusterTemplateRevision")
		}
		return w.updateEnabledFlagOnRevision(apiContext, true)
	case "listquestions":
		return w.listRevisionQuestions(actionName, action, apiContext)

	}
	return httperror.NewAPIError(httperror.NotFound, "not found")
}

func (w Wrapper) updateEnabledFlagOnRevision(apiContext *types.APIContext, enabledFlag bool) error {

	revision, err := w.loadRevision(apiContext)
	if err != nil {
		return httperror.WrapAPIError(err, httperror.ServerError, "failed to load clusterTemplateRevision")
	}

	if revision.Spec.Enabled != nil && enabledFlag == *revision.Spec.Enabled {
		apiContext.WriteResponse(http.StatusNoContent, map[string]interface{}{})
		return nil
	}
	revisionCopy := revision.DeepCopy()
	revisionCopy.Spec.Enabled = &enabledFlag
	_, err = w.ClusterTemplateRevisions.Update(revisionCopy)

	if err != nil {
		//if conflict update, retry by loading from the store
		if apierrors.IsConflict(err) {
			revisionFromStore, err := w.ClusterTemplateRevisions.GetNamespaced(namespace.GlobalNamespace, revision.ObjectMeta.Name, v1.GetOptions{})
			if err != nil {
				return httperror.WrapAPIError(err, httperror.ServerError, "failed to load clusterTemplateRevision from store")
			}
			revisionStoreCopy := revisionFromStore.DeepCopy()
			revisionStoreCopy.Spec.Enabled = &enabledFlag

			_, err = w.ClusterTemplateRevisions.Update(revisionStoreCopy)
			if err != nil {
				return httperror.WrapAPIError(err, httperror.ServerError, fmt.Sprintf("failed to set enabled flag to %v on clusterTemplateRevision", enabledFlag))
			}
		}
		return httperror.WrapAPIError(err, httperror.ServerError, fmt.Sprintf("failed to set enabled flag to %v on clusterTemplateRevision", enabledFlag))
	}

	apiContext.WriteResponse(http.StatusNoContent, map[string]interface{}{})
	return nil
}

func (w Wrapper) loadRevision(apiContext *types.APIContext) (*v3.ClusterTemplateRevision, error) {
	//load the templaterevision
	split := strings.SplitN(apiContext.ID, ":", 2)
	if len(split) != 2 {
		return nil, httperror.NewAPIError(httperror.InvalidFormat, fmt.Sprintf("error in splitting clusterTemplateRevision name %v", apiContext.ID))
	}
	revisionName := split[1]

	revision, err := w.ClusterTemplateRevisionLister.Get(namespace.GlobalNamespace, revisionName)
	if err != nil {
		return nil, httperror.WrapAPIError(err, httperror.NotFound, "clusterTemplateRevision is not found")
	}
	if revision.DeletionTimestamp != nil {
		return nil, httperror.NewAPIError(httperror.InvalidType, "clusterTemplateRevision is marked for deletion")
	}
	return revision, nil
}

func (w Wrapper) listRevisionQuestions(actionName string, action *types.Action, apiContext *types.APIContext) error {
	questionsOutput := v32.ClusterTemplateQuestionsOutput{}

	if len(w.ClusterTemplateQuestions) == 0 {
		w.ClusterTemplateQuestions = w.BuildQuestionsFromSchema(apiContext.Schemas.Schema(&managementschema.Version, client.ClusterSpecBaseType), apiContext.Schemas, "")
	}
	questionsOutput.Questions = w.ClusterTemplateQuestions
	res, err := json.Marshal(questionsOutput)
	if err != nil {
		return httperror.WrapAPIError(err, httperror.ServerError, fmt.Sprintf("Error marshalling the Cluster Template questions output, %v", err))
	}
	apiContext.Response.Write(res)

	return nil
}

func (w Wrapper) BuildQuestionsFromSchema(schema *types.Schema, schemas *types.Schemas, pathTofield string) []v32.Question {
	questions := []v32.Question{}
	for name, field := range schema.ResourceFields {
		fieldType := field.Type
		if strings.HasPrefix(fieldType, "array") {
			fieldType = strings.Split(fieldType, "[")[1]
			fieldType = fieldType[:len(fieldType)-1]
		}
		checkSchema := schemas.Schema(&managementschema.Version, fieldType)
		if checkSchema != nil {
			subPath := name
			if pathTofield != "" {
				subPath = pathTofield + "." + name
			}
			subQuestions := w.BuildQuestionsFromSchema(checkSchema, schemas, subPath)
			if len(subQuestions) > 0 {
				questions = append(questions, subQuestions...)
			}
		} else {
			//add a Question
			newQuestion := v32.Question{}
			if field.Type == "password" {
				newQuestion.Group = "password"
			}
			newQuestion.Variable = name
			if pathTofield != "" {
				newQuestion.Variable = pathTofield + "." + name
			}
			newQuestion.Type = fieldType
			newQuestion.Description = field.Description
			newQuestion.Default = convert.ToString(field.Default)
			newQuestion.Required = field.Required
			questions = append(questions, newQuestion)
		}
	}
	return questions
}
