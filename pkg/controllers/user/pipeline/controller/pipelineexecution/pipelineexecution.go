package pipelineexecution

import (
	"context"
	"fmt"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/engine"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/remote"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//Lifecycle is responsible for initializing logs for pipeline execution
//and calling the run for the execution.
type Lifecycle struct {
	pipelineExecutionLogs      v3.PipelineExecutionLogInterface
	pipelineEngine             engine.PipelineEngine
	clusterPipelineLister      v3.ClusterPipelineLister
	sourceCodeCredentialLister v3.SourceCodeCredentialLister
}

func Register(ctx context.Context, cluster *config.UserContext) {
	clusterName := cluster.ClusterName
	clusterPipelineLister := cluster.Management.Management.ClusterPipelines("").Controller().Lister()

	pipelines := cluster.Management.Management.Pipelines("")
	pipelineLister := pipelines.Controller().Lister()
	pipelineExecutions := cluster.Management.Management.PipelineExecutions("")
	pipelineExecutionLister := pipelineExecutions.Controller().Lister()
	pipelineExecutionLogs := cluster.Management.Management.PipelineExecutionLogs("")
	pipelineExecutionLogLister := pipelineExecutionLogs.Controller().Lister()
	sourceCodeCredentialLister := cluster.Management.Management.SourceCodeCredentials("").Controller().Lister()

	pipelineEngine := engine.New(cluster)
	pipelineExecutionLifecycle := &Lifecycle{
		pipelineExecutionLogs:      pipelineExecutionLogs,
		pipelineEngine:             pipelineEngine,
		clusterPipelineLister:      clusterPipelineLister,
		sourceCodeCredentialLister: sourceCodeCredentialLister,
	}
	stateSyncer := &ExecutionStateSyncer{
		clusterName:           clusterName,
		clusterPipelineLister: clusterPipelineLister,

		pipelineLister:          pipelineLister,
		pipelines:               pipelines,
		pipelineExecutionLister: pipelineExecutionLister,
		pipelineExecutions:      pipelineExecutions,
		pipelineEngine:          pipelineEngine,
	}
	logSyncer := &ExecutionLogSyncer{
		clusterName:           clusterName,
		clusterPipelineLister: clusterPipelineLister,

		pipelineExecutionLister:    pipelineExecutionLister,
		pipelineExecutionLogLister: pipelineExecutionLogLister,
		pipelineExecutionLogs:      pipelineExecutionLogs,
		pipelineEngine:             pipelineEngine,
	}

	pipelineExecutions.AddClusterScopedLifecycle(pipelineExecutionLifecycle.GetName(), cluster.ClusterName, pipelineExecutionLifecycle)

	go stateSyncer.sync(ctx, syncStateInterval)
	go logSyncer.sync(ctx, syncLogInterval)

}

func (l *Lifecycle) Create(obj *v3.PipelineExecution) (*v3.PipelineExecution, error) {

	if obj.Status.ExecutionState != utils.StateWaiting {
		return obj, nil
	}
	if err := l.initLogs(obj); err != nil {
		return obj, err
	}

	obj.Status.ExecutionState = utils.StateBuilding
	if err := l.pipelineEngine.PreCheck(); err != nil {
		logrus.Errorf("Error get Jenkins engine - %v", err)
		obj.Status.ExecutionState = utils.StateFail
		v3.PipelineExecutionConditionCompleted.Unknown(obj)
		v3.PipelineExecutionConditionCompleted.ReasonAndMessageFromError(obj, err)
		return obj, nil
	}

	//fetch commit info before running the pipeline execution
	if obj.Status.Commit == "" {
		commit, err := l.getHeadCommit(obj)
		if err != nil {
			return obj, err
		}
		obj.Status.Commit = commit
	}

	if err := l.pipelineEngine.RunPipelineExecution(obj, obj.Spec.TriggeredBy); err != nil {
		logrus.Errorf("Error run pipeline - %v", err)
		obj.Status.ExecutionState = utils.StateFail
		v3.PipelineExecutionConditionCompleted.Unknown(obj)
		v3.PipelineExecutionConditionCompleted.ReasonAndMessageFromError(obj, err)
		return obj, nil
	}

	v3.PipelineExecutionConditonProvisioned.CreateUnknownIfNotExists(obj)
	v3.PipelineExecutionConditonProvisioned.Message(obj, "Assigning jobs to pipeline engine")

	return obj, nil
}

func (l *Lifecycle) Updated(obj *v3.PipelineExecution) (*v3.PipelineExecution, error) {
	return obj, nil
}

func (l *Lifecycle) Remove(obj *v3.PipelineExecution) (*v3.PipelineExecution, error) {
	return obj, nil
}

func (l *Lifecycle) GetName() string {
	return "pipeline-execution-controller"
}

func (l *Lifecycle) initLogs(obj *v3.PipelineExecution) error {

	pipeline := obj.Spec.Pipeline
	//create log entries
	for j, stage := range pipeline.Spec.Stages {
		for k := range stage.Steps {
			log := &v3.PipelineExecutionLog{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-%d-%d", obj.Name, j, k),
					Namespace: obj.Namespace,
					Labels:    map[string]string{utils.PipelineFinishLabel: "false"},
				},
				ProjectName: pipeline.ProjectName,
				Spec: v3.PipelineExecutionLogSpec{
					PipelineExecutionName: obj.Namespace + ":" + obj.Name,
					Stage: j,
					Step:  k,
				},
			}
			if _, err := l.pipelineExecutionLogs.Create(log); err != nil {
				return err
			}
		}
	}
	return nil
}

func (l *Lifecycle) getHeadCommit(execution *v3.PipelineExecution) (string, error) {
	pipeline := execution.Spec.Pipeline
	if err := utils.ValidPipelineSpec(pipeline.Spec); err != nil {
		return "", err
	}
	sourceCodeConfig := pipeline.Spec.Stages[0].Steps[0].SourceCodeConfig
	clusterName, _ := ref.Parse(pipeline.ProjectName)

	clusterPipeline, err := l.clusterPipelineLister.Get(clusterName, clusterName)
	if err != nil {
		return "", err
	}
	sourceCodeType := ""
	if clusterPipeline.Spec.GithubConfig != nil {
		sourceCodeType = "github"
	}

	sourceCodeCredentialID := sourceCodeConfig.SourceCodeCredentialName
	url := sourceCodeConfig.URL
	branch := sourceCodeConfig.Branch
	ns, name := ref.Parse(sourceCodeCredentialID)
	var credential *v3.SourceCodeCredential
	if sourceCodeCredentialID != "" {
		credential, err = l.sourceCodeCredentialLister.Get(ns, name)
		if err != nil {
			return "", err
		}
	}

	client, err := remote.New(*clusterPipeline, sourceCodeType)
	if err != nil {
		return "", err
	}

	return client.GetHeadCommit(url, branch, credential)
}
