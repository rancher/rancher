package drivers

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/pipeline/providers"
	"github.com/rancher/rancher/pkg/pipeline/remote/model"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/xanzy/go-gitlab"
	"io/ioutil"
	"net/http"
	"strings"
)

const (
	GitlabWebhookHeader = "X-Gitlab-Event"
	GitlabTokenHeader   = "X-Gitlab-Token"
	gitlabPushEvent     = "Push Hook"
	gitlabMREvent       = "Merge Request Hook"
	gitlabTagEvent      = "Tag Push Hook"
)

type GitlabDriver struct {
	PipelineLister             v3.PipelineLister
	PipelineExecutions         v3.PipelineExecutionInterface
	SourceCodeCredentialLister v3.SourceCodeCredentialLister
}

func (g GitlabDriver) Execute(req *http.Request) (int, error) {
	var signature string
	if signature = req.Header.Get(GitlabTokenHeader); len(signature) == 0 {
		return http.StatusUnprocessableEntity, errors.New("gitlab webhook missing token")
	}
	event := req.Header.Get(GitlabWebhookHeader)
	if event != gitlabPushEvent && event != gitlabMREvent && event != gitlabTagEvent {
		return http.StatusUnprocessableEntity, fmt.Errorf("not trigger for event:%s", event)
	}

	pipelineID := req.URL.Query().Get("pipelineId")
	ns, name := ref.Parse(pipelineID)
	pipeline, err := g.PipelineLister.Get(ns, name)
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

	info := &model.BuildInfo{}
	if event == gitlabPushEvent {
		info, err = gitlabParsePushPayload(body)
		if err != nil {
			return http.StatusUnprocessableEntity, err
		}
	} else if event == gitlabMREvent {
		info, err = gitlabParseMergeRequestPayload(body)
		if err != nil {
			return http.StatusUnprocessableEntity, err
		}
	} else if event == gitlabTagEvent {
		info, err = gitlabParseTagPayload(body)
		if err != nil {
			return http.StatusUnprocessableEntity, err
		}
	}

	pipelineConfig, err := providers.GetPipelineConfigByBranch(g.SourceCodeCredentialLister, pipeline, info.Branch)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if pipelineConfig == nil {
		//no pipeline config to run
		return http.StatusOK, nil
	}

	if !utils.Match(pipelineConfig.Branch, info.Branch) {
		return http.StatusUnavailableForLegalReasons, fmt.Errorf("skipped branch '%s'", info.Branch)
	}

	if _, err := utils.GenerateExecution(g.PipelineExecutions, pipeline, pipelineConfig, info); err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusOK, nil
}

func gitlabParsePushPayload(raw []byte) (*model.BuildInfo, error) {
	payload := &gitlab.PushEvent{}
	if err := json.Unmarshal(raw, payload); err != nil {
		return nil, err
	}

	info := &model.BuildInfo{}
	if err := json.Unmarshal(raw, payload); err != nil {
		return nil, err
	}
	info.TriggerType = utils.TriggerTypeWebhook
	info.Event = utils.WebhookEventPush
	info.Commit = payload.After
	info.Branch = strings.TrimPrefix(payload.Ref, RefsBranchPrefix)
	info.Ref = payload.Ref
	info.HTMLLink = payload.Repository.HTTPURL
	lastCommit := payload.Commits[len(payload.Commits)-1]
	info.Message = lastCommit.Message
	info.Email = payload.UserEmail
	info.AvatarURL = payload.UserAvatar
	info.Author = payload.UserName
	info.Sender = payload.UserName

	return info, nil
}

func gitlabParseMergeRequestPayload(raw []byte) (*model.BuildInfo, error) {
	payload := &gitlab.MergeEvent{}
	if err := json.Unmarshal(raw, payload); err != nil {
		return nil, err
	}

	info := &model.BuildInfo{}
	if err := json.Unmarshal(raw, payload); err != nil {
		return nil, err
	}

	info.TriggerType = utils.TriggerTypeWebhook
	info.Event = utils.WebhookEventPullRequest
	info.Branch = payload.ObjectAttributes.TargetBranch
	info.Ref = fmt.Sprintf("refs/merge-requests/%d/head", payload.ObjectAttributes.IID)
	info.HTMLLink = payload.ObjectAttributes.URL
	info.Title = payload.ObjectAttributes.LastCommit.Message
	info.Message = payload.ObjectAttributes.LastCommit.Message
	info.Commit = payload.ObjectAttributes.LastCommit.ID
	info.Author = payload.User.Name
	info.AvatarURL = payload.User.AvatarURL
	info.Email = payload.User.Email
	info.Sender = payload.User.Name

	return info, nil
}

func gitlabParseTagPayload(raw []byte) (*model.BuildInfo, error) {
	info := &model.BuildInfo{}
	payload := &gitlab.TagEvent{}
	if err := json.Unmarshal(raw, payload); err != nil {
		return nil, err
	}

	info.TriggerType = utils.TriggerTypeWebhook
	info.Event = utils.WebhookEventTag
	info.Ref = payload.Ref
	tag := strings.TrimPrefix(payload.Ref, RefsTagPrefix)
	info.Message = "tag " + tag
	info.Branch = tag
	info.Commit = payload.After
	info.Author = payload.UserName
	info.AvatarURL = payload.UserAvatar

	return info, nil
}
