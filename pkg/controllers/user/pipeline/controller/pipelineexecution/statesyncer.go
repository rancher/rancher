package pipelineexecution

import (
	"context"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/engine"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/utils"
	"github.com/rancher/rancher/pkg/ticker"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"time"
)

const (
	syncStateInterval = 10 * time.Second
)

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
		logrus.Debugf("Start sync pipeline execution state")
		s.syncState()
		logrus.Debugf("Sync pipeline execution state complete")
	}

}

func (s *ExecutionStateSyncer) syncState() {
	if !utils.IsPipelineDeploy(s.clusterPipelineLister, s.clusterName) {
		return
	}

	executions, err := s.pipelineExecutionLister.List("", utils.PipelineInprogressLabel.AsSelector())
	if err != nil {
		logrus.Errorf("Error listing PipelineExecutions - %v", err)
		return
	}
	if len(executions) < 1 {
		return
	}
	if err := s.pipelineEngine.PreCheck(); err != nil {
		logrus.Errorf("Error get Jenkins engine - %v", err)
		return
	}
	for _, e := range executions {
		if e.Status.ExecutionState == utils.StateWaiting || e.Status.ExecutionState == utils.StateBuilding {
			updated, err := s.pipelineEngine.SyncExecution(e)
			if err != nil {
				logrus.Errorf("Error sync pipeline execution - %v", err)
				e.Status.ExecutionState = utils.StateFail
				if _, err := s.pipelineExecutions.Update(e); err != nil {
					logrus.Errorf("Error update pipeline execution - %v", err)
					return
				}
			} else if updated {
				if _, err := s.pipelineExecutions.Update(e); err != nil {
					logrus.Errorf("Error update pipeline execution - %v", err)
					return
				}

				//update lastrunstate of the pipeline
				p, err := s.pipelineLister.Get(e.Spec.Pipeline.Namespace, e.Spec.Pipeline.Name)
				if err != nil {
					logrus.Errorf("Error get pipeline - %v", err)
					continue
				}
				if p.Status.LastExecutionID == e.Name {
					p.Status.LastRunState = e.Status.ExecutionState
					if _, err := s.pipelines.Update(p); err != nil {
						logrus.Errorf("Error update pipeline - %v", err)
						return
					}
				}
			}
		} else {
			e.Labels["pipeline.management.cattle.io/finish"] = "true"
			if _, err := s.pipelineExecutions.Update(e); err != nil {
				logrus.Errorf("Error update pipeline execution - %v", err)
				return
			}
		}
	}
}
