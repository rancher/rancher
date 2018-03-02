package pipelineexecution

import (
	"context"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/engine"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/utils"
	"github.com/rancher/rancher/pkg/ticker"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"time"
)

const (
	syncStateInterval = 5 * time.Second
)

//ExecutionStateSyncer is responsible for updating pipeline execution states
//by syncing with the pipeline engine
type ExecutionStateSyncer struct {
	clusterName           string
	clusterPipelineLister v3.ClusterPipelineLister

	pipelineLister          v3.PipelineLister
	pipelines               v3.PipelineInterface
	pipelineExecutionLister v3.PipelineExecutionLister
	pipelineExecutions      v3.PipelineExecutionInterface
	pipelineEngine          engine.PipelineEngine
}

func (s *ExecutionStateSyncer) sync(ctx context.Context, syncInterval time.Duration) {
	for range ticker.Context(ctx, syncInterval) {
		s.syncState()
	}

}

func (s *ExecutionStateSyncer) syncState() {
	if !utils.IsPipelineDeploy(s.clusterPipelineLister, s.clusterName) {
		return
	}

	set := labels.Set(map[string]string{utils.PipelineFinishLabel: "false"})
	executions, err := s.pipelineExecutionLister.List("", set.AsSelector())
	if err != nil {
		logrus.Errorf("Error listing PipelineExecutions - %v", err)
		return
	}
	if len(executions) < 1 {
		return
	}
	if err := s.pipelineEngine.PreCheck(); err != nil {
		//fail to connect engine, mark the remaining executions as failed
		logrus.Errorf("Error get Jenkins engine - %v", err)
		for _, e := range executions {
			e.Status.ExecutionState = utils.StateFail
			if err := s.updateExecutionAndLastRunState(e); err != nil {
				logrus.Errorf("Error update pipeline execution - %v", err)
				return
			}
		}
		return
	}
	for _, e := range executions {
		if e.Status.ExecutionState == utils.StateWaiting || e.Status.ExecutionState == utils.StateBuilding {
			updated, err := s.pipelineEngine.SyncExecution(e)
			if err != nil {
				logrus.Errorf("Error sync pipeline execution - %v", err)
				e.Status.ExecutionState = utils.StateFail
				updated = true
			}
			if updated {
				if err := s.updateExecutionAndLastRunState(e); err != nil {
					logrus.Errorf("Error update pipeline execution - %v", err)
					return
				}
			}
		} else {
			if err := s.updateExecutionAndLastRunState(e); err != nil {
				logrus.Errorf("Error update pipeline execution - %v", err)
				return
			}
		}
	}
	logrus.Debugf("Sync pipeline execution state complete")
}

func (s *ExecutionStateSyncer) updateExecutionAndLastRunState(execution *v3.PipelineExecution) error {
	if execution.Status.ExecutionState != utils.StateWaiting && execution.Status.ExecutionState != utils.StateBuilding {
		execution.Labels[utils.PipelineFinishLabel] = "true"
	}
	if _, err := s.pipelineExecutions.Update(execution); err != nil {
		return err
	}

	//check and update lastrunstate of the pipeline when necessary
	p, err := s.pipelineLister.Get(execution.Spec.Pipeline.Namespace, execution.Spec.Pipeline.Name)
	if apierrors.IsNotFound(err) {
		logrus.Warningf("pipeline of execution '%s' is not found", execution.Name)
		return nil
	} else if err != nil {
		return err
	}

	if p.Status.LastExecutionID == execution.Namespace+":"+execution.Name &&
		p.Status.LastRunState != execution.Status.ExecutionState {
		p.Status.LastRunState = execution.Status.ExecutionState
		if _, err := s.pipelines.Update(p); err != nil {
			return err
		}
	}
	return nil
}
