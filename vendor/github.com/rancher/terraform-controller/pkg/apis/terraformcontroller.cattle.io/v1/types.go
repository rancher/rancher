package v1

import (
	"github.com/rancher/wrangler/pkg/condition"
	"github.com/rancher/wrangler/pkg/genericcondition"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	ModuleConditionGitUpdated = condition.Cond("GitUpdated")

	StateConditionJobDeployed      = condition.Cond("JobDeployed")
	ExecutionConditionMissingInfo  = condition.Cond("MissingInfo")
	ExecutionConditionWatchRunning = condition.Cond("WatchRunning")
	StateConditionDestroyed        = condition.Cond("Destroyed")

	ExecutionRunConditionPlanned = condition.Cond("Planned")
	ExecutionRunConditionApplied = condition.Cond("Applied")
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Module struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ModuleSpec   `json:"spec"`
	Status ModuleStatus `json:"status"`
}

type ModuleSpec struct {
	ModuleContent
}

type ModuleContent struct {
	Content map[string]string `json:"content,omitempty"`
	Git     GitLocation       `json:"git,omitempty"`
}

type ModuleStatus struct {
	CheckTime   metav1.Time                         `json:"time,omitempty"`
	GitChecked  *GitLocation                        `json:"gitChecked,omitempty"`
	Content     ModuleContent                       `json:"content,omitempty"`
	ContentHash string                              `json:"contentHash,omitempty"`
	Conditions  []genericcondition.GenericCondition `json:"conditions,omitempty"`
}

type GitLocation struct {
	URL             string `json:"url,omitempty"`
	Branch          string `json:"branch,omitempty"`
	Tag             string `json:"tag,omitempty"`
	Commit          string `json:"commit,omitempty"`
	SecretName      string `json:"secretName,omitempty"`
	IntervalSeconds int    `json:"intervalSeconds,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type State struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StateSpec   `json:"spec"`
	Status StateStatus `json:"status"`
}

type Variables struct {
	EnvConfigName  []string `json:"envConfigNames,omitempty"`
	EnvSecretNames []string `json:"envSecretNames,omitempty"`
	ConfigNames    []string `json:"configNames,omitempty"`
	SecretNames    []string `json:"secretNames,omitempty"`
}

type StateSpec struct {
	Image      string    `json:"image,omitempty"`
	Variables  Variables `json:"variables,omitempty"`
	ModuleName string    `json:"moduleName,omitempty"`
	// Data is dataName mapped to another execution name
	// so terraform variable name that should be an output from the run
	Data            map[string]string `json:"data,omitempty"`
	AutoConfirm     bool              `json:"autoConfirm,omitempty"`
	DestroyOnDelete bool              `json:"destroyOnDelete,omitempty"`
	Version         int32             `json:"version,omitempty"`
}

type StateStatus struct {
	Conditions    []genericcondition.GenericCondition `json:"conditions,omitempty"`
	LastRunHash   string                              `json:"lastRunHash,omitempty"`
	ExecutionName string                              `json:"executionName,omitempty"`
	StatePlanName string                              `json:"executionPlanName,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Execution struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ExecutionSpec   `json:"spec"`
	Status ExecutionStatus `json:"status"`
}

type ExecutionSpec struct {
	AutoConfirm      bool              `json:"autoConfirm,omitempty"`
	Content          ModuleContent     `json:"content,omitempty"`
	ContentHash      string            `json:"contentHash,omitempty"`
	RunHash          string            `json:"runHash,omitempty"`
	Data             map[string]string `json:"data,omitempty"`
	ExecutionName    string            `json:"executionName,omitempty"`
	ExecutionVersion int32             `json:"executionVersion,omitempty"`
	// Secrets and config maps referenced in the Execution spec will be combined into this secret
	SecretName string `json:"secretName,omitempty"`
}

type ExecutionStatus struct {
	Conditions    []genericcondition.GenericCondition `json:"conditions,omitempty"`
	JobName       string                              `json:"jobName,omitempty"`
	JobLogs       string                              `json:"jobLogs,omitempty"`
	PlanOutput    string                              `json:"planOutput,omitempty"`
	PlanConfirmed bool                                `json:"planConfirmed,omitempty"`
	ApplyOutput   string                              `json:"applyOutput,omitempty"`
	Outputs       string                              `json:"outputs,omitempty"`
}
