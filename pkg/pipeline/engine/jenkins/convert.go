package jenkins

import (
	"fmt"
	"regexp"
	"strings"

	images "github.com/rancher/rancher/pkg/image"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/settings"
	mv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	v3 "github.com/rancher/types/apis/project.cattle.io/v3"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
)

func (c *jenkinsPipelineConverter) getStepContainer(stageOrdinal int, stepOrdinal int) (v1.Container, error) {
	stage := c.execution.Spec.PipelineConfig.Stages[stageOrdinal]
	step := &stage.Steps[stepOrdinal]

	container := v1.Container{
		Name:    fmt.Sprintf("step-%d-%d", stageOrdinal, stepOrdinal),
		TTY:     true,
		Command: []string{"cat"},
		Env:     []v1.EnvVar{},
	}
	if step.SourceCodeConfig != nil {
		if err := c.configCloneStepContainer(&container, step); err != nil {
			return container, err
		}
	} else if step.RunScriptConfig != nil {
		c.configRunScriptStepContainer(&container, step)
	} else if step.PublishImageConfig != nil {
		c.configPublishStepContainer(&container, step)
	} else if step.ApplyYamlConfig != nil {
		if err := c.configApplyYamlStepContainer(&container, step, stageOrdinal); err != nil {
			return container, err
		}
	} else if step.PublishCatalogConfig != nil {
		if err := c.configPublishCatalogContainer(&container, step); err != nil {
			return container, err
		}
	} else if step.ApplyAppConfig != nil {
		if err := c.configApplyAppContainer(&container, step); err != nil {
			return container, err
		}
	}

	//common step configurations
	for k, v := range utils.GetEnvVarMap(c.execution) {
		container.Env = append(container.Env, v1.EnvVar{Name: k, Value: v})
	}
	for k, v := range step.Env {
		container.Env = append(container.Env, v1.EnvVar{Name: k, Value: v})
	}
	if c.execution.Spec.Event != utils.WebhookEventPullRequest {
		//expose no secrets on pull_request events
		for _, e := range step.EnvFrom {
			envName := e.SourceKey
			if e.TargetKey != "" {
				envName = e.TargetKey
			}
			container.Env = append(container.Env, v1.EnvVar{
				Name: envName,
				ValueFrom: &v1.EnvVarSource{SecretKeyRef: &v1.SecretKeySelector{
					LocalObjectReference: v1.LocalObjectReference{
						Name: e.SourceName,
					},
					Key: e.SourceKey,
				}}})
		}
	}
	if step.Privileged {
		container.SecurityContext = &v1.SecurityContext{Privileged: &step.Privileged}
	}
	err := injectSetpContainerResources(&container, step)
	return container, err
}

func (c *jenkinsPipelineConverter) getJenkinsStepCommand(stageOrdinal int, stepOrdinal int) string {
	stage := c.execution.Spec.PipelineConfig.Stages[stageOrdinal]
	step := &stage.Steps[stepOrdinal]
	command := ""

	if !utils.MatchAll(stage.When, c.execution) || !utils.MatchAll(step.When, c.execution) {
		stepName := fmt.Sprintf("step-%d-%d", stageOrdinal, stepOrdinal)
		command = fmt.Sprintf(markSkipScript, stepName)
	} else if step.SourceCodeConfig != nil {
		command = fmt.Sprintf("checkout([$class: 'GitSCM', branches: [[name: 'local/temp']], userRemoteConfigs: [[url: '%s', refspec: '+%s:refs/remotes/local/temp', credentialsId: '%s']]])",
			c.execution.Spec.RepositoryURL, c.execution.Spec.Ref, c.execution.Name)
	} else if step.RunScriptConfig != nil {
		command = fmt.Sprintf(`sh ''' %s '''`, step.RunScriptConfig.ShellScript)
	} else if step.PublishImageConfig != nil {
		command = `sh '''/usr/local/bin/dockerd-entrypoint.sh /bin/drone-docker'''`
	} else if step.ApplyYamlConfig != nil {
		command = `sh ''' kube-apply '''`
	} else if step.PublishCatalogConfig != nil {
		command = `sh ''' publish-catalog '''`
	} else if step.ApplyAppConfig != nil {
		command = `sh ''' apply-app '''`
	}
	return command
}

func (c *jenkinsPipelineConverter) getAgentContainer() (v1.Container, error) {
	container := v1.Container{
		Name:  utils.JenkinsAgentContainerName,
		Image: images.Resolve(mv3.ToolsSystemImages.PipelineSystemImages.JenkinsJnlp),
		Args:  []string{"$(JENKINS_SECRET)", "$(JENKINS_NAME)"},
	}
	cloneContainer, err := c.getStepContainer(0, 0)
	if err != nil {
		return container, err
	}
	container.Env = append(container.Env, cloneContainer.Env...)
	container.EnvFrom = append(container.EnvFrom, cloneContainer.EnvFrom...)
	err = c.injectAgentResources(&container)
	return container, err
}

func (c *jenkinsPipelineConverter) configCloneStepContainer(container *v1.Container, step *v3.Step) error {
	container.Image = images.Resolve(mv3.ToolsSystemImages.PipelineSystemImages.AlpineGit)
	return injectResources(container, utils.PipelineToolsCPULimitDefault, utils.PipelineToolsCPURequestDefault, utils.PipelineToolsMemoryLimitDefault, utils.PipelineToolsMemoryRequestDefault)
}

func (c *jenkinsPipelineConverter) configRunScriptStepContainer(container *v1.Container, step *v3.Step) {
	container.Image = step.RunScriptConfig.Image
}

func (c *jenkinsPipelineConverter) configPublishStepContainer(container *v1.Container, step *v3.Step) {
	ns := utils.GetPipelineCommonName(c.execution.Spec.ProjectName)
	config := step.PublishImageConfig
	m := utils.GetEnvVarMap(c.execution)
	config.Tag = substituteEnvVar(m, config.Tag)

	registry, repo, tag := utils.SplitImageTag(config.Tag)

	if config.PushRemote {
		registry = config.Registry
	} else {
		_, projectID := ref.Parse(c.execution.Spec.ProjectName)
		registry = fmt.Sprintf("%s.%s-pipeline", utils.LocalRegistry, projectID)
	}

	reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
	processedRegistry := strings.ToLower(reg.ReplaceAllString(registry, ""))
	secretName := fmt.Sprintf("%s-%s", c.execution.Namespace, processedRegistry)
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

	container.Image = images.Resolve(mv3.ToolsSystemImages.PipelineSystemImages.PluginsDocker)
	publishEnv := map[string]string{
		"DOCKER_REGISTRY":   registry,
		"PLUGIN_REPO":       pluginRepo,
		"PLUGIN_TAG":        tag,
		"PLUGIN_DOCKERFILE": config.DockerfilePath,
		"PLUGIN_CONTEXT":    config.BuildContext,
	}
	for k, v := range publishEnv {
		container.Env = append(container.Env, v1.EnvVar{Name: k, Value: v})
	}
	container.Env = append(container.Env, v1.EnvVar{
		Name: "DOCKER_USERNAME",
		ValueFrom: &v1.EnvVarSource{SecretKeyRef: &v1.SecretKeySelector{
			LocalObjectReference: v1.LocalObjectReference{
				Name: secretName,
			},
			Key: secretUserKey,
		}}})
	container.Env = append(container.Env, v1.EnvVar{
		Name: "DOCKER_PASSWORD",
		ValueFrom: &v1.EnvVarSource{SecretKeyRef: &v1.SecretKeySelector{
			LocalObjectReference: v1.LocalObjectReference{
				Name: secretName,
			},
			Key: secretPwKey,
		}}})
	privileged := true
	container.SecurityContext = &v1.SecurityContext{Privileged: &privileged}
	container.VolumeMounts = []v1.VolumeMount{
		{
			Name:      utils.RegistryCrtVolumeName,
			MountPath: fmt.Sprintf("/etc/docker/certs.d/docker-registry.%s", ns),
			ReadOnly:  true,
		},
	}
}

func (c *jenkinsPipelineConverter) configApplyYamlStepContainer(container *v1.Container, step *v3.Step, stageOrdinal int) error {
	config := step.ApplyYamlConfig
	container.Image = images.Resolve(mv3.ToolsSystemImages.PipelineSystemImages.KubeApply)

	applyEnv := map[string]string{
		"YAML_PATH":    config.Path,
		"YAML_CONTENT": config.Content,
		"NAMESPACE":    config.Namespace,
	}

	//for deploy step, get registry & image variable from a previous publish step
	var registry, imageRepo string
StageLoop:
	for i := stageOrdinal; i >= 0; i-- {
		stage := c.execution.Spec.PipelineConfig.Stages[i]
		for j := len(stage.Steps) - 1; j >= 0; j-- {
			step := stage.Steps[j]
			if step.PublishImageConfig != nil && utils.MatchAll(stage.When, c.execution) && utils.MatchAll(step.When, c.execution) {
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

	for k, v := range applyEnv {
		container.Env = append(container.Env, v1.EnvVar{Name: k, Value: v})
	}
	return injectResources(container, utils.PipelineToolsCPULimitDefault, utils.PipelineToolsCPURequestDefault, utils.PipelineToolsMemoryLimitDefault, utils.PipelineToolsMemoryRequestDefault)
}

func (c *jenkinsPipelineConverter) configPublishCatalogContainer(container *v1.Container, step *v3.Step) error {
	if c.opts.gitCaCerts != "" {
		c.injectGitCaCertToContainer(container)
	}
	config := step.PublishCatalogConfig
	container.Image = images.Resolve(mv3.ToolsSystemImages.PipelineSystemImages.KubeApply)
	envs := map[string]string{
		"CATALOG_PATH":          config.Path,
		"CATALOG_TEMPLATE_NAME": config.CatalogTemplate,
		"VERSION":               config.Version,
		"GIT_AUTHOR":            config.GitAuthor,
		"GIT_EMAIL":             config.GitEmail,
		"GIT_URL":               config.GitURL,
		"GIT_BRANCH":            config.GitBranch,
	}
	for k, v := range envs {
		container.Env = append(container.Env, v1.EnvVar{Name: k, Value: v})
	}
	var customEnvs []string
	for k := range step.Env {
		customEnvs = append(customEnvs, k)
	}
	container.Env = append(container.Env, v1.EnvVar{Name: "CICD_SUBSTITUTE_VARS", Value: strings.Join(customEnvs, ",")})
	return injectResources(container, utils.PipelineToolsCPULimitDefault, utils.PipelineToolsCPURequestDefault, utils.PipelineToolsMemoryLimitDefault, utils.PipelineToolsMemoryRequestDefault)
}

func (c *jenkinsPipelineConverter) configApplyAppContainer(container *v1.Container, step *v3.Step) error {
	config := step.ApplyAppConfig
	container.Image = images.Resolve(mv3.ToolsSystemImages.PipelineSystemImages.KubeApply)
	answerBytes, _ := yaml.Marshal(config.Answers)
	envs := map[string]string{
		"APP_NAME":              config.Name,
		"ANSWERS":               string(answerBytes),
		"CATALOG_TEMPLATE_NAME": config.CatalogTemplate,
		"VERSION":               config.Version,
		"TARGET_NAMESPACE":      config.TargetNamespace,
		"RANCHER_URL":           settings.ServerURL.Get(),
	}
	for k, v := range envs {
		container.Env = append(container.Env, v1.EnvVar{Name: k, Value: v})
	}
	container.Env = append(container.Env, v1.EnvVar{
		Name: utils.PipelineSecretAPITokenKey,
		ValueFrom: &v1.EnvVarSource{SecretKeyRef: &v1.SecretKeySelector{
			LocalObjectReference: v1.LocalObjectReference{
				Name: utils.PipelineAPIKeySecretName,
			},
			Key: utils.PipelineSecretAPITokenKey,
		}}})
	return injectResources(container, utils.PipelineToolsCPULimitDefault, utils.PipelineToolsCPURequestDefault, utils.PipelineToolsMemoryLimitDefault, utils.PipelineToolsMemoryRequestDefault)
}
