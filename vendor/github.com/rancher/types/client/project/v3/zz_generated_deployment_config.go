package client

const (
	DeploymentConfigType                         = "deploymentConfig"
	DeploymentConfigFieldMinReadySeconds         = "minReadySeconds"
	DeploymentConfigFieldPaused                  = "paused"
	DeploymentConfigFieldProgressDeadlineSeconds = "progressDeadlineSeconds"
	DeploymentConfigFieldRevisionHistoryLimit    = "revisionHistoryLimit"
	DeploymentConfigFieldStrategy                = "strategy"
)

type DeploymentConfig struct {
	MinReadySeconds         *int64              `json:"minReadySeconds,omitempty"`
	Paused                  bool                `json:"paused,omitempty"`
	ProgressDeadlineSeconds *int64              `json:"progressDeadlineSeconds,omitempty"`
	RevisionHistoryLimit    *int64              `json:"revisionHistoryLimit,omitempty"`
	Strategy                *DeploymentStrategy `json:"strategy,omitempty"`
}
