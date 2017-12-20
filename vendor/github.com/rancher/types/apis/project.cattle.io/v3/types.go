package v3

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type WorkloadSpec struct {
	DeployConfig DeployConfig       `json:"deployConfig"`
	Template     v1.PodTemplateSpec `json:"template"`
	ServiceLinks []Link             `json:"serviceLinks"`
}

type WorkloadStatus struct {
}

type Workload struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              WorkloadSpec    `json:"spec"`
	Status            *WorkloadStatus `json:"status"`
}

type DeployConfig struct {
	Scale              int64           `json:"scale"`
	BatchSize          string          `json:"batchSize"`
	DeploymentStrategy *DeployStrategy `json:"deploymentStrategy"`
}

type DeploymentParallelConfig struct {
	StartFirst              bool  `json:"startFirst"`
	MinReadySeconds         int64 `json:"minReadySeconds"`
	ProgressDeadlineSeconds int64 `json:"processDeadlineSeconds"`
}

type DeploymentJobConfig struct {
	BatchLimit            int64 `json:"batchLimit"`
	ActiveDeadlineSeconds int64 `json:"activeDeadlineSeconds"`
	OnDelete              bool  `json:"onDelete"`
}

type DeploymentOrderedConfig struct {
	PartitionSize int64 `json:"partitionSize"`
	OnDelete      bool  `json:"onDelete"`
}

type DeploymentGlobalConfig struct {
	OnDelete bool `json:"onDelete"`
}

type DeployStrategy struct {
	Kind           string                    `json:"kind"`
	ParallelConfig *DeploymentParallelConfig `json:"parallelConfig"`
	JobConfig      *DeploymentJobConfig      `json:"jobConfig"`
	OrderedConfig  *DeploymentOrderedConfig  `json:"orderedConfig"`
	GlobalConfig   *DeploymentGlobalConfig   `json:"globalConfig"`
}

type Link struct {
	Name  string `json:"name"`
	Alias string `json:"alias"`
}
