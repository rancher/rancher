package clustertemplate

import (
	"net/http"
	"sort"
	"time"

	"encoding/json"
	"fmt"

	"strings"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	client "github.com/rancher/types/client/management/v3"
	"github.com/sirupsen/logrus"
)

type Wrapper struct {
	ClusterTemplates              v3.ClusterTemplateInterface
	ClusterTemplateLister         v3.ClusterTemplateLister
	ClusterTemplateRevisionLister v3.ClusterTemplateRevisionLister
	ClusterTemplateRevisions      v3.ClusterTemplateRevisionInterface
	ClusterTemplateQuestions      []v3.Question
}

func (w Wrapper) Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.Links["revisions"] = apiContext.URLBuilder.Link("revisions", resource)
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
	if actionName != "listquestions" {
		return httperror.NewAPIError(httperror.NotFound, "not found")
	}

	questionsOutput := v3.ClusterTemplateQuestionsOutput{}

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

func (w Wrapper) BuildQuestionsFromSchema(schema *types.Schema, schemas *types.Schemas, pathTofield string) []v3.Question {
	questions := []v3.Question{}
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
			newQuestion := v3.Question{}
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
