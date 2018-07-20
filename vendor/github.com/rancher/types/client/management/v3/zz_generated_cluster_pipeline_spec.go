package client

const (
	ClusterPipelineSpecType              = "clusterPipelineSpec"
	ClusterPipelineSpecFieldClusterID    = "clusterId"
	ClusterPipelineSpecFieldDeploy       = "deploy"
	ClusterPipelineSpecFieldGithubConfig = "githubConfig"
)

type ClusterPipelineSpec struct {
	ClusterID    string               `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	Deploy       bool                 `json:"deploy,omitempty" yaml:"deploy,omitempty"`
	GithubConfig *GithubClusterConfig `json:"githubConfig,omitempty" yaml:"githubConfig,omitempty"`
}
