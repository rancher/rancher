package hooks

import (
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/hooks/drivers"
	"github.com/rancher/types/config"
	"net/http"
)

var Drivers map[string]Driver

type Driver interface {
	Execute(req *http.Request) (int, error)
}

func RegisterDrivers(Management *config.ScaledContext) {

	pipelines := Management.Management.Pipelines("")
	pipelineExecutions := Management.Management.PipelineExecutions("")

	Drivers = map[string]Driver{}
	Drivers[drivers.GithubWebhookHeader] = drivers.GithubDriver{
		Pipelines:          pipelines,
		PipelineExecutions: pipelineExecutions,
	}
	Drivers[drivers.GitlabWebhookHeader] = drivers.GitlabDriver{
		Pipelines:          pipelines,
		PipelineExecutions: pipelineExecutions,
	}
}
