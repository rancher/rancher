package remote

import (
	"errors"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/remote/github"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/remote/gitlab"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/remote/model"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

func New(pipeline v3.ClusterPipeline, remoteType string) (model.Remote, error) {

	if remoteType == "" {
		remoteType = inferRemoteType(pipeline)
	}
	switch remoteType {
	case model.GithubType:
		return github.New(pipeline)
	case model.GitlabType:
		return gitlab.New(pipeline)
	}
	return nil, errors.New("unsupported remote type")
}

func inferRemoteType(pipeline v3.ClusterPipeline) string {
	if pipeline.Spec.GithubConfig != nil {
		return model.GithubType
	}
	if pipeline.Spec.GitlabConfig != nil {
		return model.GitlabType
	}
	return ""
}
