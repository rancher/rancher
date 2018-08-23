package jenkins

import (
	"fmt"
	"regexp"
	"strings"

	images "github.com/rancher/rancher/pkg/image"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
	mv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/project.cattle.io/v3"
)

func toJenkinsStep(execution *v3.PipelineExecution, stageOrdinal int, stepOrdinal int) PipelineStep {
	stage := execution.Spec.PipelineConfig.Stages[stageOrdinal]
	step := &stage.Steps[stepOrdinal]
	var pStep PipelineStep

	if step.SourceCodeConfig != nil {
		pStep = convertSourceCodeConfig(execution, step)
	} else if step.RunScriptConfig != nil {
		pStep = convertRunScriptconfig(execution, step)
	} else if step.PublishImageConfig != nil {
		pStep = convertPublishImageconfig(execution, step)
	} else if step.ApplyYamlConfig != nil {
		pStep = convertApplyYamlconfig(execution, step, stageOrdinal)
	}

	if !utils.MatchAll(stage.When, execution) || !utils.MatchAll(step.When, execution) {
		stepName := fmt.Sprintf("step-%d-%d", stageOrdinal, stepOrdinal)
		pStep.command = fmt.Sprintf(markSkipScript, stepName)
	}

	return pStep
}

func convertSourceCodeConfig(execution *v3.PipelineExecution, step *v3.Step) PipelineStep {
	pStep := PipelineStep{}
	pStep.command = fmt.Sprintf("checkout([$class: 'GitSCM', branches: [[name: 'local/temp']], userRemoteConfigs: [[url: '%s', refspec: '+%s:refs/remotes/local/temp', credentialsId: '%s']]])",
		execution.Spec.RepositoryURL, execution.Spec.Ref, execution.Name)
	pStep.image = images.Resolve(mv3.ToolsSystemImages.PipelineSystemImages.AlpineGit)
	pStep.containerOptions = getStepContainerOptions(execution, step.Privileged, step.Env, step.EnvFrom)

	return pStep
}

func convertRunScriptconfig(execution *v3.PipelineExecution, step *v3.Step) PipelineStep {
	config := step.RunScriptConfig
	pStep := PipelineStep{}

	pStep.image = config.Image
	pStep.command = fmt.Sprintf(`sh ''' %s '''`, config.ShellScript)
	pStep.containerOptions = getStepContainerOptions(execution, step.Privileged, step.Env, step.EnvFrom)

	return pStep
}

func convertPublishImageconfig(execution *v3.PipelineExecution, step *v3.Step) PipelineStep {
	config := step.PublishImageConfig
	pStep := PipelineStep{}
	m := utils.GetEnvVarMap(execution)
	config.Tag = substituteEnvVar(m, config.Tag)

	registry, repo, tag := utils.SplitImageTag(config.Tag)

	if config.PushRemote {
		registry = config.Registry
	} else {
		_, projectID := ref.Parse(execution.Spec.ProjectName)
		registry = fmt.Sprintf("%s.%s-pipeline", utils.LocalRegistry, projectID)
	}

	reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
	processedRegistry := strings.ToLower(reg.ReplaceAllString(registry, ""))
	secretName := fmt.Sprintf("%s-%s", execution.Namespace, processedRegistry)
	secretUserKey := utils.PublishSecretUserKey
	secretPwKey := utils.PublishSecretPwKey
	if !config.PushRemote {
		//use local registry credential
		secretName = utils.PipelineSecretName
		secretUserKey = utils.PipelineSecretUserKey
		secretPwKey = utils.PipelineSecretTokenKey
	}
	pluginRepo := fmt.Sprintf("%s/%s", registry, repo)
	if registry == utils.DefaultRegistry {
		//the `plugins/docker` image fails when setting DOCKER_REGISTRY to index.docker.io
		registry = ""
	}

	pStep.image = images.Resolve(mv3.ToolsSystemImages.PipelineSystemImages.PluginsDocker)
	pStep.command = `sh '''/usr/local/bin/dockerd-entrypoint.sh /bin/drone-docker'''`
	publishEnv := map[string]string{
		"DOCKER_REGISTRY":   registry,
		"PLUGIN_REPO":       pluginRepo,
		"PLUGIN_TAG":        tag,
		"PLUGIN_DOCKERFILE": config.DockerfilePath,
		"PLUGIN_CONTEXT":    config.BuildContext,
	}

	for k, v := range step.Env {
		publishEnv[k] = v
	}
	envFrom := append(step.EnvFrom, v3.EnvFrom{
		SourceName: secretName,
		SourceKey:  secretUserKey,
		TargetKey:  "DOCKER_USERNAME",
	}, v3.EnvFrom{
		SourceName: secretName,
		SourceKey:  secretPwKey,
		TargetKey:  "DOCKER_PASSWORD",
	})

	pStep.containerOptions = getStepContainerOptions(execution, true, publishEnv, envFrom)

	return pStep
}

func convertApplyYamlconfig(execution *v3.PipelineExecution, step *v3.Step, stageOrdinal int) PipelineStep {
	config := step.ApplyYamlConfig
	pStep := PipelineStep{}

	pStep.image = images.Resolve(mv3.ToolsSystemImages.PipelineSystemImages.KubeApply)

	pStep.command = `sh ''' kube-apply '''`

	applyEnv := map[string]string{
		"YAML_PATH":    config.Path,
		"YAML_CONTENT": config.Content,
		"NAMESPACE":    config.Namespace,
	}

	//for deploy step, get registry & image variable from a previous publish step
	var registry, imageRepo string
StageLoop:
	for i := stageOrdinal; i >= 0; i-- {
		stage := execution.Spec.PipelineConfig.Stages[i]
		for j := len(stage.Steps) - 1; j >= 0; j-- {
			step := stage.Steps[j]
			if step.PublishImageConfig != nil {
				config := step.PublishImageConfig
				if config.PushRemote {
					registry = step.PublishImageConfig.Registry
				}
				_, imageRepo, _ = utils.SplitImageTag(step.PublishImageConfig.Tag)
				break StageLoop
			}
		}
	}

	applyEnv[utils.EnvRegistry] = registry
	applyEnv[utils.EnvImageRepo] = imageRepo

	for k, v := range step.Env {
		applyEnv[k] = v
	}
	pStep.containerOptions = getStepContainerOptions(execution, step.Privileged, applyEnv, step.EnvFrom)

	return pStep
}
