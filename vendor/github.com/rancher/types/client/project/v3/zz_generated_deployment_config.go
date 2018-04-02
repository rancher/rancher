package client

import "k8s.io/apimachinery/pkg/util/intstr"

const (
	DeploymentConfigType                         = "deploymentConfig"
	DeploymentConfigFieldMaxSurge                = "maxSurge"
	DeploymentConfigFieldMaxUnavailable          = "maxUnavailable"
	DeploymentConfigFieldMinReadySeconds         = "minReadySeconds"
	DeploymentConfigFieldProgressDeadlineSeconds = "progressDeadlineSeconds"
	DeploymentConfigFieldRevisionHistoryLimit    = "revisionHistoryLimit"
	DeploymentConfigFieldStrategy                = "strategy"
)

type DeploymentConfig struct {
	MaxSurge                intstr.IntOrString `json:"maxSurge,omitempty" yaml:"maxSurge,omitempty"`
	MaxUnavailable          intstr.IntOrString `json:"maxUnavailable,omitempty" yaml:"maxUnavailable,omitempty"`
	MinReadySeconds         int64              `json:"minReadySeconds,omitempty" yaml:"minReadySeconds,omitempty"`
	ProgressDeadlineSeconds *int64             `json:"progressDeadlineSeconds,omitempty" yaml:"progressDeadlineSeconds,omitempty"`
	RevisionHistoryLimit    *int64             `json:"revisionHistoryLimit,omitempty" yaml:"revisionHistoryLimit,omitempty"`
	Strategy                string             `json:"strategy,omitempty" yaml:"strategy,omitempty"`
}
