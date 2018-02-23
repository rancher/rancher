package remote

import (
	"errors"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/remote/github"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/remote/model"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

func New(pipeline v3.ClusterPipeline, remoteType string) (model.Remote, error) {
	switch remoteType {
	case "github":
		return github.New(pipeline)
	}
	return nil, errors.New("unsupported remote type")
}
