package drivers

import (
	"fmt"
	"net/http"

	"github.com/rancher/rancher/pkg/pipeline/providers"
	"github.com/rancher/rancher/pkg/pipeline/remote/model"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	v3 "github.com/rancher/rancher/pkg/types/apis/project.cattle.io/v3"
)

const (
	RefsBranchPrefix = "refs/heads/"
	RefsTagPrefix    = "refs/tags/"
)

func isEventActivated(info *model.BuildInfo, pipeline *v3.Pipeline) bool {
	if (info.Event == utils.WebhookEventPush && pipeline.Spec.TriggerWebhookPush) ||
		(info.Event == utils.WebhookEventTag && pipeline.Spec.TriggerWebhookTag) ||
		(info.Event == utils.WebhookEventPullRequest && pipeline.Spec.TriggerWebhookPr) {
		return true
	}
	return false
}

func validateAndGeneratePipelineExecution(pipelineExecutions v3.PipelineExecutionInterface,
	sourceCodeCredentials v3.SourceCodeCredentialInterface,
	sourceCodeCredentialLister v3.SourceCodeCredentialLister,
	info *model.BuildInfo,
	pipeline *v3.Pipeline) (int, error) {

	if !isEventActivated(info, pipeline) {
		return http.StatusUnavailableForLegalReasons, fmt.Errorf("trigger for event '%s' is disabled", info.Event)
	}

	pipelineConfig, err := providers.GetPipelineConfigByBranch(sourceCodeCredentials, sourceCodeCredentialLister, pipeline, info.Branch)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	if pipelineConfig == nil {
		//no pipeline config to run
		return http.StatusOK, nil
	}

	if !utils.Match(pipelineConfig.Branch, info.Branch) {
		return http.StatusUnavailableForLegalReasons, fmt.Errorf("skipped branch '%s'", info.Branch)
	}

	if _, err := utils.GenerateExecution(pipelineExecutions, pipeline, pipelineConfig, info); err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}
