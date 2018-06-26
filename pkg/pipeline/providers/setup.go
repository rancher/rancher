package providers

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/pipeline/providers/github"
	"github.com/rancher/rancher/pkg/pipeline/providers/gitlab"
	"github.com/rancher/rancher/pkg/pipeline/remote/model"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/types/apis/project.cattle.io/v3/schema"
	"github.com/rancher/types/client/project/v3"
	"github.com/rancher/types/config"
	"k8s.io/client-go/tools/cache"
)

var (
	providers       = make(map[string]SourceCodeProvider)
	providersByType = make(map[string]SourceCodeProvider)
)

var sourceCodeProviderConfigTypes = []string{
	client.GithubPipelineConfigType,
	client.GitlabPipelineConfigType,
}

func SetupSourceCodeProviderConfig(management *config.ScaledContext, schemas *types.Schemas) {
	configure(management)

	providerBaseSchema := schemas.Schema(&schema.Version, client.SourceCodeProviderType)
	setSourceCodeProviderStore(providerBaseSchema, management)

	for _, scpSubtype := range sourceCodeProviderConfigTypes {
		providersByType[scpSubtype].CustomizeSchemas(schemas)
	}
}

func configure(management *config.ScaledContext) {

	// Indexers for looking up resources by projectName, etc.
	pipelineInformer := management.Project.Pipelines("").Controller().Informer()
	pipelineIndexers := map[string]cache.IndexFunc{
		utils.PipelineByProjectIndex: utils.PipelineByProjectName,
	}
	pipelineInformer.AddIndexers(pipelineIndexers)
	executionInformer := management.Project.PipelineExecutions("").Controller().Informer()
	executionIndexers := map[string]cache.IndexFunc{
		utils.PipelineExecutionByProjectIndex: utils.PipelineExecutionByProjectName,
	}
	executionInformer.AddIndexers(executionIndexers)
	sourceCodeCredentialInformer := management.Project.SourceCodeCredentials("").Controller().Informer()
	sourceCodeCredentialIndexers := map[string]cache.IndexFunc{
		utils.SourceCodeCredentialByProjectAndTypeIndex: utils.SourceCodeCredentialByProjectNameAndType,
	}
	sourceCodeCredentialInformer.AddIndexers(sourceCodeCredentialIndexers)
	sourceCodeRepositoryInformer := management.Project.SourceCodeRepositories("").Controller().Informer()
	sourceCodeRepositoryIndexers := map[string]cache.IndexFunc{
		utils.SourceCodeRepositoryByCredentialIndex:     utils.SourceCodeRepositoryByCredentialName,
		utils.SourceCodeRepositoryByProjectAndTypeIndex: utils.SourceCodeRepositoryByProjectNameAndType,
	}
	sourceCodeRepositoryInformer.AddIndexers(sourceCodeRepositoryIndexers)

	ghProvider := &github.GhProvider{
		SourceCodeProviderConfigs:  management.Project.SourceCodeProviderConfigs(""),
		SourceCodeCredentialLister: management.Project.SourceCodeCredentials("").Controller().Lister(),
		SourceCodeCredentials:      management.Project.SourceCodeCredentials(""),
		SourceCodeRepositories:     management.Project.SourceCodeRepositories(""),
		Pipelines:                  management.Project.Pipelines(""),
		PipelineExecutions:         management.Project.PipelineExecutions(""),

		PipelineIndexer:             pipelineInformer.GetIndexer(),
		PipelineExecutionIndexer:    executionInformer.GetIndexer(),
		SourceCodeCredentialIndexer: sourceCodeCredentialInformer.GetIndexer(),
		SourceCodeRepositoryIndexer: sourceCodeRepositoryInformer.GetIndexer(),

		AuthConfigs: management.Management.AuthConfigs(""),
	}
	providers[model.GithubType] = ghProvider
	providersByType[client.GithubPipelineConfigType] = ghProvider

	glProvider := &gitlab.GlProvider{
		SourceCodeProviderConfigs:  management.Project.SourceCodeProviderConfigs(""),
		SourceCodeCredentialLister: management.Project.SourceCodeCredentials("").Controller().Lister(),
		SourceCodeCredentials:      management.Project.SourceCodeCredentials(""),
		SourceCodeRepositories:     management.Project.SourceCodeRepositories(""),
		Pipelines:                  management.Project.Pipelines(""),
		PipelineExecutions:         management.Project.PipelineExecutions(""),

		PipelineIndexer:             pipelineInformer.GetIndexer(),
		PipelineExecutionIndexer:    executionInformer.GetIndexer(),
		SourceCodeCredentialIndexer: sourceCodeCredentialInformer.GetIndexer(),
		SourceCodeRepositoryIndexer: sourceCodeRepositoryInformer.GetIndexer(),

		AuthConfigs: management.Management.AuthConfigs(""),
	}
	providers[model.GitlabType] = glProvider
	providersByType[client.GitlabPipelineConfigType] = glProvider

}
