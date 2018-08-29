package pipeline

import (
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/client/project/v3"
	"net/http"
	"time"
)

const (
	executionStateField = "executionState"
	actionRerun         = "rerun"
	actionStop          = "stop"
	linkLog             = "log"
)

type ExecutionHandler struct {
	ClusterManager *clustermanager.Manager

	PipelineLister          v3.PipelineLister
	PipelineExecutionLister v3.PipelineExecutionLister
	PipelineExecutions      v3.PipelineExecutionInterface
}

func (h *ExecutionHandler) ExecutionFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	if e := convert.ToString(resource.Values[executionStateField]); utils.IsFinishState(e) {
		resource.AddAction(apiContext, actionRerun)
	}
	if e := convert.ToString(resource.Values[executionStateField]); !utils.IsFinishState(e) {
		resource.AddAction(apiContext, actionStop)
	}
	resource.Links[linkLog] = apiContext.URLBuilder.Link(linkLog, resource)
}

func (h *ExecutionHandler) LinkHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	if apiContext.Link == linkLog {
		return h.handleLog(apiContext)
	}

	return httperror.NewAPIError(httperror.NotFound, "Link not found")

}

func (h *ExecutionHandler) ActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	switch actionName {
	case actionRerun:
		return h.rerun(apiContext)
	case actionStop:
		return h.stop(apiContext)
	}

	return httperror.NewAPIError(httperror.InvalidAction, "unsupported action")
}

func (h *ExecutionHandler) rerun(apiContext *types.APIContext) error {
	ns, name := ref.Parse(apiContext.ID)
	execution, err := h.PipelineExecutionLister.Get(ns, name)
	if err != nil {
		return err
	}
	ns, name = ref.Parse(execution.Spec.PipelineName)
	pipeline, err := h.PipelineLister.Get(ns, name)
	if err != nil {
		return err
	}

	//rerun triggers a new execution with original configs
	toCreate := execution.DeepCopy()
	toCreate.ResourceVersion = ""
	toCreate.Name = utils.GetNextExecutionName(pipeline)
	toCreate.Labels = map[string]string{utils.PipelineFinishLabel: ""}
	toCreate.Spec.Run = pipeline.Status.NextRun

	toCreate.Status.ExecutionState = utils.StateWaiting
	toCreate.Status.Started = time.Now().Format(time.RFC3339)
	toCreate.Status.Ended = ""
	toCreate.Status.Conditions = nil
	for i := 0; i < len(toCreate.Status.Stages); i++ {
		stage := &toCreate.Status.Stages[i]
		stage.State = utils.StateWaiting
		stage.Started = ""
		stage.Ended = ""
		for j := 0; j < len(stage.Steps); j++ {
			step := &stage.Steps[j]
			step.State = utils.StateWaiting
			step.Started = ""
			step.Ended = ""
		}
	}
	if _, err := h.PipelineExecutions.Create(toCreate); err != nil {
		return err
	}

	data := map[string]interface{}{}
	if err := access.ByID(apiContext, apiContext.Version, client.PipelineExecutionType, ref.Ref(toCreate), &data); err != nil {
		return err
	}
	apiContext.WriteResponse(http.StatusOK, data)
	return nil
}

func (h *ExecutionHandler) stop(apiContext *types.APIContext) error {
	ns, name := ref.Parse(apiContext.ID)
	execution, err := h.PipelineExecutionLister.Get(ns, name)
	if err != nil {
		return err
	}

	if utils.IsFinishState(execution.Status.ExecutionState) {
		return httperror.NewAPIError(httperror.InvalidAction, "pipeline execution is already finished")
	}

	toUpdate := execution.DeepCopy()
	toUpdate.Status.ExecutionState = utils.StateAborted
	toUpdate.Status.Ended = time.Now().Format(time.RFC3339)
	toUpdate.Labels[utils.PipelineFinishLabel] = "true"
	if _, err := h.PipelineExecutions.Update(toUpdate); err != nil {
		return err
	}
	return nil
}
