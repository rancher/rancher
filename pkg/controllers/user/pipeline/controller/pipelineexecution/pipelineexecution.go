package pipelineexecution

import (
	"context"
	"fmt"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/engine"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/utils"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//Lifecycle is responsible for initializing logs for pipeline execution
//and calling the run for the execution.
type Lifecycle struct {
	pipelineExecutionLogs v3.PipelineExecutionLogInterface
	pipelineEngine        engine.PipelineEngine
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

	pipelineEngine := engine.New(cluster)
	pipelineExecutionLifecycle := &Lifecycle{
		pipelineExecutionLogs: pipelineExecutionLogs,
		pipelineEngine:        pipelineEngine,
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
	if err := l.pipelineEngine.PreCheck(); err != nil {
		logrus.Errorf("Error get Jenkins engine - %v", err)
		obj.Status.ExecutionState = utils.StateFail
		return obj, nil
	}
	if err := l.pipelineEngine.RunPipeline(&obj.Spec.Pipeline, obj.Spec.TriggeredBy); err != nil {
		logrus.Errorf("Error run pipeline - %v", err)
		obj.Status.ExecutionState = utils.StateFail
		return obj, nil
	}
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
					Labels:    utils.PipelineInprogressLabel,
				},
				Spec: v3.PipelineExecutionLogSpec{
					ProjectName:           pipeline.Spec.ProjectName,
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
