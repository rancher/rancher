package jenkins

import (
	"bytes"
	"fmt"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/utils"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"regexp"
	"strconv"
	"strings"
)

func ConvertPipelineToJenkinsPipeline(pipeline *v3.Pipeline) PipelineJob {
	parsedPipeline := parsePreservedEnvVar(*pipeline)
	pipelineJob := PipelineJob{
		Plugin: WorkflowJobPlugin,
		Definition: Definition{
			Class:   FlowDefinitionClass,
			Plugin:  FlowDefinitionPlugin,
			Sandbox: true,
			Script:  convertPipeline(parsedPipeline),
		},
	}

	return pipelineJob
}

func convertStep(pipeline *v3.Pipeline, stageOrdinal int, stepOrdinal int) string {

	stepContent := ""
	stepName := fmt.Sprintf("step-%d-%d", stageOrdinal, stepOrdinal)
	step := pipeline.Spec.Stages[stageOrdinal].Steps[stepOrdinal]

	if step.SourceCodeConfig != nil {
		branch := step.SourceCodeConfig.Branch
		branchCondition := step.SourceCodeConfig.BranchCondition
		//default only branch xxx
		if branchCondition == "except" {
			branch = fmt.Sprintf(":^(?!(%s))", branch)
		} else if branchCondition == "all" {
			branch = "**"
		}
		stepContent = fmt.Sprintf("git url: '%s', branch: '%s', credentialsId: '%s'", step.SourceCodeConfig.URL, branch, step.SourceCodeConfig.SourceCodeCredentialName)
	} else if step.RunScriptConfig != nil {
		if step.RunScriptConfig.IsShell {
			stepContent = fmt.Sprintf(`sh ''' %s '''`, step.RunScriptConfig.ShellScript)
		} else {
			script := step.RunScriptConfig.Entrypoint
			if step.RunScriptConfig.Command != "" {
				script = script + " " + step.RunScriptConfig.Command
			}
			stepContent = fmt.Sprintf(`sh ''' %s '''`, script)
		}
	} else if step.PublishImageConfig != nil {
		stepContent = fmt.Sprintf(`sh """/usr/local/bin/dockerd-entrypoint.sh /bin/drone-docker"""`)
	} else {
		return ""
	}
	return fmt.Sprintf(stepBlock, stepName, stepName, stepName, stepContent)
}

func convertStage(pipeline *v3.Pipeline, stageOrdinal int) string {
	var buffer bytes.Buffer
	stage := pipeline.Spec.Stages[stageOrdinal]
	for i := range stage.Steps {
		buffer.WriteString(convertStep(pipeline, stageOrdinal, i))
		if i != len(stage.Steps)-1 {
			buffer.WriteString(",")
		}
	}

	return fmt.Sprintf(stageBlock, stage.Name, buffer.String())
}

func convertPipeline(pipeline *v3.Pipeline) string {
	var containerbuffer bytes.Buffer
	var pipelinebuffer bytes.Buffer
	for j, stage := range pipeline.Spec.Stages {
		pipelinebuffer.WriteString(convertStage(pipeline, j))
		pipelinebuffer.WriteString("\n")
		for k, step := range stage.Steps {
			stepName := fmt.Sprintf("step-%d-%d", j, k)
			image := ""
			options := ""
			if step.SourceCodeConfig != nil {
				image = "alpine/git"
			} else if step.RunScriptConfig != nil {
				image = step.RunScriptConfig.Image

				options = getPreservedEnvVarOptions(pipeline)
			} else if step.PublishImageConfig != nil {
				registry, repo, tag := utils.SplitImageTag(step.PublishImageConfig.Tag)
				//TODO key-key mapping instead of registry-key mapping
				reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
				proceccedRegistry := strings.ToLower(reg.ReplaceAllString(registry, ""))
				secretName := fmt.Sprintf("%s-%s", pipeline.Namespace, proceccedRegistry)
				pluginRepo := fmt.Sprintf("%s/%s", registry, repo)
				if registry == utils.DefaultRegistry {
					//the `plugins/docker` image fails when setting DOCKER_REGISTRY to index.docker.io
					registry = ""
				}
				image = "plugins/docker"
				publishoption := `, privileged: true, envVars: [
			envVar(key: 'PLUGIN_REPO', value: '%s'),
			envVar(key: 'PLUGIN_TAG', value: '%s'),
			envVar(key: 'PLUGIN_DOCKERFILE', value: '%s'),
			envVar(key: 'PLUGIN_CONTEXT', value: '%s'),
			envVar(key: 'DOCKER_REGISTRY', value: '%s'),
            secretEnvVar(key: 'DOCKER_USERNAME', secretName: '%s', secretKey: 'username'),
            secretEnvVar(key: 'DOCKER_PASSWORD', secretName: '%s', secretKey: 'password'),
        ]`
				options = fmt.Sprintf(publishoption, pluginRepo, tag, step.PublishImageConfig.DockerfilePath, step.PublishImageConfig.BuildContext, registry, secretName, secretName)
			} else {
				return ""
			}
			containerDef := fmt.Sprintf(containerBlock, stepName, image, options)
			containerbuffer.WriteString(containerDef)

		}
	}

	return fmt.Sprintf(pipelineBlock, containerbuffer.String(), pipelinebuffer.String())
}

func getPreservedEnvVarOptions(pipeline *v3.Pipeline) string {
	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf(envVarSkel, "CICD_PIPELINE_NAME", pipeline.Spec.DisplayName))
	buffer.WriteString(fmt.Sprintf(envVarSkel, "CICD_RUN_NUMBER", strconv.Itoa(pipeline.Status.NextRun)))

	return fmt.Sprintf(envVarsSkel, buffer.String())
}

func parsePreservedEnvVar(pipeline v3.Pipeline) *v3.Pipeline {
	m := map[string]string{}
	m["CICD_PIPELINE_NAME"] = pipeline.Spec.DisplayName
	m["CICD_RUN_NUMBER"] = strconv.Itoa(pipeline.Status.NextRun)

	//environment variables substituion in configs
	for _, stage := range pipeline.Spec.Stages {
		for _, step := range stage.Steps {
			if step.RunScriptConfig != nil {
				for k, v := range m {
					step.RunScriptConfig.Image = strings.Replace(step.RunScriptConfig.Image, "${"+k+"}", v, -1)
				}
			} else if step.PublishImageConfig != nil {
				for k, v := range m {
					step.PublishImageConfig.Tag = strings.Replace(step.PublishImageConfig.Tag, "${"+k+"}", v, -1)
				}
			}
		}
	}
	return &pipeline
}

const stageBlock = `stage('%s'){
parallel %s
}
`

const stepBlock = `'%s': {
  stage('%s'){
    container(name: '%s') {
      %s
    }
  }
}
`

const pipelineBlock = `def label = "buildpod.${env.JOB_NAME}.${env.BUILD_NUMBER}".replace('-', '_').replace('/', '_')
podTemplate(label: label, containers: [
%s
containerTemplate(name: 'jnlp', image: 'jenkins/jnlp-slave:3.10-1-alpine', envVars: [
envVar(key: 'JENKINS_URL', value: 'http://jenkins:8080')], args: '${computer.jnlpmac} ${computer.name}', ttyEnabled: false)]) {
node(label) {
timestamps {
%s
}
}
}`

const containerBlock = `containerTemplate(name: '%s', image: '%s', ttyEnabled: true, command: 'cat' %s),`

const envVarSkel = "envVar(key: '%s', value: '%s'),"

const envVarsSkel = `, envVars: [
    %s
]`
