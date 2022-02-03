package v3

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PipelineConditionType string

const (
	PipelineExecutionConditionProvisioned condition.Cond = "Provisioned"
	PipelineExecutionConditionInitialized condition.Cond = "Initialized"
	PipelineExecutionConditionBuilt       condition.Cond = "Built"
	PipelineExecutionConditionNotified    condition.Cond = "Notified"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type SourceCodeProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	ProjectName string `json:"projectName" norman:"type=reference[project]"`
	Type        string `json:"type" norman:"options=github|gitlab|bitbucketcloud|bitbucketserver"`
}

func (s *SourceCodeProvider) ObjClusterName() string {
	if parts := strings.SplitN(s.ProjectName, ":", 2); len(parts) == 2 {
		return parts[0]
	}
	return ""
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type OauthProvider struct {
	SourceCodeProvider `json:",inline"`

	RedirectURL string `json:"redirectUrl"`
}

type GithubProvider struct {
	OauthProvider `json:",inline"`
}

type GitlabProvider struct {
	OauthProvider `json:",inline"`
}

type BitbucketCloudProvider struct {
	OauthProvider `json:",inline"`
}

type BitbucketServerProvider struct {
	OauthProvider `json:",inline"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type SourceCodeProviderConfig struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	ProjectName      string `json:"projectName" norman:"required,type=reference[project]"`
	Type             string `json:"type" norman:"noupdate,options=github|gitlab|bitbucketcloud|bitbucketserver"`
	Enabled          bool   `json:"enabled,omitempty"`
	CredentialSecret string `json:"credentialSecret,omitempty" norman:"nocreate,noupdate"`
}

func (s *SourceCodeProviderConfig) ObjClusterName() string {
	if parts := strings.SplitN(s.ProjectName, ":", 2); len(parts) == 2 {
		return parts[0]
	}
	return ""
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type GithubPipelineConfig struct {
	SourceCodeProviderConfig `json:",inline" mapstructure:",squash"`

	Hostname     string `json:"hostname,omitempty" norman:"default=github.com" norman:"noupdate"`
	TLS          bool   `json:"tls,omitempty" norman:"notnullable,default=true" norman:"noupdate"`
	ClientID     string `json:"clientId,omitempty" norman:"noupdate"`
	ClientSecret string `json:"clientSecret,omitempty" norman:"noupdate,type=password"`
	Inherit      bool   `json:"inherit,omitempty" norman:"noupdate"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type GitlabPipelineConfig struct {
	SourceCodeProviderConfig `json:",inline" mapstructure:",squash"`

	Hostname     string `json:"hostname,omitempty" norman:"default=gitlab.com" norman:"noupdate"`
	TLS          bool   `json:"tls,omitempty" norman:"notnullable,default=true" norman:"noupdate"`
	ClientID     string `json:"clientId,omitempty" norman:"noupdate"`
	ClientSecret string `json:"clientSecret,omitempty" norman:"noupdate,type=password"`
	RedirectURL  string `json:"redirectUrl,omitempty" norman:"noupdate"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type BitbucketCloudPipelineConfig struct {
	SourceCodeProviderConfig `json:",inline" mapstructure:",squash"`

	ClientID     string `json:"clientId,omitempty" norman:"noupdate"`
	ClientSecret string `json:"clientSecret,omitempty" norman:"noupdate,type=password"`
	RedirectURL  string `json:"redirectUrl,omitempty" norman:"noupdate"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type BitbucketServerPipelineConfig struct {
	SourceCodeProviderConfig `json:",inline" mapstructure:",squash"`

	Hostname    string `json:"hostname,omitempty"`
	TLS         bool   `json:"tls,omitempty"`
	ConsumerKey string `json:"consumerKey,omitempty"`
	PublicKey   string `json:"publicKey,omitempty"`
	PrivateKey  string `json:"privateKey,omitempty" norman:"type=password"`
	RedirectURL string `json:"redirectUrl,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Pipeline struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PipelineSpec   `json:"spec"`
	Status PipelineStatus `json:"status"`
}

func (p *Pipeline) ObjClusterName() string {
	return p.Spec.ObjClusterName()
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type PipelineExecution struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PipelineExecutionSpec   `json:"spec"`
	Status PipelineExecutionStatus `json:"status"`
}

func (p *PipelineExecution) ObjClusterName() string {
	return p.Spec.ObjClusterName()
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type PipelineSetting struct {
	types.Namespaced

	ProjectName       string `json:"projectName" norman:"type=reference[project]"`
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Value      string `json:"value" norman:"required"`
	Default    string `json:"default" norman:"nocreate,noupdate"`
	Customized bool   `json:"customized" norman:"nocreate,noupdate"`
}

func (p *PipelineSetting) ObjClusterName() string {
	if parts := strings.SplitN(p.ProjectName, ":", 2); len(parts) == 2 {
		return parts[0]
	}
	return ""
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type SourceCodeCredential struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SourceCodeCredentialSpec   `json:"spec"`
	Status SourceCodeCredentialStatus `json:"status"`
}

func (s *SourceCodeCredential) ObjClusterName() string {
	return s.Spec.ObjClusterName()
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type SourceCodeRepository struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SourceCodeRepositorySpec   `json:"spec"`
	Status SourceCodeRepositoryStatus `json:"status"`
}

func (s *SourceCodeRepository) ObjClusterName() string {
	return s.Spec.ObjClusterName()
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
	ProjectName string `json:"projectName" yaml:"projectName" norman:"required,type=reference[project]"`

	DisplayName        string `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	TriggerWebhookPush bool   `json:"triggerWebhookPush,omitempty" yaml:"triggerWebhookPush,omitempty"`
	TriggerWebhookPr   bool   `json:"triggerWebhookPr,omitempty" yaml:"triggerWebhookPr,omitempty"`
	TriggerWebhookTag  bool   `json:"triggerWebhookTag,omitempty" yaml:"triggerWebhookTag,omitempty"`

	RepositoryURL            string `json:"repositoryUrl,omitempty" yaml:"repositoryUrl,omitempty"`
	SourceCodeCredentialName string `json:"sourceCodeCredentialName,omitempty" yaml:"sourceCodeCredentialName,omitempty" norman:"type=reference[sourceCodeCredential],noupdate"`
}

func (p *PipelineSpec) ObjClusterName() string {
	if parts := strings.SplitN(p.ProjectName, ":", 2); len(parts) == 2 {
		return parts[0]
	}
	return ""
}

type PipelineConfig struct {
	Stages []Stage `json:"stages,omitempty" yaml:"stages,omitempty"`

	Timeout      int                   `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Branch       *Constraint           `json:"branch,omitempty" yaml:"branch,omitempty"`
	Notification *PipelineNotification `json:"notification,omitempty" yaml:"notification,omitempty"`
}

type PipelineNotification struct {
	Recipients []Recipient   `json:"recipients,omitempty" yaml:"recipients,omitempty"`
	Message    string        `json:"message,omitempty" yaml:"message,omitempty"`
	Condition  stringorslice `json:"condition,omitempty" yaml:"condition,omitempty"`
}

type Recipient struct {
	Recipient string `json:"recipient,omitempty"`
	Notifier  string `json:"notifier,omitempty"`
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

	When *Constraints `json:"when,omitempty" yaml:"when,omitempty"`
}

type Step struct {
	SourceCodeConfig     *SourceCodeConfig     `json:"sourceCodeConfig,omitempty" yaml:"sourceCodeConfig,omitempty"`
	RunScriptConfig      *RunScriptConfig      `json:"runScriptConfig,omitempty" yaml:"runScriptConfig,omitempty"`
	PublishImageConfig   *PublishImageConfig   `json:"publishImageConfig,omitempty" yaml:"publishImageConfig,omitempty"`
	ApplyYamlConfig      *ApplyYamlConfig      `json:"applyYamlConfig,omitempty" yaml:"applyYamlConfig,omitempty"`
	PublishCatalogConfig *PublishCatalogConfig `json:"publishCatalogConfig,omitempty" yaml:"publishCatalogConfig,omitempty"`
	ApplyAppConfig       *ApplyAppConfig       `json:"applyAppConfig,omitempty" yaml:"applyAppConfig,omitempty"`

	Env           map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
	EnvFrom       []EnvFrom         `json:"envFrom,omitempty" yaml:"envFrom,omitempty"`
	Privileged    bool              `json:"privileged,omitempty" yaml:"privileged,omitempty"`
	CPURequest    string            `json:"cpuRequest,omitempty" yaml:"cpuRequest,omitempty"`
	CPULimit      string            `json:"cpuLimit,omitempty" yaml:"cpuLimit,omitempty"`
	MemoryRequest string            `json:"memoryRequest,omitempty" yaml:"memoryRequest,omitempty"`
	MemoryLimit   string            `json:"memoryLimit,omitempty" yaml:"memoryLimit,omitempty"`
	When          *Constraints      `json:"when,omitempty" yaml:"when,omitempty"`
}

type Constraints struct {
	Branch *Constraint `json:"branch,omitempty" yaml:"branch,omitempty"`
	Event  *Constraint `json:"event,omitempty" yaml:"event,omitempty"`
}

type Constraint struct {
	Include []string `json:"include,omitempty" yaml:"include,omitempty"`
	Exclude []string `json:"exclude,omitempty" yaml:"exclude,omitempty"`
}

type SourceCodeConfig struct {
}

type RunScriptConfig struct {
	Image       string `json:"image,omitempty" yaml:"image,omitempty" norman:"required"`
	ShellScript string `json:"shellScript,omitempty" yaml:"shellScript,omitempty"`
}

type PublishImageConfig struct {
	DockerfilePath string `json:"dockerfilePath,omittempty" yaml:"dockerfilePath,omitempty" norman:"required,default=./Dockerfile"`
	BuildContext   string `json:"buildContext,omitempty" yaml:"buildContext,omitempty" norman:"required,default=."`
	Tag            string `json:"tag,omitempty" yaml:"tag,omitempty" norman:"required,default=${CICD_GIT_REPOSITORY_NAME}:${CICD_GIT_BRANCH}"`
	PushRemote     bool   `json:"pushRemote,omitempty" yaml:"pushRemote,omitempty"`
	Registry       string `json:"registry,omitempty" yaml:"registry,omitempty"`
}

type ApplyYamlConfig struct {
	Path      string `json:"path,omitempty" yaml:"path,omitempty"`
	Content   string `json:"content,omitempty" yaml:"content,omitempty"`
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
}

type PublishCatalogConfig struct {
	Path            string `json:"path,omitempty" yaml:"path,omitempty"`
	CatalogTemplate string `json:"catalogTemplate,omitempty" yaml:"catalogTemplate,omitempty"`
	Version         string `json:"version,omitempty" yaml:"version,omitempty"`
	GitURL          string `json:"gitUrl,omitempty" yaml:"gitUrl,omitempty"`
	GitBranch       string `json:"gitBranch,omitempty" yaml:"gitBranch,omitempty"`
	GitAuthor       string `json:"gitAuthor,omitempty" yaml:"gitAuthor,omitempty"`
	GitEmail        string `json:"gitEmail,omitempty" yaml:"gitEmail,omitempty"`
}

type ApplyAppConfig struct {
	CatalogTemplate string            `json:"catalogTemplate,omitempty" yaml:"catalogTemplate,omitempty"`
	Version         string            `json:"version,omitempty" yaml:"version,omitempty"`
	Answers         map[string]string `json:"answers,omitempty" yaml:"answers,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	TargetNamespace string            `json:"targetNamespace,omitempty" yaml:"targetNamespace,omitempty"`
}

type PipelineExecutionSpec struct {
	ProjectName string `json:"projectName" yaml:"projectName" norman:"required,type=reference[project]"`

	PipelineName    string         `json:"pipelineName" norman:"required,type=reference[pipeline]"`
	PipelineConfig  PipelineConfig `json:"pipelineConfig,omitempty" norman:"required"`
	RepositoryURL   string         `json:"repositoryUrl,omitempty"`
	Run             int            `json:"run,omitempty" norman:"required,min=1"`
	TriggeredBy     string         `json:"triggeredBy,omitempty" norman:"required,options=user|cron|webhook"`
	TriggerUserName string         `json:"triggerUserName,omitempty" norman:"type=reference[user]"`
	Commit          string         `json:"commit,omitempty"`
	Event           string         `json:"event,omitempty"`
	Branch          string         `json:"branch,omitempty"`
	Ref             string         `json:"ref,omitempty"`
	HTMLLink        string         `json:"htmlLink,omitempty"`
	Title           string         `json:"title,omitempty"`
	Message         string         `json:"message,omitempty"`
	Author          string         `json:"author,omitempty"`
	AvatarURL       string         `json:"avatarUrl,omitempty"`
	Email           string         `json:"email,omitempty"`
}

func (p *PipelineExecutionSpec) ObjClusterName() string {
	if parts := strings.SplitN(p.ProjectName, ":", 2); len(parts) == 2 {
		return parts[0]
	}
	return ""
}

type PipelineExecutionStatus struct {
	Conditions []PipelineCondition `json:"conditions,omitempty"`

	ExecutionState string        `json:"executionState,omitempty"`
	Started        string        `json:"started,omitempty"`
	Ended          string        `json:"ended,omitempty"`
	Stages         []StageStatus `json:"stages,omitempty"`
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
	ProjectName    string `json:"projectName" norman:"type=reference[project]"`
	SourceCodeType string `json:"sourceCodeType,omitempty" norman:"required,options=github|gitlab|bitbucketcloud|bitbucketserver"`
	UserName       string `json:"userName" norman:"required,type=reference[user]"`
	DisplayName    string `json:"displayName,omitempty" norman:"required"`
	AvatarURL      string `json:"avatarUrl,omitempty"`
	HTMLURL        string `json:"htmlUrl,omitempty"`
	LoginName      string `json:"loginName,omitempty"`
	GitLoginName   string `json:"gitLoginName,omitempty"`
	GitCloneToken  string `json:"gitCloneToken,omitempty" norman:"writeOnly,noupdate"`
	AccessToken    string `json:"accessToken,omitempty" norman:"writeOnly,noupdate"`
	RefreshToken   string `json:"refreshToken,omitempty" norman:"writeOnly,noupdate"`
	Expiry         string `json:"expiry,omitempty"`
}

func (s *SourceCodeCredentialSpec) ObjClusterName() string {
	if parts := strings.SplitN(s.ProjectName, ":", 2); len(parts) == 2 {
		return parts[0]
	}
	return ""
}

type SourceCodeCredentialStatus struct {
	Logout bool `json:"logout,omitempty"`
}

type SourceCodeRepositorySpec struct {
	ProjectName              string   `json:"projectName" norman:"type=reference[project]"`
	SourceCodeType           string   `json:"sourceCodeType,omitempty" norman:"required,options=github|gitlab|bitbucketcloud|bitbucketserver"`
	UserName                 string   `json:"userName" norman:"required,type=reference[user]"`
	SourceCodeCredentialName string   `json:"sourceCodeCredentialName,omitempty" norman:"required,type=reference[sourceCodeCredential]"`
	URL                      string   `json:"url,omitempty"`
	Permissions              RepoPerm `json:"permissions,omitempty"`
	Language                 string   `json:"language,omitempty"`
	DefaultBranch            string   `json:"defaultBranch,omitempty"`
}

func (s *SourceCodeRepositorySpec) ObjClusterName() string {
	if parts := strings.SplitN(s.ProjectName, ":", 2); len(parts) == 2 {
		return parts[0]
	}
	return ""
}

type SourceCodeRepositoryStatus struct {
}

type RepoPerm struct {
	Pull  bool `json:"pull,omitempty"`
	Push  bool `json:"push,omitempty"`
	Admin bool `json:"admin,omitempty"`
}

type RunPipelineInput struct {
	Branch string `json:"branch,omitempty"`
}

type AuthAppInput struct {
	InheritGlobal  bool   `json:"inheritGlobal,omitempty"`
	SourceCodeType string `json:"sourceCodeType,omitempty" norman:"type=string,required,options=github|gitlab|bitbucketcloud|bitbucketserver"`
	RedirectURL    string `json:"redirectUrl,omitempty" norman:"type=string"`
	TLS            bool   `json:"tls,omitempty"`
	Host           string `json:"host,omitempty"`
	ClientID       string `json:"clientId,omitempty" norman:"type=string,required"`
	ClientSecret   string `json:"clientSecret,omitempty" norman:"type=string,required"`
	Code           string `json:"code,omitempty" norman:"type=string,required"`
}

type AuthUserInput struct {
	SourceCodeType string `json:"sourceCodeType,omitempty" norman:"type=string,required,options=github|gitlab|bitbucketcloud|bitbucketserver"`
	RedirectURL    string `json:"redirectUrl,omitempty" norman:"type=string"`
	Code           string `json:"code,omitempty" norman:"type=string,required"`
}

type PushPipelineConfigInput struct {
	Configs map[string]PipelineConfig `json:"configs,omitempty"`
}

type PipelineSystemImages struct {
	Jenkins       string `json:"jenkins,omitempty"`
	JenkinsJnlp   string `json:"jenkinsJnlp,omitempty"`
	AlpineGit     string `json:"alpineGit,omitempty"`
	PluginsDocker string `json:"pluginsDocker,omitempty"`
	Minio         string `json:"minio,omitempty"`
	Registry      string `json:"registry,omitempty"`
	RegistryProxy string `json:"registryProxy,omitempty"`
	KubeApply     string `json:"kubeApply,omitempty"`
}

type OauthApplyInput struct {
	Hostname     string `json:"hostname,omitempty"`
	TLS          bool   `json:"tls,omitempty"`
	RedirectURL  string `json:"redirectUrl,omitempty"`
	ClientID     string `json:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
	Code         string `json:"code,omitempty"`
}

type GithubApplyInput struct {
	OauthApplyInput
	InheritAuth bool `json:"inheritAuth,omitempty"`
}

type GitlabApplyInput struct {
	OauthApplyInput
}

type BitbucketCloudApplyInput struct {
	OauthApplyInput
}

type BitbucketServerApplyInput struct {
	OAuthToken    string `json:"oauthToken,omitempty"`
	OAuthVerifier string `json:"oauthVerifier,omitempty"`
	Hostname      string `json:"hostname,omitempty"`
	TLS           bool   `json:"tls,omitempty"`
	RedirectURL   string `json:"redirectUrl,omitempty"`
}

type BitbucketServerRequestLoginInput struct {
	Hostname    string `json:"hostname,omitempty"`
	TLS         bool   `json:"tls,omitempty"`
	RedirectURL string `json:"redirectUrl,omitempty"`
}

type BitbucketServerRequestLoginOutput struct {
	LoginURL string `json:"loginUrl"`
}

type EnvFrom struct {
	SourceName string `json:"sourceName,omitempty" yaml:"sourceName,omitempty" norman:"type=string,required"`
	SourceKey  string `json:"sourceKey,omitempty" yaml:"sourceKey,omitempty" norman:"type=string,required"`
	TargetKey  string `json:"targetKey,omitempty" yaml:"targetKey,omitempty"`
}

// UnmarshalYAML unmarshals the constraint.
// So as to support yaml syntax including:
// branch: dev,  branch: ["dev","hotfix"], branch: {include:[],exclude:[]}
func (c *Constraint) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var out1 = struct {
		Include stringorslice
		Exclude stringorslice
	}{}

	var out2 stringorslice

	unmarshal(&out1)
	unmarshal(&out2)

	c.Exclude = out1.Exclude
	c.Include = append(
		out1.Include,
		out2...,
	)
	return nil
}

type stringorslice []string

// UnmarshalYAML implements the Unmarshaller interface.
func (s *stringorslice) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var stringType string
	if err := unmarshal(&stringType); err == nil {
		*s = []string{stringType}
		return nil
	}

	var sliceType []interface{}
	if err := unmarshal(&sliceType); err == nil {
		*s = convert.ToStringSlice(sliceType)
		return nil
	}

	return errors.New("Failed to unmarshal stringorslice")
}
