package jenkins

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	images "github.com/rancher/rancher/pkg/image"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	mv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/project.cattle.io/v3"
)

func ConvertPipelineExecutionToJenkinsPipeline(execution *v3.PipelineExecution) (*PipelineJob, error) {
	if execution == nil {
		return nil, errors.New("nil pipeline execution")
	}

	if err := utils.ValidPipelineConfig(execution.Spec.PipelineConfig); err != nil {
		return nil, err
	}

	copyExecution := execution.DeepCopy()
	parsePreservedEnvVar(copyExecution)
	pipelineJob := &PipelineJob{
		Plugin: WorkflowJobPlugin,
		Definition: Definition{
			Class:   FlowDefinitionClass,
			Plugin:  FlowDefinitionPlugin,
			Sandbox: true,
			Script:  convertPipelineExecution(copyExecution),
		},
	}
	return pipelineJob, nil
}

func convertStep(execution *v3.PipelineExecution, stageOrdinal int, stepOrdinal int) string {
	stepName := fmt.Sprintf("step-%d-%d", stageOrdinal, stepOrdinal)

	jStep := toJenkinsStep(execution, stageOrdinal, stepOrdinal)

	return fmt.Sprintf(stepBlock, stepName, stepName, stepName, jStep.command)
}

func convertStage(execution *v3.PipelineExecution, stageOrdinal int) string {
	var buffer bytes.Buffer
	pipelineConfig := execution.Spec.PipelineConfig
	stage := pipelineConfig.Stages[stageOrdinal]
	for i := range stage.Steps {
		buffer.WriteString(convertStep(execution, stageOrdinal, i))
		if i != len(stage.Steps)-1 {
			buffer.WriteString(",")
		}
	}
	skipOption := ""
	if !utils.MatchAll(stage.When, execution) {
		skipOption = fmt.Sprintf(markSkipScript, stage.Name)
	}

	return fmt.Sprintf(stageBlock, stage.Name, skipOption, buffer.String())
}

func convertPipelineExecution(execution *v3.PipelineExecution) string {
	var containerbuffer bytes.Buffer
	var pipelinebuffer bytes.Buffer
	for j, stage := range execution.Spec.PipelineConfig.Stages {
		pipelinebuffer.WriteString(convertStage(execution, j))
		pipelinebuffer.WriteString("\n")
		for k := range stage.Steps {
			stepName := fmt.Sprintf("step-%d-%d", j, k)
			jStep := toJenkinsStep(execution, j, k)
			containerDef := fmt.Sprintf(containerBlock, stepName, jStep.image, jStep.containerOptions)
			containerbuffer.WriteString(containerDef)

		}
	}
	ns := utils.GetPipelineCommonName(execution)
	jenkinsURL := fmt.Sprintf("http://%s:%d", utils.JenkinsName, utils.JenkinsPort)
	timeout := utils.DefaultTimeout
	if execution.Spec.PipelineConfig.Timeout > 0 {
		timeout = execution.Spec.PipelineConfig.Timeout
	}
	return fmt.Sprintf(pipelineBlock, ns, ns, containerbuffer.String(), images.Resolve(mv3.ToolsSystemImages.PipelineSystemImages.JenkinsJnlp), jenkinsURL, execution.Name, timeout, pipelinebuffer.String())
}

func getStepContainerOptions(execution *v3.PipelineExecution, privileged bool, optional map[string]string, envFrom []v3.EnvFrom) string {
	var buffer bytes.Buffer
	for k, v := range utils.GetEnvVarMap(execution) {
		buffer.WriteString(fmt.Sprintf(envVarSkel, k, v))
	}
	for k, v := range optional {
		buffer.WriteString(fmt.Sprintf(envVarSkel, k, v))
	}
	if execution.Spec.Event != utils.WebhookEventPullRequest {
		//expose no secrets on pull_request events
		for _, e := range envFrom {
			envName := e.SourceKey
			if e.TargetKey != "" {
				envName = e.TargetKey
			}
			buffer.WriteString(fmt.Sprintf(secretEnvSkel, envName, e.SourceName, e.SourceKey))
		}
	}
	result := fmt.Sprintf(envVarsSkel, buffer.String())
	if privileged {
		result = ", privileged: true" + result
	}
	return result
}

func parsePreservedEnvVar(execution *v3.PipelineExecution) {
	m := utils.GetEnvVarMap(execution)
	pipelineConfig := execution.Spec.PipelineConfig

	//environment variables substitution in configs
	for _, stage := range pipelineConfig.Stages {
		for _, step := range stage.Steps {
			if step.RunScriptConfig != nil {
				step.RunScriptConfig.Image = substituteEnvVar(m, step.RunScriptConfig.Image)
			} else if step.PublishImageConfig != nil {
				step.PublishImageConfig.Tag = substituteEnvVar(m, step.PublishImageConfig.Tag)
			} else if step.ApplyYamlConfig != nil {
				step.ApplyYamlConfig.Path = substituteEnvVar(m, step.ApplyYamlConfig.Path)
				step.ApplyYamlConfig.Content = substituteEnvVar(m, step.ApplyYamlConfig.Content)
			}
			for k, v := range step.Env {
				step.Env[k] = substituteEnvVar(m, v)
			}
		}
	}
}

func substituteEnvVar(envvar map[string]string, raw string) string {
	result := raw
	for k, v := range envvar {
		result = strings.Replace(result, "${"+k+"}", v, -1)
	}
	return result
}

const stageBlock = `stage('%s'){
%s
parallel %s
}
`

const stepBlock = `'%s': {
  stage('%s'){
    container(name: '%s') {
      %s
    }
  }
}`

const pipelineBlock = `import org.jenkinsci.plugins.pipeline.modeldefinition.Utils
def label = "buildpod.${env.JOB_NAME}.${env.BUILD_NUMBER}".replace('-', '_').replace('/', '_')
podTemplate(label: label, namespace: '%s', instanceCap: 1, serviceAccount: 'jenkins',volumes: [emptyDirVolume(mountPath: '/var/lib/docker', memory: false), secretVolume(mountPath: '/etc/docker/certs.d/docker-registry.%s', secretName: 'registry-crt')], containers: [
%s
containerTemplate(name: 'jnlp', image: '%s', envVars: [
envVar(key: 'JENKINS_URL', value: '%s')], args: '${computer.jnlpmac} ${computer.name}', ttyEnabled: false)], yaml: """
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: jenkins
    execution: %s
spec:
  affinity:
    podAntiAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchExpressions:
            - key: app
              operator: In
              values:
              - jenkins
          topologyKey: kubernetes.io/hostname
"""
) {
node(label) {
timestamps {
timeout(%d) {
%s
}
}
}
}`

const containerBlock = `containerTemplate(name: '%s', image: '%s', ttyEnabled: true, command: 'cat' %s),`

const envVarSkel = "envVar(key: '%s', value: '%s'),"

const secretEnvSkel = "secretEnvVar(key: '%s', secretName: '%s', secretKey: '%s'),"

const envVarsSkel = `, envVars: [
    %s
]`
