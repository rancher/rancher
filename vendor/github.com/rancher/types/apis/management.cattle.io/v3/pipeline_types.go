package v3

import (
	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/types"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PipelineConditionType string

const (
	PipelineExecutionConditonProvisioned condition.Cond = "Provisioned"
	PipelineExecutionConditionCompleted  condition.Cond = "Completed"
)

type ClusterPipeline struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterPipelineSpec   `json:"spec"`
	Status ClusterPipelineStatus `json:"status"`
}

type Pipeline struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	ProjectName string `json:"projectName" yaml:"projectName" norman:"required,type=reference[project]"`

	Spec   PipelineSpec   `json:"spec"`
	Status PipelineStatus `json:"status"`
}

type PipelineExecution struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	ProjectName string `json:"projectName" yaml:"projectName" norman:"required,type=reference[project]"`

	Spec   PipelineExecutionSpec   `json:"spec"`
	Status PipelineExecutionStatus `json:"status"`
}

type PipelineExecutionLog struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	ProjectName string `json:"projectName" yaml:"projectName" norman:"required,type=reference[project]"`

	Spec PipelineExecutionLogSpec `json:"spec"`
}

type SourceCodeCredential struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SourceCodeCredentialSpec   `json:"spec"`
	Status SourceCodeCredentialStatus `json:"status"`
}

type SourceCodeRepository struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SourceCodeRepositorySpec   `json:"spec"`
	Status SourceCodeRepositoryStatus `json:"status"`
}

type ClusterPipelineSpec struct {
	ClusterName  string        `json:"clusterName" norman:"type=reference[cluster]"`
	Deploy       bool          `json:"deploy"`
	GithubConfig *GitAppConfig `json:"githubConfig,omitempty"`
	GitlabConfig *GitAppConfig `json:"gitlabConfig,omitempty"`
}

type ClusterPipelineStatus struct {
}

type GitAppConfig struct {
	TLS          bool   `json:"tls,omitempty"`
	Host         string `json:"host,omitempty"`
	ClientID     string `json:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
	RedirectURL  string `json:"redirectUrl,omitempty"`
}

type PipelineStatus struct {
	PipelineState        string                `json:"pipelineState,omitempty" norman:"required,options=active|inactive,default=active"`
	NextRun              int                   `json:"nextRun" yaml:"nextRun,omitempty" norman:"default=1,min=1"`
	LastExecutionID      string                `json:"lastExecutionId,omitempty" yaml:"lastExecutionId,omitempty"`
	LastRunState         string                `json:"lastRunState,omitempty" yaml:"lastRunState,omitempty"`
	LastStarted          string                `json:"lastStarted,omitempty" yaml:"lastStarted,omitempty"`
	NextStart            string                `json:"nextStart,omitempty" yaml:"nextStart,omitempty"`
	WebHookID            string                `json:"webhookId,omitempty" yaml:"webhookId,omitempty"`
	Token                string                `json:"token,omitempty" yaml:"token,omitempty" norman:"writeOnly,noupdate"`
	SourceCodeCredential *SourceCodeCredential `json:"sourceCodeCredential,omitempty" yaml:"sourceCodeCredential,omitempty"`
}

type PipelineSpec struct {
	DisplayName           string `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	TriggerWebhookPush    bool   `json:"triggerWebhookPush,omitempty" yaml:"triggerWebhookPush,omitempty"`
	TriggerWebhookPr      bool   `json:"triggerWebhookPr,omitempty" yaml:"triggerWebhookPr,omitempty"`
	TriggerWebhookTag     bool   `json:"triggerWebhookTag,omitempty" yaml:"triggerWebhookTag,omitempty"`
	TriggerCronTimezone   string `json:"triggerCronTimezone,omitempty" yaml:"triggerCronTimezone,omitempty"`
	TriggerCronExpression string `json:"triggerCronExpression,omitempty" yaml:"triggerCronExpression,omitempty"`

	Stages []Stage `json:"stages,omitempty" yaml:"stages,omitempty"`

	Templates map[string]string `json:"templates,omitempty" yaml:"templates,omitempty"`
}

type PipelineCondition struct {
	// Type of cluster condition.
	Type PipelineConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status v1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Human-readable message indicating details about last transition
	Message string `json:"message,omitempty"`
}

type Stage struct {
	Name  string `json:"name,omitempty" yaml:"name,omitempty" norman:"required"`
	Steps []Step `json:"steps,omitempty" yaml:"steps,omitempty" norman:"required"`
}

type Step struct {
	SourceCodeConfig   *SourceCodeConfig   `json:"sourceCodeConfig,omitempty" yaml:"sourceCodeConfig,omitempty"`
	RunScriptConfig    *RunScriptConfig    `json:"runScriptConfig,omitempty" yaml:"runScriptConfig,omitempty"`
	PublishImageConfig *PublishImageConfig `json:"publishImageConfig,omitempty" yaml:"publishImageConfig,omitempty"`
	//Step timeout in minutes
	Timeout int `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

type SourceCodeConfig struct {
	URL                      string `json:"url,omitempty" yaml:"url,omitempty" norman:"required"`
	Branch                   string `json:"branch,omitempty" yaml:"branch,omitempty" `
	BranchCondition          string `json:"branchCondition,omitempty" yaml:"branchCondition,omitempty" norman:"options=only|except|all"`
	SourceCodeCredentialName string `json:"sourceCodeCredentialName,omitempty" yaml:"sourceCodeCredentialName,omitempty" norman:"type=reference[sourceCodeCredential]"`
}

type RunScriptConfig struct {
	Image       string   `json:"image,omitempty" yaml:"image,omitempty" norman:"required"`
	IsShell     bool     `json:"isShell,omitempty" yaml:"isShell,omitempty"`
	ShellScript string   `json:"shellScript,omitempty" yaml:"shellScript,omitempty"`
	Entrypoint  string   `json:"entrypoint,omitempty" yaml:"enrtypoint,omitempty"`
	Command     string   `json:"command,omitempty" yaml:"command,omitempty"`
	Env         []string `json:"env,omitempty" yaml:"env,omitempty"`
}

type PublishImageConfig struct {
	DockerfilePath string `json:"dockerfilePath,omittempty" yaml:"dockerfilePath,omitempty" norman:"required,default=./Dockerfile"`
	BuildContext   string `json:"buildContext,omitempty" yaml:"buildContext,omitempty" norman:"required,default=."`
	Tag            string `json:"tag,omitempty" yaml:"tag,omitempty" norman:"required,default=${CICD_GIT_REPOSITORY_NAME}:${CICD_GIT_BRANCH}"`
}

type PipelineExecutionSpec struct {
	PipelineName    string   `json:"pipelineName" norman:"required,type=reference[pipeline]"`
	Run             int      `json:"run,omitempty" norman:"required,min=1"`
	TriggeredBy     string   `json:"triggeredBy,omitempty" norman:"required,options=user|cron|webhook"`
	TriggerUserName string   `json:"triggerUserName,omitempty" norman:"type=reference[user]"`
	Pipeline        Pipeline `json:"pipeline,omitempty" norman:"required"`
}

type PipelineExecutionStatus struct {
	Conditions []PipelineCondition `json:"conditions,omitempty"`

	Commit         string            `json:"commit,omitempty"`
	ExecutionState string            `json:"executionState,omitempty"`
	Started        string            `json:"started,omitempty"`
	Ended          string            `json:"ended,omitempty"`
	Stages         []StageStatus     `json:"stages,omitempty"`
	EnvVars        map[string]string `json:"envVars,omitempty"`
}

type StageStatus struct {
	State   string       `json:"state,omitempty"`
	Started string       `json:"started,omitempty"`
	Ended   string       `json:"ended,omitempty"`
	Steps   []StepStatus `json:"steps,omitempty"`
}

type StepStatus struct {
	State   string `json:"state,omitempty"`
	Started string `json:"started,omitempty"`
	Ended   string `json:"ended,omitempty"`
}

type SourceCodeCredentialSpec struct {
	ClusterName    string `json:"clusterName" norman:"required,type=reference[cluster]"`
	SourceCodeType string `json:"sourceCodeType,omitempty" norman:"required,options=github"`
	UserName       string `json:"userName" norman:"required,type=reference[user]"`
	DisplayName    string `json:"displayName,omitempty" norman:"required"`
	AvatarURL      string `json:"avatarUrl,omitempty"`
	HTMLURL        string `json:"htmlUrl,omitempty"`
	LoginName      string `json:"loginName,omitempty"`
	AccessToken    string `json:"accessToken,omitempty" norman:"writeOnly,noupdate"`
}

type SourceCodeCredentialStatus struct {
}

type SourceCodeRepositorySpec struct {
	ClusterName              string   `json:"clusterName" norman:"required,type=reference[cluster]"`
	SourceCodeType           string   `json:"sourceCodeType,omitempty" norman:"required,options=github"`
	UserName                 string   `json:"userName" norman:"required,type=reference[user]"`
	SourceCodeCredentialName string   `json:"sourceCodeCredentialName,omitempty" norman:"required,type=reference[sourceCodeCredential]"`
	URL                      string   `json:"url,omitempty"`
	Permissions              RepoPerm `json:"permissions,omitempty"`
	Language                 string   `json:"language,omitempty"`
	DefaultBranch            string   `json:"defaultBranch,omitempty"`
}

type SourceCodeRepositoryStatus struct {
}

type RepoPerm struct {
	Pull  bool `json:"pull,omitempty"`
	Push  bool `json:"push,omitempty"`
	Admin bool `json:"admin,omitempty"`
}

type PipelineExecutionLogSpec struct {
	PipelineExecutionName string `json:"pipelineExecutionName,omitempty" norman:"type=reference[pipelineExecution]"`
	Stage                 int    `json:"stage,omitempty" norman:"min=1"`
	Step                  int    `json:"step,omitempty" norman:"min=1"`
	Line                  int    `json:"line,omitempty"`
	Message               string `json:"message,omitempty"`
}

type RunPipelineInput struct {
	Branch string `json:"branch,omitempty"`
}

type AuthAppInput struct {
	InheritGlobal  bool   `json:"inheritGlobal,omitempty"`
	SourceCodeType string `json:"sourceCodeType,omitempty" norman:"type=string,required"`
	RedirectURL    string `json:"redirectUrl,omitempty" norman:"type=string"`
	TLS            bool   `json:"tls,omitempty"`
	Host           string `json:"host,omitempty"`
	ClientID       string `json:"clientId,omitempty" norman:"type=string,required"`
	ClientSecret   string `json:"clientSecret,omitempty" norman:"type=string,required"`
	Code           string `json:"code,omitempty" norman:"type=string,required"`
}

type AuthUserInput struct {
	SourceCodeType string `json:"sourceCodeType,omitempty" norman:"type=string,required"`
	RedirectURL    string `json:"redirectUrl,omitempty" norman:"type=string"`
	Code           string `json:"code,omitempty" norman:"type=string,required"`
}

type PipelineSystemImages struct {
	Jenkins       string `json:"jenkins,omitempty"`
	JenkinsJnlp   string `json:"jenkinsJnlp,omitempty"`
	AlpineGit     string `json:"alpineGit,omitempty"`
	PluginsDocker string `json:"pluginsDocker,omitempty"`
}
