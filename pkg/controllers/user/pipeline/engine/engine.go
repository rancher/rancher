package engine

import (
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/engine/jenkins"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
)

type PipelineEngine interface {
	PreCheck() error
	RunPipelineExecution(execution *v3.PipelineExecution, triggerType string) error
	RerunExecution(execution *v3.PipelineExecution) error
	StopExecution(execution *v3.PipelineExecution) error
	GetStepLog(execution *v3.PipelineExecution, stage int, step int) (string, error)
	SyncExecution(execution *v3.PipelineExecution) (bool, error)
}

func New(cluster *config.UserContext) PipelineEngine {

	serviceLister := cluster.Core.Services("").Controller().Lister()
	secrets := cluster.Core.Secrets("")
	secretLister := secrets.Controller().Lister()
	managementSecretLister := cluster.Management.Core.Secrets("").Controller().Lister()
	sourceCodeCredentialLister := cluster.Management.Management.SourceCodeCredentials("").Controller().Lister()

	dialer := cluster.Management.Dialer

	engine := &jenkins.Engine{
		ServiceLister:              serviceLister,
		Secrets:                    secrets,
		SecretLister:               secretLister,
		ManagementSecretLister:     managementSecretLister,
		SourceCodeCredentialLister: sourceCodeCredentialLister,

		Dialer:      dialer,
		ClusterName: cluster.ClusterName,
	}
	return engine
}
