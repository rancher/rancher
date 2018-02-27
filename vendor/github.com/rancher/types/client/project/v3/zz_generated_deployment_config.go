package client

import "k8s.io/apimachinery/pkg/util/intstr"

const (
	DeploymentConfigType                         = "deploymentConfig"
	DeploymentConfigFieldMaxSurge                = "maxSurge"
	DeploymentConfigFieldMaxUnavailable          = "maxUnavailable"
	DeploymentConfigFieldMinReadySeconds         = "minReadySeconds"
	DeploymentConfigFieldPaused                  = "paused"
	DeploymentConfigFieldProgressDeadlineSeconds = "progressDeadlineSeconds"
	DeploymentConfigFieldRevisionHistoryLimit    = "revisionHistoryLimit"
	DeploymentConfigFieldStrategy                = "strategy"
)

type DeploymentConfig struct {
	MaxSurge                intstr.IntOrString `json:"maxSurge,omitempty"`
	MaxUnavailable          intstr.IntOrString `json:"maxUnavailable,omitempty"`
	MinReadySeconds         *int64             `json:"minReadySeconds,omitempty"`
	Paused                  bool               `json:"paused,omitempty"`
	ProgressDeadlineSeconds *int64             `json:"progressDeadlineSeconds,omitempty"`
	RevisionHistoryLimit    *int64             `json:"revisionHistoryLimit,omitempty"`
	Strategy                string             `json:"strategy,omitempty"`
}
