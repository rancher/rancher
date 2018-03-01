package pipeline

import (
	"context"
	"errors"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/remote"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/utils"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/satori/uuid"
	"github.com/sirupsen/logrus"
)

//Lifecycle is responsible for watching pipelines and handling webhook management
//in source code repository. It also helps to maintain labels on pipelines.
type Lifecycle struct {
	pipelines                  v3.PipelineInterface
	pipelineLister             v3.PipelineLister
	sourceCodeCredentialLister v3.SourceCodeCredentialLister
}

func Register(ctx context.Context, cluster *config.UserContext) {
	clusterName := cluster.ClusterName
	clusterPipelineLister := cluster.Management.Management.ClusterPipelines("").Controller().Lister()
	pipelines := cluster.Management.Management.Pipelines("")
	pipelineLister := pipelines.Controller().Lister()
	pipelineExecutions := cluster.Management.Management.PipelineExecutions("")
	sourceCodeCredentialLister := cluster.Management.Management.SourceCodeCredentials("").Controller().Lister()

	pipelineLifecycle := &Lifecycle{
		pipelines:                  pipelines,
		pipelineLister:             pipelineLister,
		sourceCodeCredentialLister: sourceCodeCredentialLister,
	}
	s := &CronSyncer{
		clusterName:           clusterName,
		clusterPipelineLister: clusterPipelineLister,
		pipelineLister:        pipelineLister,
		pipelines:             pipelines,
		pipelineExecutions:    pipelineExecutions,
	}

	pipelines.AddClusterScopedLifecycle("pipeline-controller", cluster.ClusterName, pipelineLifecycle)
	go s.sync(ctx, syncInterval)
}

func (l *Lifecycle) Create(obj *v3.Pipeline) (*v3.Pipeline, error) {

	if obj.Status.Token == "" {
		//random token for webhook validation
		obj.Status.Token = uuid.NewV4().String()
	}
	if obj.Spec.TriggerCronExpression != "" {
		obj.Labels = map[string]string{utils.PipelineCronLabel: "true"}
	} else {
		obj.Labels = map[string]string{utils.PipelineCronLabel: "false"}
	}

	if obj.Spec.TriggerWebhook && obj.Status.WebHookID == "" {
		id, err := l.createHook(obj)
		if err != nil {
			return obj, err
		}
		obj.Status.WebHookID = id
	}
	return obj, nil
}

func (l *Lifecycle) Updated(obj *v3.Pipeline) (*v3.Pipeline, error) {
	previous, err := l.pipelineLister.Get(obj.Namespace, obj.Name)
	if err != nil {
		return obj, err
	}
	//handle cron update
	if (obj.Spec.TriggerCronExpression != previous.Spec.TriggerCronExpression) ||
		(obj.Spec.TriggerCronTimezone != previous.Spec.TriggerCronTimezone) {
		//cron trigger changed, reset
		obj.Status.NextStart = ""
	}

	if obj.Spec.TriggerCronExpression != "" {
		obj.Labels = map[string]string{utils.PipelineCronLabel: "true"}
	} else {
		obj.Labels = map[string]string{utils.PipelineCronLabel: "false"}
	}

	//handle webhook
	if previous.Spec.TriggerWebhook && previous.Status.WebHookID != "" && !obj.Spec.TriggerWebhook {
		if err := l.deleteHook(previous); err != nil {
			logrus.Errorf("fail to delete previous set webhook")
		}
	} else if !previous.Spec.TriggerWebhook && obj.Spec.TriggerWebhook && obj.Status.WebHookID == "" {
		id, err := l.createHook(obj)
		if err != nil {
			return obj, err
		}
		obj.Status.WebHookID = id
	}

	return obj, nil
}

func (l *Lifecycle) Remove(obj *v3.Pipeline) (*v3.Pipeline, error) {

	if obj.Status.WebHookID != "" {
		if err := l.deleteHook(obj); err != nil {
			//merely log error to avoid deletion block
			logrus.Errorf("Error delete previous set webhook - %v", err)
			return obj, nil
		}
	}
	return obj, nil
}

func (l *Lifecycle) createHook(obj *v3.Pipeline) (string, error) {
	if len(obj.Spec.Stages) <= 0 || len(obj.Spec.Stages[0].Steps) <= 0 || obj.Spec.Stages[0].Steps[0].SourceCodeConfig == nil {
		return "", errors.New("invalid pipeline, missing sourcecode step")
	}
	credentialName := obj.Spec.Stages[0].Steps[0].SourceCodeConfig.SourceCodeCredentialName
	credential, err := l.sourceCodeCredentialLister.Get("", credentialName)
	if err != nil {
		return "", err
	}
	accessToken := credential.Spec.AccessToken
	kind := credential.Spec.SourceCodeType
	mockConfig := v3.ClusterPipeline{
		Spec: v3.ClusterPipelineSpec{
			GithubConfig: &v3.GithubClusterConfig{},
		},
	}
	remote, err := remote.New(mockConfig, kind)
	if err != nil {
		return "", err
	}

	id, err := remote.CreateHook(obj, accessToken)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (l *Lifecycle) deleteHook(obj *v3.Pipeline) error {
	if len(obj.Spec.Stages) <= 0 || len(obj.Spec.Stages[0].Steps) <= 0 || obj.Spec.Stages[0].Steps[0].SourceCodeConfig == nil {
		return errors.New("invalid pipeline, missing sourcecode step")
	}
	credentialName := obj.Spec.Stages[0].Steps[0].SourceCodeConfig.SourceCodeCredentialName
	credential, err := l.sourceCodeCredentialLister.Get("", credentialName)
	if err != nil {
		return err
	}
	accessToken := credential.Spec.AccessToken
	kind := credential.Spec.SourceCodeType
	mockConfig := v3.ClusterPipeline{
		Spec: v3.ClusterPipelineSpec{
			GithubConfig: &v3.GithubClusterConfig{},
		},
	}
	remote, err := remote.New(mockConfig, kind)
	if err != nil {
		return err
	}

	return remote.DeleteHook(obj, accessToken)
}
