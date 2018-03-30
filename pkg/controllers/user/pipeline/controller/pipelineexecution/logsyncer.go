package pipelineexecution

import (
	"context"
	"fmt"
	"github.com/rancher/norman/controller"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/engine"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/ticker"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
	"time"
)

const (
	syncLogInterval = 5 * time.Second
)

//ExecutionLogSyncer is responsible for updating pipeline execution logs that are in building state
//by syncing with the pipeline engine
type ExecutionLogSyncer struct {
	clusterName           string
	clusterPipelineLister v3.ClusterPipelineLister

	pipelineExecutionLister    v3.PipelineExecutionLister
	pipelineExecutionLogLister v3.PipelineExecutionLogLister
	pipelineExecutionLogs      v3.PipelineExecutionLogInterface
	pipelineEngine             engine.PipelineEngine
}

func (s *ExecutionLogSyncer) sync(ctx context.Context, syncInterval time.Duration) {
	for range ticker.Context(ctx, syncInterval) {
		s.syncLogs()
	}

}

func (s *ExecutionLogSyncer) syncLogs() {
	if !utils.IsPipelineDeploy(s.clusterPipelineLister, s.clusterName) {
		return
	}

	set := labels.Set(map[string]string{utils.PipelineFinishLabel: "false"})
	allLogs, err := s.pipelineExecutionLogLister.List("", set.AsSelector())
	if err != nil {
		logrus.Errorf("Error listing PipelineExecutionLogs - %v", err)
		return
	}
	Logs := []*v3.PipelineExecutionLog{}
	for _, log := range allLogs {
		if controller.ObjectInCluster(s.clusterName, log) {
			Logs = append(Logs, log)
		}
	}
	if len(Logs) < 1 {
		return
	}
	if err := s.pipelineEngine.PreCheck(); err != nil {
		logrus.Errorf("Error get Jenkins engine - %v", err)
		for _, log := range Logs {
			log.Spec.Message += fmt.Sprintf("Error get Jenkins engine - %v", err)
			if err := s.finishExecutionLog(log); err != nil {
				logrus.Errorf("Error update pipeline execution log - %v", err)
				return
			}
		}
		return
	}
	for _, log := range Logs {

		ns, name := ref.Parse(log.Spec.PipelineExecutionName)
		execution, err := s.pipelineExecutionLister.Get(ns, name)
		if err != nil {
			logrus.Errorf("Error get pipeline execution - %v", err)
			log.Spec.Message += fmt.Sprintf("\nError get pipeline execution - %v", err)
			if err := s.finishExecutionLog(log); err != nil {
				logrus.Errorf("Error update pipeline execution log - %v", err)
				return
			}
			continue
		}
		//get log if the step started
		if execution.Status.Stages[log.Spec.Stage].Steps[log.Spec.Step].State == utils.StateWaiting {
			continue
		}
		logText, err := s.pipelineEngine.GetStepLog(execution, log.Spec.Stage, log.Spec.Step)
		if err != nil {
			logrus.Errorf("Error get pipeline execution log - %v", err)
			log.Spec.Message += fmt.Sprintf("\nError get pipeline execution log - %v", err)
			if err := s.finishExecutionLog(log); err != nil {
				logrus.Errorf("Error update pipeline execution log - %v", err)
				return
			}
			continue
		}

		stepState := execution.Status.Stages[log.Spec.Stage].Steps[log.Spec.Step].State
		if stepState != utils.StateWaiting && stepState != utils.StateBuilding {
			log.Labels[utils.PipelineFinishLabel] = "true"
		}

		if log.Spec.Message == logText {
			//only do update on changes
			continue
		}
		log.Spec.Message = logText
		if _, err := s.pipelineExecutionLogs.Update(log); err != nil {
			logrus.Errorf("Error update pipeline execution log - %v", err)
			return
		}
	}
	logrus.Debugf("Sync pipeline execution log complete")
}

func (s *ExecutionLogSyncer) finishExecutionLog(log *v3.PipelineExecutionLog) error {
	log.Labels[utils.PipelineFinishLabel] = "true"
	_, err := s.pipelineExecutionLogs.Update(log)
	return err

}
