package v3

import (
	"strings"

	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type App struct {
	types.Namespaced
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppSpec   `json:"spec,omitempty"`
	Status AppStatus `json:"status,omitempty"`
}

func (a *App) ObjClusterName() string {
	return a.Spec.ObjClusterName()
}

type AppSpec struct {
	ProjectName         string            `json:"projectName,omitempty" norman:"type=reference[/v3/schemas/project]"`
	Description         string            `json:"description,omitempty"`
	TargetNamespace     string            `json:"targetNamespace,omitempty"`
	ExternalID          string            `json:"externalId,omitempty"`
	Files               map[string]string `json:"files,omitempty"`
	Answers             map[string]string `json:"answers,omitempty"`
	AnswersSetString    map[string]string `json:"answersSetString,omitempty"`
	Wait                bool              `json:"wait,omitempty"`
	Timeout             int               `json:"timeout,omitempty" norman:"min=1,default=300"`
	AppRevisionName     string            `json:"appRevisionName,omitempty" norman:"type=reference[/v3/project/schemas/apprevision]"`
	Prune               bool              `json:"prune,omitempty"`
	MultiClusterAppName string            `json:"multiClusterAppName,omitempty" norman:"type=reference[/v3/schemas/multiclusterapp]"`
	ValuesYaml          string            `json:"valuesYaml,omitempty"`
	MaxRevisionCount    int               `json:"maxRevisionCount,omitempty"`
}

func (a *AppSpec) ObjClusterName() string {
	if parts := strings.SplitN(a.ProjectName, ":", 2); len(parts) == 2 {
		return parts[0]
	}
	return ""
}

var (
	AppConditionInstalled                  condition.Cond = "Installed"
	AppConditionMigrated                   condition.Cond = "Migrated"
	AppConditionDeployed                   condition.Cond = "Deployed"
	AppConditionForceUpgrade               condition.Cond = "ForceUpgrade"
	AppConditionUserTriggeredAction        condition.Cond = "UserTriggeredAction"
	IstioConditionMetricExpressionDeployed condition.Cond = "MetricExpressionDeployed"
)

type AppStatus struct {
	AppliedFiles         map[string]string `json:"appliedFiles,omitempty"`
	Notes                string            `json:"notes,omitempty"`
	Conditions           []AppCondition    `json:"conditions,omitempty"`
	LastAppliedTemplates string            `json:"lastAppliedTemplate,omitempty"`
	HelmVersion          string            `json:"helmVersion,omitempty" norman:"noupdate,nocreate"`
}

type AppCondition struct {
	// Type of cluster condition.
	Type condition.Cond `json:"type"`
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

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AppRevision struct {
	types.Namespaced
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppRevisionSpec   `json:"spec,omitempty"`
	Status AppRevisionStatus `json:"status,omitempty"`
}

type AppRevisionSpec struct {
	ProjectName string `json:"projectName,omitempty" norman:"type=reference[/v3/schemas/project]"`
}

func (a *AppRevisionSpec) ObjClusterName() string {
	if parts := strings.SplitN(a.ProjectName, ":", 2); len(parts) == 2 {
		return parts[0]
	}
	return ""
}

type AppRevisionStatus struct {
	ProjectName      string            `json:"projectName,omitempty" norman:"type=reference[/v3/schemas/project]"`
	ExternalID       string            `json:"externalId"`
	Answers          map[string]string `json:"answers"`
	AnswersSetString map[string]string `json:"answersSetString"`
	Digest           string            `json:"digest"`
	ValuesYaml       string            `json:"valuesYaml,omitempty"`
	Files            map[string]string `json:"files,omitempty"`
}

func (a *AppRevisionStatus) ObjClusterName() string {
	if parts := strings.SplitN(a.ProjectName, ":", 2); len(parts) == 2 {
		return parts[0]
	}
	return ""
}

type AppUpgradeConfig struct {
	ExternalID       string            `json:"externalId,omitempty"`
	Answers          map[string]string `json:"answers,omitempty"`
	AnswersSetString map[string]string `json:"answersSetString,omitempty"`
	ForceUpgrade     bool              `json:"forceUpgrade,omitempty"`
	Files            map[string]string `json:"files,omitempty"`
	ValuesYaml       string            `json:"valuesYaml,omitempty"`
}

type RollbackRevision struct {
	RevisionName string `json:"revisionName,omitempty" norman:"type=reference[/v3/project/schemas/apprevision]"`
	ForceUpgrade bool   `json:"forceUpgrade,omitempty"`
}
