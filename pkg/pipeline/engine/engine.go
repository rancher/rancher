package engine

import (
	v3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/pipeline/engine/jenkins"
	"github.com/rancher/rancher/pkg/types/config"
)

type PipelineEngine interface {
	PreCheck(execution *v3.PipelineExecution) (bool, error)
	RunPipelineExecution(execution *v3.PipelineExecution) error
	RerunExecution(execution *v3.PipelineExecution) error
	StopExecution(execution *v3.PipelineExecution) error
	GetStepLog(execution *v3.PipelineExecution, stage int, step int) (string, error)
	SyncExecution(execution *v3.PipelineExecution) (bool, error)
}

func New(cluster *config.UserContext, useCache bool) PipelineEngine {
	secrets := cluster.Core.Secrets("")
	managementSecretLister := cluster.Management.Core.Secrets("").Controller().Lister()
	sourceCodeCredentials := cluster.Management.Project.SourceCodeCredentials("")
	sourceCodeCredentialLister := sourceCodeCredentials.Controller().Lister()
	pipelineLister := cluster.Management.Project.Pipelines("").Controller().Lister()
	pipelineSettingLister := cluster.Management.Project.PipelineSettings("").Controller().Lister()
	dialer := cluster.Management.Dialer

	engine := &jenkins.Engine{
		UseCache:                   useCache,
		Secrets:                    secrets,
		ManagementSecretLister:     managementSecretLister,
		SourceCodeCredentials:      sourceCodeCredentials,
		SourceCodeCredentialLister: sourceCodeCredentialLister,
		PipelineLister:             pipelineLister,
		PipelineSettingLister:      pipelineSettingLister,
		Dialer:                     dialer,
		ClusterName:                cluster.ClusterName,
	}
	if useCache {
		engine.ServiceLister = cluster.Core.Services("").Controller().Lister()
		engine.PodLister = cluster.Core.Pods("").Controller().Lister()
		engine.SecretLister = secrets.Controller().Lister()
	} else {
		engine.Services = cluster.Core.Services("")
	}
	return engine
}
