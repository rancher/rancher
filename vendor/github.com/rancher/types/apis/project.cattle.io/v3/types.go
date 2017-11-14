package v3

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type WorkloadSpec struct {
	DeployConfig DeployConfig
	Template     v1.PodTemplateSpec
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
	Scale              int64
	BatchSize          string
	DeploymentStrategy *DeployStrategy
}

type DeploymentParallelConfig struct {
	StartFirst              bool
	MinReadySeconds         int64
	ProgressDeadlineSeconds int64
}

type DeploymentJobConfig struct {
	BatchLimit            int64
	ActiveDeadlineSeconds int64
	OnDelete              bool
}

type DeploymentOrderedConfig struct {
	PartitionSize int64
	OnDelete      bool
}

type DeploymentGlobalConfig struct {
	OnDelete bool
}

type DeployStrategy struct {
	Kind           string
	ParallelConfig *DeploymentParallelConfig
	JobConfig      *DeploymentJobConfig
	OrderedConfig  *DeploymentOrderedConfig
	GlobalConfig   *DeploymentGlobalConfig
}
