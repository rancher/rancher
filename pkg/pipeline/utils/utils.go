package utils

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/pipeline/remote/model"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
	"strings"
	"time"
)

func initExecution(p *v3.Pipeline, config *v3.PipelineConfig) *v3.PipelineExecution {
	//add Clone stage/step at the start

	toRunConfig := configWithCloneStage(config)
	execution := &v3.PipelineExecution{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetNextExecutionName(p),
			Namespace: p.Namespace,
			Labels:    map[string]string{PipelineFinishLabel: ""},
		},
		Spec: v3.PipelineExecutionSpec{
			ProjectName:    p.Spec.ProjectName,
			PipelineName:   p.Namespace + ":" + p.Name,
			RepositoryURL:  p.Spec.RepositoryURL,
			Run:            p.Status.NextRun,
			PipelineConfig: *toRunConfig,
		},
	}
	execution.Status.ExecutionState = StateWaiting
	execution.Status.Started = time.Now().Format(time.RFC3339)
	execution.Status.Stages = make([]v3.StageStatus, len(toRunConfig.Stages))

	for i := 0; i < len(execution.Status.Stages); i++ {
		stage := &execution.Status.Stages[i]
		stage.State = StateWaiting
		stepsize := len(toRunConfig.Stages[i].Steps)
		stage.Steps = make([]v3.StepStatus, stepsize)
		for j := 0; j < stepsize; j++ {
			step := &stage.Steps[j]
			step.State = StateWaiting
		}
	}
	return execution
}

func configWithCloneStage(config *v3.PipelineConfig) *v3.PipelineConfig {
	result := config.DeepCopy()
	if len(config.Stages) > 0 && len(config.Stages[0].Steps) > 0 &&
		config.Stages[0].Steps[0].SourceCodeConfig != nil {
		return result
	}
	cloneStage := v3.Stage{
		Name:  "Clone",
		Steps: []v3.Step{{SourceCodeConfig: &v3.SourceCodeConfig{}}},
	}
	result.Stages = append([]v3.Stage{cloneStage}, result.Stages...)
	return result
}

func GetNextExecutionName(p *v3.Pipeline) string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("%s-%d", p.Name, p.Status.NextRun)
}

func IsStageSuccess(stage v3.StageStatus) bool {
	if stage.State == StateSuccess {
		return true
	} else if stage.State == StateFailed || stage.State == StateDenied {
		return false
	}
	successSteps := 0
	for _, step := range stage.Steps {
		if step.State == StateSuccess || step.State == StateSkipped {
			successSteps++
		}
	}
	return successSteps == len(stage.Steps)
}

func IsFinishState(state string) bool {
	if state == StateBuilding ||
		state == StateWaiting ||
		state == StateQueueing ||
		state == StatePending {
		return false
	}
	return true
}

func GenerateExecution(executions v3.PipelineExecutionInterface, pipeline *v3.Pipeline, pipelineConfig *v3.PipelineConfig, info *model.BuildInfo) (*v3.PipelineExecution, error) {

	//Generate a new pipeline execution
	execution := initExecution(pipeline, pipelineConfig)
	execution.Spec.TriggeredBy = info.TriggerType
	execution.Spec.TriggerUserName = info.TriggerUserName
	execution.Spec.Branch = info.Branch
	execution.Spec.Author = info.Author
	execution.Spec.AvatarURL = info.AvatarURL
	execution.Spec.Email = info.Email
	execution.Spec.Message = info.Message
	execution.Spec.HTMLLink = info.HTMLLink
	execution.Spec.Title = info.Title
	execution.Spec.Ref = info.Ref
	execution.Spec.Commit = info.Commit
	execution.Spec.Event = info.Event

	if !Match(execution.Spec.PipelineConfig.Branch, info.Branch) {
		logrus.Debug("conditions do not match")
		return nil, nil
	}

	execution, err := executions.Create(execution)
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

func ValidPipelineConfig(config v3.PipelineConfig) error {
	if len(config.Stages) < 1 ||
		len(config.Stages[0].Steps) < 1 ||
		config.Stages[0].Steps[0].SourceCodeConfig == nil {
		return fmt.Errorf("invalid definition for pipeline: expect souce code step at the start")
	}
	return nil
}

func GetPipelineCommonName(obj *v3.PipelineExecution) string {
	_, p := ref.Parse(obj.Spec.ProjectName)
	return p + PipelineNamespaceSuffix
}

func GetEnvVarMap(execution *v3.PipelineExecution) map[string]string {

	m := map[string]string{}
	repoURL := execution.Spec.RepositoryURL
	repoName := ""
	if strings.Contains(repoURL, "/") {
		trimmedURL := strings.TrimRight(repoURL, "/")
		idx := strings.LastIndex(trimmedURL, "/")
		repoName = strings.TrimSuffix(trimmedURL[idx+1:], ".git")
	}

	commit := execution.Spec.Commit
	if commit != "" && len(commit) > 7 {
		//use abbreviated SHA
		commit = commit[:7]
	}
	_, pipelineID := ref.Parse(execution.Spec.PipelineName)
	clusterID, projectID := ref.Parse(execution.Spec.ProjectName)

	m[EnvGitCommit] = commit
	m[EnvGitRepoName] = repoName
	m[EnvGitRef] = execution.Spec.Ref
	m[EnvGitBranch] = execution.Spec.Branch
	m[EnvGitURL] = execution.Spec.RepositoryURL
	m[EnvPipelineID] = pipelineID
	m[EnvTriggerType] = execution.Spec.TriggeredBy
	m[EnvEvent] = execution.Spec.Event
	m[EnvExecutionID] = execution.Name
	m[EnvExecutionSequence] = strconv.Itoa(execution.Spec.Run)
	m[EnvProjectID] = projectID
	m[EnvClusterID] = clusterID

	if execution.Spec.Event == WebhookEventTag {
		m[EnvGitTag] = strings.TrimPrefix(execution.Spec.Ref, "refs/tags/")
	}

	return m
}

func PipelineConfigToYaml(pipelineConfig *v3.PipelineConfig) ([]byte, error) {

	content, err := yaml.Marshal(pipelineConfig)
	if err != nil {
		return nil, err
	}

	return content, nil
}

func PipelineConfigFromYaml(content []byte) (*v3.PipelineConfig, error) {

	out := &v3.PipelineConfig{}
	err := yaml.Unmarshal(content, out)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to parse the pipeline file")
	}

	return out, nil
}
