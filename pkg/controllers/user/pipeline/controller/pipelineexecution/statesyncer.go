package pipelineexecution

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/rancher/pkg/pipeline/engine"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
	v3 "github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
)

// This controller is responsible for updating pipeline execution states
// by syncing with the pipeline engine. It also detect executors' status
// and do the actual run pipeline when they are ready

const (
	syncStateInterval = 5 * time.Second
)

type ExecutionStateSyncer struct {
	clusterName string

	pipelineLister          v3.PipelineLister
	pipelines               v3.PipelineInterface
	pipelineExecutionLister v3.PipelineExecutionLister
	pipelineExecutions      v3.PipelineExecutionInterface
	pipelineEngine          engine.PipelineEngine
}

func (s *ExecutionStateSyncer) sync(ctx context.Context, syncInterval time.Duration) {
	tryTicker := time.NewTicker(syncInterval)

	for {
		select{
		case <-ctx.Done():
			return
		case <-tryTicker.C:
			s.syncState()
		}
	}
}

func (s *ExecutionStateSyncer) syncState() {
	set := labels.Set(map[string]string{utils.PipelineFinishLabel: "false"})
	allExecutions, err := s.pipelineExecutionLister.List("", set.AsSelector())
	if err != nil {
		logrus.Errorf("Error listing PipelineExecutions - %v", err)
		return
	}
	executions := []*v3.PipelineExecution{}
	for _, e := range allExecutions {
		if controller.ObjectInCluster(s.clusterName, e) {
			executions = append(executions, e)
		}
	}
	if len(executions) < 1 {
		return
	}

	for _, execution := range executions {
		if v3.PipelineExecutionConditionInitialized.IsUnknown(execution) {
			s.checkAndRun(execution)
		} else if v3.PipelineExecutionConditionInitialized.IsTrue(execution) {
			e := execution.DeepCopy()
			updated, err := s.pipelineEngine.SyncExecution(e)
			if err != nil {
				logrus.Errorf("got error in syncExecution: %v", err)
				v3.PipelineExecutionConditionBuilt.False(e)
				v3.PipelineExecutionConditionBuilt.ReasonAndMessageFromError(e, err)
				e.Status.ExecutionState = utils.StateFailed
				updated = true
			}
			if updated {
				if err := s.updateExecutionAndLastRunState(e); err != nil {
					logrus.Error(err)
					continue
				}
			}
		} else {
			if err := s.updateExecutionAndLastRunState(execution); err != nil {
				logrus.Errorf("Error update pipeline execution - %v", err)
			}
		}
	}

	logrus.Debugf("Sync pipeline execution state complete")
}

func (s *ExecutionStateSyncer) checkAndRun(execution *v3.PipelineExecution) {
	ready, err := s.pipelineEngine.PreCheck(execution)
	if err != nil {
		e := execution.DeepCopy()
		v3.PipelineExecutionConditionBuilt.False(e)
		v3.PipelineExecutionConditionBuilt.ReasonAndMessageFromError(e, err)
		e.Status.ExecutionState = utils.StateFailed
		if err := s.updateExecutionAndLastRunState(e); err != nil {
			logrus.Error(err)
		}
	}
	if ready {
		e := execution.DeepCopy()
		if err := s.pipelineEngine.RunPipelineExecution(e); err != nil {
			v3.PipelineExecutionConditionProvisioned.False(e)
			v3.PipelineExecutionConditionProvisioned.ReasonAndMessageFromError(e, err)
			e.Status.ExecutionState = utils.StateFailed
			if err := s.updateExecutionAndLastRunState(e); err != nil {
				logrus.Error(err)
			}
			return
		}
		v3.PipelineExecutionConditionInitialized.True(e)
		v3.PipelineExecutionConditionProvisioned.CreateUnknownIfNotExists(e)
		v3.PipelineExecutionConditionProvisioned.Message(e, "Assigning jobs to pipeline engine")
		if err := s.updateExecutionAndLastRunState(e); err != nil {
			logrus.Error(err)
		}
	}
	if v3.PipelineExecutionConditionInitialized.GetMessage(execution) == "" {
		e := execution.DeepCopy()
		v3.PipelineExecutionConditionInitialized.Message(e, "Setting up jenkins. If it is not deployed, this can take a few minutes.")
		if err := s.updateExecutionAndLastRunState(e); err != nil {
			logrus.Error(err)
		}
	}

}

func (s *ExecutionStateSyncer) updateExecutionAndLastRunState(execution *v3.PipelineExecution) error {
	if v3.PipelineExecutionConditionInitialized.IsFalse(execution) || v3.PipelineExecutionConditionProvisioned.IsFalse(execution) ||
		v3.PipelineExecutionConditionBuilt.IsFalse(execution) {
		execution.Labels[utils.PipelineFinishLabel] = "true"

		if execution.Status.Ended == "" {
			execution.Status.Ended = time.Now().Format(time.RFC3339)
		}
	}

	if _, err := s.pipelineExecutions.Update(execution); err != nil {
		return err
	}

	//check and update lastrunstate of the pipeline when necessary
	ns, name := ref.Parse(execution.Spec.PipelineName)
	p, err := s.pipelineLister.Get(ns, name)
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
