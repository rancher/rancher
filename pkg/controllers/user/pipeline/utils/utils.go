package utils

import (
	"fmt"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/url"
	"strings"
	"time"
)

var CIEndpoint = ""

func InitClusterPipeline(clusterPipelines v3.ClusterPipelineInterface, clusterName string) error {
	clusterPipeline := &v3.ClusterPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: clusterName,
		},
		Spec: v3.ClusterPipelineSpec{
			ClusterName: clusterName,
		},
	}

	if _, err := clusterPipelines.Create(clusterPipeline); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func IsPipelineDeploy(clusterPipelineLister v3.ClusterPipelineLister, clusterName string) bool {
	clusterPipeline, err := clusterPipelineLister.Get(clusterName, clusterName)
	if err != nil {
		logrus.Errorf("Error get clusterpipeline - %v", err)
		return false
	}
	return clusterPipeline.Spec.Deploy
}

func InitExecution(p *v3.Pipeline, triggerType string, triggerUserName string, branch string, commit string, envVars map[string]string) *v3.PipelineExecution {
	execution := &v3.PipelineExecution{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getNextExecutionName(p),
			Namespace: p.Namespace,
			Labels:    map[string]string{PipelineFinishLabel: "false"},
		},
		ProjectName: p.ProjectName,
		Spec: v3.PipelineExecutionSpec{
			PipelineName:    p.Namespace + ":" + p.Name,
			Run:             p.Status.NextRun,
			TriggeredBy:     triggerType,
			TriggerUserName: triggerUserName,
			Pipeline:        *p.DeepCopy(),
		},
	}
	if branch != "" {
		setExecutionBranch(execution, branch)
	}
	if commit != "" {
		execution.Status.Commit = commit
	}
	execution.Status.ExecutionState = StateWaiting
	execution.Status.Started = time.Now().Format(time.RFC3339)
	execution.Status.Stages = make([]v3.StageStatus, len(p.Spec.Stages))
	execution.Status.EnvVars = envVars

	for i := 0; i < len(execution.Status.Stages); i++ {
		stage := &execution.Status.Stages[i]
		stage.State = StateWaiting
		stepsize := len(p.Spec.Stages[i].Steps)
		stage.Steps = make([]v3.StepStatus, stepsize)
		for j := 0; j < stepsize; j++ {
			step := &stage.Steps[j]
			step.State = StateWaiting
		}
	}
	return execution
}

func getNextExecutionName(p *v3.Pipeline) string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("%s-%d", p.Name, p.Status.NextRun)
}

func setExecutionBranch(execution *v3.PipelineExecution, branch string) {
	if len(execution.Spec.Pipeline.Spec.Stages) < 1 ||
		len(execution.Spec.Pipeline.Spec.Stages[0].Steps) < 1 ||
		execution.Spec.Pipeline.Spec.Stages[0].Steps[0].SourceCodeConfig == nil {
		return
	}
	sourceCodeconfig := execution.Spec.Pipeline.Spec.Stages[0].Steps[0].SourceCodeConfig
	sourceCodeconfig.BranchCondition = "only"
	sourceCodeconfig.Branch = branch
}

func IsStageSuccess(stage v3.StageStatus) bool {
	if stage.State == StateSuccess {
		return true
	} else if stage.State == StateFail || stage.State == StateDenied {
		return false
	}
	successSteps := 0
	for _, step := range stage.Steps {
		if step.State == StateSuccess || step.State == StateSkip {
			successSteps++
		}
	}
	return successSteps == len(stage.Steps)
}

func UpdateEndpoint(requestURL string) error {
	u, err := url.Parse(requestURL)
	if err != nil {
		return err
	}
	CIEndpoint = fmt.Sprintf("%s://%s/hooks", u.Scheme, u.Host)
	return nil
}

func IsExecutionFinish(execution *v3.PipelineExecution) bool {
	if execution == nil {
		return false
	}
	if execution.Status.ExecutionState != StateWaiting && execution.Status.ExecutionState != StateBuilding {
		return true
	}
	return false
}

func GenerateExecution(pipelines v3.PipelineInterface, executions v3.PipelineExecutionInterface, pipeline *v3.Pipeline, triggerType string, triggerUserName string, branch string, commit string, envVars map[string]string) (*v3.PipelineExecution, error) {

	//Generate a new pipeline execution
	execution := InitExecution(pipeline, triggerType, triggerUserName, branch, commit, envVars)
	execution, err := executions.Create(execution)
	if err != nil {
		return nil, err
	}
	//update pipeline status
	pipeline.Status.NextRun++
	pipeline.Status.LastExecutionID = pipeline.Namespace + ":" + execution.Name
	pipeline.Status.LastStarted = time.Now().Format(time.RFC3339)
	pipeline.Status.LastRunState = StateWaiting
	_, err = pipelines.Update(pipeline)
	if err != nil {
		return nil, err
	}
	return execution, nil
}

func SplitImageTag(image string) (string, string, string) {
	registry, repo, tag := "", "", ""
	i := strings.Index(image, "/")
	if i == -1 || (!strings.ContainsAny(image[:i], ".:") && image[:i] != "localhost") {
		registry = DefaultRegistry
	} else {
		registry = image[:i]
		image = image[i+1:]
	}
	i = strings.Index(image, ":")
	if i == -1 {
		repo = image
		tag = DefaultTag
	} else {
		repo = image[:i]
		tag = image[i+1:]
	}
	return registry, repo, tag
}

func ValidPipelineSpec(spec v3.PipelineSpec) error {
	if len(spec.Stages) < 1 ||
		len(spec.Stages[0].Steps) < 1 ||
		spec.Stages[0].Steps[0].SourceCodeConfig == nil {
		return fmt.Errorf("invalid definition for pipeline '%s': expect souce code step at the start", spec.DisplayName)
	}
	return nil
}
