package jenkins

import (
	"bytes"
	"fmt"
	"strings"

	v33 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	v32 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"

	"github.com/pkg/errors"
	apiv1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	images "github.com/rancher/rancher/pkg/image"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
)

type jenkinsPipelineConverter struct {
	execution *v3.PipelineExecution
	opts      *executeOptions
}

type executeOptions struct {
	gitCaCerts            string
	imagePullSecretNames  []string
	executorMemoryRequest string
	executorMemoryLimit   string
	executorCPURequest    string
	executorCPULimit      string
}

func initJenkinsPipelineConverter(execution *v3.PipelineExecution, pipelineSettingLister v3.PipelineSettingLister, secretLister apiv1.SecretLister) (*jenkinsPipelineConverter, error) {
	_, projectID := ref.Parse(execution.Spec.ProjectName)
	cacertSetting, err := pipelineSettingLister.Get(projectID, utils.SettingGitCaCerts)
	if err != nil {
		return nil, err
	}
	secretNames, err := getImagePullSecretNames(secretLister, execution)
	if err != nil {
		return nil, err
	}
	memoryRequestSetting, err := pipelineSettingLister.Get(projectID, utils.SettingExecutorMemoryRequest)
	if err != nil {
		return nil, err
	}
	if err := validateQuantity(memoryRequestSetting.Value); err != nil {
		return nil, errors.Wrap(err, "invalid executor memory request config")
	}
	memoryLimitSetting, err := pipelineSettingLister.Get(projectID, utils.SettingExecutorMemoryLimit)
	if err != nil {
		return nil, err
	}
	if err := validateQuantity(memoryLimitSetting.Value); err != nil {
		return nil, errors.Wrap(err, "invalid executor memory limit config")
	}
	cpuRequestSetting, err := pipelineSettingLister.Get(projectID, utils.SettingExecutorCPURequest)
	if err != nil {
		return nil, err
	}
	if err := validateQuantity(cpuRequestSetting.Value); err != nil {
		return nil, errors.Wrap(err, "invalid executor cpu request config")
	}
	cpuLimitSetting, err := pipelineSettingLister.Get(projectID, utils.SettingExecutorCPULimit)
	if err != nil {
		return nil, err
	}
	if err := validateQuantity(cpuLimitSetting.Value); err != nil {
		return nil, errors.Wrap(err, "invalid executor cpu limit config")
	}
	opts := &executeOptions{
		gitCaCerts:            cacertSetting.Value,
		imagePullSecretNames:  secretNames,
		executorMemoryRequest: getPipelineSettingValue(memoryRequestSetting),
		executorMemoryLimit:   getPipelineSettingValue(memoryLimitSetting),
		executorCPURequest:    getPipelineSettingValue(cpuRequestSetting),
		executorCPULimit:      getPipelineSettingValue(cpuLimitSetting),
	}
	return &jenkinsPipelineConverter{
		execution: execution.DeepCopy(),
		opts:      opts,
	}, nil
}

func validateQuantity(value string) error {
	if value == "" {
		return nil
	}
	_, err := resource.ParseQuantity(value)
	return err
}

func (c *jenkinsPipelineConverter) convertPipelineExecutionToJenkinsPipeline() (*PipelineJob, error) {
	if c.execution == nil {
		return nil, errors.New("nil pipeline execution")
	}

	if err := utils.ValidPipelineConfig(c.execution.Spec.PipelineConfig); err != nil {
		return nil, err
	}
	parsePreservedEnvVar(c.execution)
	script, err := c.convertPipelineExecutionToPipelineScript()
	if err != nil {
		return nil, err
	}

	pipelineJob := &PipelineJob{
		Plugin: WorkflowJobPlugin,
		Definition: Definition{
			Class:   FlowDefinitionClass,
			Plugin:  FlowDefinitionPlugin,
			Sandbox: true,
			Script:  script,
		},
	}
	return pipelineJob, nil
}

func (c *jenkinsPipelineConverter) convertStep(stageOrdinal int, stepOrdinal int) string {
	stepName := fmt.Sprintf("step-%d-%d", stageOrdinal, stepOrdinal)

	command := c.getJenkinsStepCommand(stageOrdinal, stepOrdinal)

	return fmt.Sprintf(stepBlock, stepName, stepName, stepName, command)
}

func (c *jenkinsPipelineConverter) convertStage(stageOrdinal int) string {
	var buffer bytes.Buffer
	pipelineConfig := c.execution.Spec.PipelineConfig
	stage := pipelineConfig.Stages[stageOrdinal]
	for i := range stage.Steps {
		buffer.WriteString(c.convertStep(stageOrdinal, i))
		if i != len(stage.Steps)-1 {
			buffer.WriteString(",")
		}
	}
	skipOption := ""
	if !utils.MatchAll(stage.When, c.execution) {
		skipOption = fmt.Sprintf(markSkipScript, stage.Name)
	}

	return fmt.Sprintf(stageBlock, stage.Name, skipOption, buffer.String())
}

func (c *jenkinsPipelineConverter) convertPipelineExecutionToPipelineScript() (string, error) {
	pod := c.getBasePodTemplate()
	var pipelinebuffer bytes.Buffer
	for j, stage := range c.execution.Spec.PipelineConfig.Stages {
		pipelinebuffer.WriteString(c.convertStage(j))
		pipelinebuffer.WriteString("\n")
		for k := range stage.Steps {
			container, err := c.getStepContainer(j, k)
			if err != nil {
				return "", err
			}
			pod.Spec.Containers = append(pod.Spec.Containers, container)
		}
	}
	agentContainer, err := c.getAgentContainer()
	if err != nil {
		return "", err
	}
	pod.Spec.Containers = append(pod.Spec.Containers, agentContainer)
	timeout := utils.DefaultTimeout
	if c.execution.Spec.PipelineConfig.Timeout > 0 {
		timeout = c.execution.Spec.PipelineConfig.Timeout
	}
	if c.opts.gitCaCerts != "" {
		c.injectGitCaCert(pod)
	}
	if len(c.opts.imagePullSecretNames) > 0 {
		c.configImagePullSecrets(pod)
	}
	b := &bytes.Buffer{}
	e := serializer.NewYAMLSerializer(serializer.DefaultMetaFactory, nil, nil)
	if err := e.Encode(pod, b); err != nil {
		return "", err
	}

	return fmt.Sprintf(pipelineBlock, b.String(), timeout, pipelinebuffer.String()), nil
}

func (c *jenkinsPipelineConverter) getBasePodTemplate() *v1.Pod {
	ns := utils.GetPipelineCommonName(c.execution.Spec.ProjectName)
	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				utils.LabelKeyApp:       utils.JenkinsName,
				utils.LabelKeyExecution: c.execution.Name,
			},
			Namespace: ns,
		},
		Spec: v1.PodSpec{
			ServiceAccountName: utils.JenkinsName,
			Affinity: &v1.Affinity{
				PodAntiAffinity: &v1.PodAntiAffinity{
					PreferredDuringSchedulingIgnoredDuringExecution: []v1.WeightedPodAffinityTerm{
						{
							Weight: 100,
							PodAffinityTerm: v1.PodAffinityTerm{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      utils.LabelKeyApp,
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{utils.JenkinsName},
										},
									},
								},
								TopologyKey: "kubernetes.io/hostname",
							},
						},
					},
				},
			},
			Volumes: []v1.Volume{
				{
					Name: utils.RegistryCrtVolumeName,
					VolumeSource: v1.VolumeSource{
						Secret: &v1.SecretVolumeSource{
							SecretName: utils.RegistryCrtSecretName,
						},
					},
				},
			},
		},
	}
	return pod
}

func (c *jenkinsPipelineConverter) configImagePullSecrets(pod *v1.Pod) {
	var refs []v1.LocalObjectReference
	for _, secretName := range c.opts.imagePullSecretNames {
		refs = append(refs, v1.LocalObjectReference{
			Name: secretName,
		})
	}
	pod.Spec.ImagePullSecrets = refs
}

func (c *jenkinsPipelineConverter) injectGitCaCert(pod *v1.Pod) {
	pod.Spec.InitContainers = []v1.Container{
		{
			Name:    "config-crt",
			Image:   images.Resolve(v33.ToolsSystemImages.PipelineSystemImages.AlpineGit),
			Command: []string{"sh", "-c", "CERT_PATH=/home/jenkins/certs/ca.crt;printf \"%s\" \"$CA_CERT\" > $CERT_PATH;chown 10000:10000 $CERT_PATH;"},
			Env: []v1.EnvVar{
				{
					Name:  "CA_CERT",
					Value: c.opts.gitCaCerts,
				},
			},
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      utils.GitCaCertVolumeName,
					MountPath: utils.GitCaCertPath,
				},
			},
		},
	}
	for i, container := range pod.Spec.Containers {
		if container.Name == utils.JenkinsAgentContainerName {
			c.injectGitCaCertToContainer(&pod.Spec.Containers[i])
			break
		}
	}
	pod.Spec.Volumes = append(pod.Spec.Volumes, v1.Volume{
		Name: utils.GitCaCertVolumeName,
		VolumeSource: v1.VolumeSource{
			EmptyDir: &v1.EmptyDirVolumeSource{},
		},
	})
}

func (c *jenkinsPipelineConverter) injectGitCaCertToContainer(container *v1.Container) {
	container.Env = append(container.Env, v1.EnvVar{
		Name:  "GIT_SSL_CAINFO",
		Value: utils.GitCaCertPath + "/ca.crt",
	})
	container.VolumeMounts = append(container.VolumeMounts, v1.VolumeMount{
		Name:      utils.GitCaCertVolumeName,
		MountPath: utils.GitCaCertPath,
	})
}

func (c *jenkinsPipelineConverter) injectAgentResources(container *v1.Container) error {
	return injectResources(container, c.opts.executorCPULimit, c.opts.executorCPURequest, c.opts.executorMemoryLimit, c.opts.executorMemoryRequest)
}

func injectSetpContainerResources(container *v1.Container, step *v32.Step) error {
	return injectResources(container, step.CPULimit, step.CPURequest, step.MemoryLimit, step.MemoryRequest)
}

func injectResources(container *v1.Container, cpuLimit string, cpuRequest string, memoryLimit string, memoryRequest string) error {
	if cpuLimit != "" {
		if container.Resources.Limits == nil {
			container.Resources.Limits = v1.ResourceList{}
		}
		quantity, err := resource.ParseQuantity(cpuLimit)
		if err != nil {
			return errors.Wrapf(err, "invalid CPU limit %q", cpuLimit)
		}

		container.Resources.Limits[v1.ResourceCPU] = quantity
	}
	if cpuRequest != "" {
		if container.Resources.Requests == nil {
			container.Resources.Requests = v1.ResourceList{}
		}
		quantity, err := resource.ParseQuantity(cpuRequest)
		if err != nil {
			return errors.Wrapf(err, "invalid CPU request %q", cpuRequest)
		}

		container.Resources.Requests[v1.ResourceCPU] = quantity
	}
	if memoryLimit != "" {
		if container.Resources.Limits == nil {
			container.Resources.Limits = v1.ResourceList{}
		}
		quantity, err := resource.ParseQuantity(memoryLimit)
		if err != nil {
			return errors.Wrapf(err, "invalid memory limit %q", memoryLimit)
		}

		container.Resources.Limits[v1.ResourceMemory] = quantity
	}
	if memoryRequest != "" {
		if container.Resources.Requests == nil {
			container.Resources.Requests = v1.ResourceList{}
		}
		quantity, err := resource.ParseQuantity(memoryRequest)
		if err != nil {
			return errors.Wrapf(err, "invalid memory request %q", memoryRequest)
		}

		container.Resources.Requests[v1.ResourceMemory] = quantity
	}
	return nil
}

func getImagePullSecretNames(secretLister apiv1.SecretLister, execution *v3.PipelineExecution) ([]string, error) {
	result := []string{}
	ns := utils.GetPipelineCommonName(execution.Spec.ProjectName)
	secrets, err := secretLister.List(ns, labels.Everything())
	if err != nil {
		return nil, err
	}
	for _, s := range secrets {
		if s.Type == v1.SecretTypeDockerConfigJson {
			result = append(result, s.Name)
		}
	}
	logrus.Debugf("using imagepullsecrets %v for the build", result)
	return result, nil
}

func getPipelineSettingValue(setting *v3.PipelineSetting) string {
	if setting.Value != "" {
		return setting.Value
	}
	return setting.Default
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
			} else if step.PublishCatalogConfig != nil {
				step.PublishCatalogConfig.Path = substituteEnvVar(m, step.PublishCatalogConfig.Path)
				step.PublishCatalogConfig.CatalogTemplate = substituteEnvVar(m, step.PublishCatalogConfig.CatalogTemplate)
				step.PublishCatalogConfig.Version = substituteEnvVar(m, step.PublishCatalogConfig.Version)
			} else if step.ApplyAppConfig != nil {
				step.ApplyAppConfig.CatalogTemplate = substituteEnvVar(m, step.ApplyAppConfig.CatalogTemplate)
				step.ApplyAppConfig.Version = substituteEnvVar(m, step.ApplyAppConfig.Version)
				step.ApplyAppConfig.Name = substituteEnvVar(m, step.ApplyAppConfig.Name)
				step.ApplyAppConfig.TargetNamespace = substituteEnvVar(m, step.ApplyAppConfig.TargetNamespace)
				for k, v := range step.ApplyAppConfig.Answers {
					step.ApplyAppConfig.Answers[k] = substituteEnvVar(m, v)
				}
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
podTemplate(label: label, instanceCap: 1, yaml: '''
%s
'''
) {
node(label) {
timestamps {
timeout(%d) {
%s
}
}
}
}`
