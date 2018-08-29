package hooks

import (
	"github.com/rancher/rancher/pkg/pipeline/hooks/drivers"
	"github.com/rancher/types/config"
	"net/http"
)

var Drivers map[string]Driver

type Driver interface {
	Execute(req *http.Request) (int, error)
}

func RegisterDrivers(Management *config.ScaledContext) {
	pipelineLister := Management.Project.Pipelines("").Controller().Lister()
	pipelineExecutions := Management.Project.PipelineExecutions("")
	sourceCodeCredentialLister := Management.Project.SourceCodeCredentials("").Controller().Lister()

	Drivers = map[string]Driver{}
	Drivers[drivers.GithubWebhookHeader] = drivers.GithubDriver{
		PipelineLister:             pipelineLister,
		PipelineExecutions:         pipelineExecutions,
		SourceCodeCredentialLister: sourceCodeCredentialLister,
	}
	Drivers[drivers.GitlabWebhookHeader] = drivers.GitlabDriver{
		PipelineLister:             pipelineLister,
		PipelineExecutions:         pipelineExecutions,
		SourceCodeCredentialLister: sourceCodeCredentialLister,
	}
}
