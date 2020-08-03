package remote

import (
	"errors"

	v32 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"

	"github.com/rancher/rancher/pkg/pipeline/remote/bitbucketcloud"
	"github.com/rancher/rancher/pkg/pipeline/remote/bitbucketserver"
	"github.com/rancher/rancher/pkg/pipeline/remote/github"
	"github.com/rancher/rancher/pkg/pipeline/remote/gitlab"
	"github.com/rancher/rancher/pkg/pipeline/remote/model"
)

func New(config interface{}) (model.Remote, error) {
	if config == nil {
		return github.New(nil)
	}
	switch config := config.(type) {
	case *v32.GithubPipelineConfig:
		return github.New(config)
	case *v32.GitlabPipelineConfig:
		return gitlab.New(config)
	case *v32.BitbucketCloudPipelineConfig:
		return bitbucketcloud.New(config)
	case *v32.BitbucketServerPipelineConfig:
		return bitbucketserver.New(config)
	}

	return nil, errors.New("unsupported remote type")
}
