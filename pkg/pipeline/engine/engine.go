package engine

import (
	"github.com/rancher/rancher/pkg/pipeline/engine/jenkins"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/config"
)

type PipelineEngine interface {
	PreCheck(execution *v3.PipelineExecution) (bool, error)
	RunPipelineExecution(execution *v3.PipelineExecution) error
	RerunExecution(execution *v3.PipelineExecution) error
	StopExecution(execution *v3.PipelineExecution) error
	GetStepLog(execution *v3.PipelineExecution, stage int, step int) (string, error)
	SyncExecution(execution *v3.PipelineExecution) (bool, error)
}

func New(cluster *config.UserContext) PipelineEngine {
	serviceLister := cluster.Core.Services("").Controller().Lister()
	podLister := cluster.Core.Pods("").Controller().Lister()
	secrets := cluster.Core.Secrets("")
	secretLister := secrets.Controller().Lister()
	managementSecretLister := cluster.Management.Core.Secrets("").Controller().Lister()
	sourceCodeCredentialLister := cluster.Management.Project.SourceCodeCredentials("").Controller().Lister()
	pipelineLister := cluster.Management.Project.Pipelines("").Controller().Lister()
	dialer := cluster.Management.Dialer

	engine := &jenkins.Engine{
		ServiceLister:              serviceLister,
		PodLister:                  podLister,
		Secrets:                    secrets,
		SecretLister:               secretLister,
		ManagementSecretLister:     managementSecretLister,
		SourceCodeCredentialLister: sourceCodeCredentialLister,
		PipelineLister:             pipelineLister,

		Dialer:      dialer,
		ClusterName: cluster.ClusterName,
	}
	return engine
}
