package drivers

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"github.com/xanzy/go-gitlab"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"strings"
)

const (
	GitlabWebhookHeader = "X-Gitlab-Event"
	gitlabPushEvent     = "Push Hook"
	gitlabMREvent       = "Merge Request Hook"
	gitlabTagEvent      = "Tag Push Hook"
)

type GitlabDriver struct {
	Pipelines          v3.PipelineInterface
	PipelineExecutions v3.PipelineExecutionInterface
}

func (g GitlabDriver) Execute(req *http.Request) (int, error) {
	var signature string
	if signature = req.Header.Get("X-Gitlab-Token"); len(signature) == 0 {
		return http.StatusUnprocessableEntity, errors.New("gitlab webhook missing token")
	}
	event := req.Header.Get(GitlabWebhookHeader)
	if event != gitlabPushEvent && event != gitlabMREvent && event != gitlabTagEvent {
		return http.StatusUnprocessableEntity, fmt.Errorf("not trigger for event:%s", event)
	}

	pipelineID := req.URL.Query().Get("pipelineId")
	ns, id := ref.Parse(pipelineID)
	pipeline, err := g.Pipelines.GetNamespaced(ns, id, metav1.GetOptions{})
	if err != nil {
		return http.StatusInternalServerError, err
	}
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return http.StatusUnprocessableEntity, err
	}
	if pipeline.Status.Token != signature {
		return http.StatusUnprocessableEntity, errors.New("gitlab webhook invalid token")
	}

	if pipeline.Status.PipelineState == "inactive" {
		return http.StatusUnavailableForLegalReasons, errors.New("pipeline is not active")
	}

	if (event == gitlabPushEvent && !pipeline.Spec.TriggerWebhookPush) ||
		(event == gitlabMREvent && !pipeline.Spec.TriggerWebhookPr) ||
		(event == gitlabTagEvent && !pipeline.Spec.TriggerWebhookTag) {
		return http.StatusUnavailableForLegalReasons, fmt.Errorf("trigger for event '%s' is disabled", event)
	}

	if err := utils.ValidPipelineSpec(pipeline.Spec); err != nil {
		return http.StatusInternalServerError, err
	}

	ref, branch, commit := "", "", ""
	envVars := map[string]string{}
	if event == gitlabPushEvent {
		payload := &gitlab.PushEvent{}
		if err := json.Unmarshal(body, payload); err != nil {
			return http.StatusUnprocessableEntity, err
		}
		ref = payload.Ref

		branch = strings.TrimPrefix(payload.Ref, RefsBranchPrefix)
		commit = payload.After

	} else if event == gitlabMREvent {
		payload := &gitlab.MergeEvent{}
		if err := json.Unmarshal(body, payload); err != nil {
			return http.StatusUnprocessableEntity, err
		}
		ref = fmt.Sprintf("refs/merge-requests/%d/head", payload.ObjectAttributes.IID)
		branch = payload.ObjectAttributes.TargetBranch
		commit = payload.ObjectAttributes.LastCommit.ID
	} else if event == gitlabTagEvent {
		payload := &gitlab.TagEvent{}
		if err := json.Unmarshal(body, payload); err != nil {
			return http.StatusUnprocessableEntity, err
		}
		ref = payload.Ref
		tag := strings.TrimPrefix(ref, RefsTagPrefix)
		envVars["CICD_GIT_TAG"] = tag
		commit = payload.After
	}

	if (event != gitlabTagEvent) && !VerifyBranch(pipeline.Spec.Stages[0].Steps[0].SourceCodeConfig, branch) {
		return http.StatusUnprocessableEntity, errors.New("Error Ref is not match")
	}

	logrus.Debugf("receieve github webhook, triggered '%s' on branch '%s'", pipeline.Spec.DisplayName, ref)
	if _, err := utils.GenerateExecution(g.Pipelines, g.PipelineExecutions, pipeline, utils.TriggerTypeWebhook, "", ref, commit, envVars); err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}
