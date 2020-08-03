package hooks

import (
	"net/http"

	"github.com/rancher/rancher/pkg/pipeline/hooks/drivers"
	"github.com/rancher/rancher/pkg/types/config"
)

var Drivers map[string]Driver

type Driver interface {
	Execute(req *http.Request) (int, error)
}

func RegisterDrivers(Management *config.ScaledContext) {
	pipelineLister := Management.Project.Pipelines("").Controller().Lister()
	pipelineExecutions := Management.Project.PipelineExecutions("")
	sourceCodeCredentials := Management.Project.SourceCodeCredentials("")
	sourceCodeCredentialLister := Management.Project.SourceCodeCredentials("").Controller().Lister()

	Drivers = map[string]Driver{}
	Drivers[drivers.GithubWebhookHeader] = drivers.GithubDriver{
		PipelineLister:             pipelineLister,
		PipelineExecutions:         pipelineExecutions,
		SourceCodeCredentials:      sourceCodeCredentials,
		SourceCodeCredentialLister: sourceCodeCredentialLister,
	}
	Drivers[drivers.GitlabWebhookHeader] = drivers.GitlabDriver{
		PipelineLister:             pipelineLister,
		PipelineExecutions:         pipelineExecutions,
		SourceCodeCredentials:      sourceCodeCredentials,
		SourceCodeCredentialLister: sourceCodeCredentialLister,
	}
	Drivers[drivers.BitbucketCloudWebhookHeader] = drivers.BitbucketCloudDriver{
		PipelineLister:             pipelineLister,
		PipelineExecutions:         pipelineExecutions,
		SourceCodeCredentials:      sourceCodeCredentials,
		SourceCodeCredentialLister: sourceCodeCredentialLister,
	}
	Drivers[drivers.BitbucketServerWebhookHeader] = drivers.BitbucketServerDriver{
		PipelineLister:             pipelineLister,
		PipelineExecutions:         pipelineExecutions,
		SourceCodeCredentials:      sourceCodeCredentials,
		SourceCodeCredentialLister: sourceCodeCredentialLister,
	}
}
