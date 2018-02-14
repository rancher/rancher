package client

const (
	ClusterPipelineSpecType              = "clusterPipelineSpec"
	ClusterPipelineSpecFieldClusterId    = "clusterId"
	ClusterPipelineSpecFieldDeploy       = "deploy"
	ClusterPipelineSpecFieldGithubConfig = "githubConfig"
)

type ClusterPipelineSpec struct {
	ClusterId    string               `json:"clusterId,omitempty"`
	Deploy       bool                 `json:"deploy,omitempty"`
	GithubConfig *GithubClusterConfig `json:"githubConfig,omitempty"`
}
