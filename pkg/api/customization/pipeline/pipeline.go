package pipeline

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/api/handler"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/parse/builder"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/client/management/v3"
	"github.com/robfig/cron"
	"io/ioutil"
	"net/http"
	"time"
)

type Handler struct {
	Pipelines          v3.PipelineInterface
	PipelineLister     v3.PipelineLister
	PipelineExecutions v3.PipelineExecutionInterface
}

func Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "activate")
	resource.AddAction(apiContext, "deactivate")
	resource.AddAction(apiContext, "run")
	resource.Links["export"] = apiContext.URLBuilder.Link("export", resource)
	resource.Links["config"] = apiContext.URLBuilder.Link("config", resource)
}

func Validator(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	pipelineSpec := v3.PipelineSpec{}
	if err := convert.ToObj(data, &pipelineSpec); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, err.Error())
	}

	if !utils.ValidSourceCodeConfig(pipelineSpec) {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			"invalid pipeline definition, expected sourceCode step at the start")
	}

	if pipelineSpec.TriggerCronExpression != "" {
		sourceCodeConfig := pipelineSpec.Stages[0].Steps[0].SourceCodeConfig
		if sourceCodeConfig.BranchCondition == "all" || sourceCodeConfig.BranchCondition == "except" {
			return httperror.NewAPIError(httperror.InvalidBodyContent,
				"cron trigger only works for only branch option")
		}
		_, err := cron.ParseStandard(pipelineSpec.TriggerCronExpression)
		if err != nil {
			return httperror.NewAPIError(httperror.InvalidBodyContent,
				"error parse cron trigger")
		}
	}

	return nil
}

func (h *Handler) LinkHandler(apiContext *types.APIContext, next types.RequestHandler) error {

	ns, name := ref.Parse(apiContext.ID)

	if apiContext.Link == "export" {
		pipeline, err := h.PipelineLister.Get(ns, name)
		if err != nil {
			return err
		}

		content, err := toYaml(pipeline)
		if err != nil {
			return err
		}
		fileName := fmt.Sprintf("pipeline-%s.yaml", pipeline.Spec.DisplayName)
		apiContext.Response.Header().Add("Content-Disposition", "attachment; filename="+fileName)
		http.ServeContent(apiContext.Response, apiContext.Request, fileName, time.Now(), bytes.NewReader(content))
		return nil
	} else if apiContext.Link == "config" {
		pipeline, err := h.PipelineLister.Get(ns, name)
		if err != nil {
			return err
		}

		content, err := toYaml(pipeline)
		if err != nil {
			return err
		}
		_, err = apiContext.Response.Write([]byte(content))
		return err
	}

	return httperror.NewAPIError(httperror.NotFound, "Link not found")
}

func (h *Handler) CreateHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	//update hooks endpoint for webhook
	if err := utils.UpdateEndpoint(apiContext.URLBuilder.Current()); err != nil {
		return err
	}
	data, err := parse.Body(apiContext.Request)
	if err != nil {
		return err
	}
	//handler import
	if data != nil && data["templates"] != nil {

		templates, ok := data["templates"].(map[string]interface{})
		if !ok {
			return httperror.NewAPIError(httperror.InvalidBodyContent,
				"error invalid templates format")
		}

		store := apiContext.Schema.Store
		if store == nil {
			return httperror.NewAPIError(httperror.NotFound, "no store found")
		}

		for _, val := range templates {
			valStr, ok := val.(string)
			if !ok {
				return httperror.NewAPIError(httperror.InvalidBodyContent,
					"error invalid template format")
			}

			pipelineMap, err := fromYaml([]byte(valStr))
			if err != nil {
				return err
			}
			pipelineMap["projectId"] = data["projectId"]

			if err := createData(apiContext, pipelineMap); err != nil {
				return err
			}
		}

		apiContext.WriteResponse(http.StatusCreated, nil)
		return nil

	}

	return createData(apiContext, data)
}

func (h *Handler) UpdateHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	//update hooks endpoint for webhook
	if err := utils.UpdateEndpoint(apiContext.URLBuilder.Current()); err != nil {
		return err
	}
	return handler.UpdateHandler(apiContext, next)
}

func (h *Handler) ActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {

	switch actionName {
	case "activate":
		return h.changeState(apiContext, "inactive", "active")
	case "deactivate":
		return h.changeState(apiContext, "active", "inactive")
	case "run":
		return h.run(apiContext)
	}
	return httperror.NewAPIError(httperror.InvalidAction, "unsupported action")
}

func (h *Handler) changeState(apiContext *types.APIContext, curState, newState string) error {

	ns, name := ref.Parse(apiContext.ID)
	pipeline, err := h.PipelineLister.Get(ns, name)
	if err != nil {
		return err
	}

	if pipeline.Status.PipelineState == curState {
		pipeline.Status.PipelineState = newState
		if _, err = h.Pipelines.Update(pipeline); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("Error resource is not %s", curState)
	}

	data := map[string]interface{}{}
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &data); err != nil {
		return err
	}

	apiContext.WriteResponse(http.StatusOK, data)
	return nil
}

func (h *Handler) run(apiContext *types.APIContext) error {

	ns, name := ref.Parse(apiContext.ID)
	pipeline, err := h.PipelineLister.Get(ns, name)
	if err != nil {
		return err
	}
	runPipelineInput := v3.RunPipelineInput{}
	requestBytes, err := ioutil.ReadAll(apiContext.Request.Body)
	if err != nil {
		return err
	}
	if string(requestBytes) != "" {
		if err := json.Unmarshal(requestBytes, &runPipelineInput); err != nil {
			return err
		}
	}
	if !utils.ValidSourceCodeConfig(pipeline.Spec) {
		return errors.New("Error invalid pipeline definition")
	}
	branch := runPipelineInput.Branch
	branchCondition := pipeline.Spec.Stages[0].Steps[0].SourceCodeConfig.BranchCondition
	if branchCondition == "except" || branchCondition == "all" {
		if branch == "" {
			return httperror.NewAPIError(httperror.InvalidBodyContent, "Error branch is not specified for the pipeline to run")
		}
	} else {
		branch = ""
	}

	userName := apiContext.Request.Header.Get("Impersonate-User")
	execution, err := utils.GenerateExecution(h.Pipelines, h.PipelineExecutions, pipeline, utils.TriggerTypeUser, userName, runPipelineInput.Branch)
	if err != nil {
		return err
	}

	data := map[string]interface{}{}
	if err := access.ByID(apiContext, apiContext.Version, client.PipelineExecutionType, ns+":"+execution.Name, &data); err != nil {
		return err
	}

	apiContext.WriteResponse(http.StatusOK, data)
	return err
}

func validSourceCodeConfig(spec v3.PipelineSpec) bool {
	if len(spec.Stages) < 1 ||
		len(spec.Stages[0].Steps) < 1 ||
		spec.Stages[0].Steps[0].SourceCodeConfig == nil {
		return false
	}
	return true
}

func createData(apiContext *types.APIContext, data map[string]interface{}) error {
	var err error

	for key, value := range apiContext.SubContextAttributeProvider.Create(apiContext, apiContext.Schema) {
		if data == nil {
			data = map[string]interface{}{}
		}
		data[key] = value
	}

	b := builder.NewBuilder(apiContext)

	op := builder.Create
	data, err = b.Construct(apiContext.Schema, data, op)
	if err != nil {
		return err
	}

	store := apiContext.Schema.Store
	if store == nil {
		return httperror.NewAPIError(httperror.NotFound, "no store found")
	}

	_, err = store.Create(apiContext, apiContext.Schema, data)

	return err
}
