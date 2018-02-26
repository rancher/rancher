package pipeline

import (
	"strings"

	"fmt"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/api/handler"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/utils"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/client/management/v3"
	"github.com/sirupsen/logrus"
	"net/http"
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
}

func (h *Handler) CreateHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	//update hooks endpoint for webhook
	if err := utils.UpdateEndpoint(apiContext.URLBuilder.Current()); err != nil {
		return err
	}
	return handler.CreateHandler(apiContext, next)
}

func (h *Handler) ActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	logrus.Debugf("do pipeline action:%s", actionName)

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
	parts := strings.Split(apiContext.ID, ":")
	ns := parts[0]
	name := parts[1]

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
	parts := strings.Split(apiContext.ID, ":")
	ns := parts[0]
	name := parts[1]
	pipeline, err := h.PipelineLister.Get(ns, name)
	if err != nil {
		return err
	}
	execution, err := utils.GenerateExecution(h.Pipelines, h.PipelineExecutions, pipeline, utils.TriggerTypeUser)
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
