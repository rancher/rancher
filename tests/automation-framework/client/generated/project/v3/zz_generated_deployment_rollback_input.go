package client

const (
	DeploymentRollbackInputType              = "deploymentRollbackInput"
	DeploymentRollbackInputFieldReplicaSetID = "replicaSetId"
)

type DeploymentRollbackInput struct {
	ReplicaSetID string `json:"replicaSetId,omitempty" yaml:"replicaSetId,omitempty"`
}
