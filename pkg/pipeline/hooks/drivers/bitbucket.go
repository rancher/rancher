package drivers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	v3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/pipeline/remote/bitbucketcloud"
	"github.com/rancher/rancher/pkg/pipeline/remote/model"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
)

const (
	BitbucketCloudWebhookHeader  = "X-Hook-UUID"
	bitbucketCloudEventHeader    = "X-Event-Key"
	bitbucketCloudPushEvent      = "repo:push"
	bitbucketCloudPrCreatedEvent = "pullrequest:created"
	bitbucketCloudPrUpdatedEvent = "pullrequest:updated"
	bitbucketCloudStateOpen      = "OPEN"
)

type BitbucketCloudDriver struct {
	PipelineLister             v3.PipelineLister
	PipelineExecutions         v3.PipelineExecutionInterface
	SourceCodeCredentials      v3.SourceCodeCredentialInterface
	SourceCodeCredentialLister v3.SourceCodeCredentialLister
}

func (b BitbucketCloudDriver) Execute(req *http.Request) (int, error) {
	event := req.Header.Get(bitbucketCloudEventHeader)
	if event != bitbucketCloudPushEvent && event != bitbucketCloudPrCreatedEvent && event != bitbucketCloudPrUpdatedEvent {
		return http.StatusUnprocessableEntity, fmt.Errorf("not trigger for event:%s", event)
	}

	pipelineID := req.URL.Query().Get("pipelineId")
	ns, name := ref.Parse(pipelineID)
	pipeline, err := b.PipelineLister.Get(ns, name)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return http.StatusUnprocessableEntity, err
	}

	info := &model.BuildInfo{}
	if event == bitbucketCloudPushEvent {
		info, err = parseBitbucketPushPayload(body)
		if err != nil {
			return http.StatusUnprocessableEntity, err
		}
	} else if event == bitbucketCloudPrCreatedEvent || event == bitbucketCloudPrUpdatedEvent {
		info, err = parseBitbucketPullRequestPayload(body)
		if err != nil {
			return http.StatusUnprocessableEntity, err
		}
	}

	return validateAndGeneratePipelineExecution(b.PipelineExecutions, b.SourceCodeCredentials, b.SourceCodeCredentialLister, info, pipeline)
}

func parseBitbucketPushPayload(raw []byte) (*model.BuildInfo, error) {
	info := &model.BuildInfo{}
	payload := bitbucketcloud.PushEventPayload{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	info.TriggerType = utils.TriggerTypeWebhook

	if len(payload.Push.Changes) > 0 {
		change := payload.Push.Changes[0]
		info.Commit = change.New.Target.Hash
		info.Branch = change.New.Name
		info.Message = change.New.Target.Message
		info.Author = change.New.Target.Author.User.UserName
		info.AvatarURL = change.New.Target.Author.User.Links.Avatar.Href
		info.HTMLLink = change.New.Target.Links.HTML.Href

		switch change.New.Type {
		case "tag", "annotated_tag", "bookmark":
			info.Event = utils.WebhookEventTag
			info.Ref = RefsTagPrefix + change.New.Name
		default:
			info.Event = utils.WebhookEventPush
			info.Ref = RefsBranchPrefix + change.New.Name
		}
	}
	return info, nil
}

func parseBitbucketPullRequestPayload(raw []byte) (*model.BuildInfo, error) {
	info := &model.BuildInfo{}
	payload := bitbucketcloud.PullRequestEventPayload{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}

	if payload.PullRequest.State != bitbucketCloudStateOpen {
		return nil, fmt.Errorf("no trigger for closed pull requests")
	}

	info.TriggerType = utils.TriggerTypeWebhook
	info.Event = utils.WebhookEventPullRequest
	info.RepositoryURL = fmt.Sprintf("https://bitbucket.org/%s", payload.PullRequest.Source.Repository.FullName)
	info.Branch = payload.PullRequest.Destination.Branch.Name
	info.Ref = RefsBranchPrefix + payload.PullRequest.Source.Branch.Name
	info.HTMLLink = payload.PullRequest.Links.HTML.Href
	info.Title = payload.PullRequest.Title
	info.Message = payload.PullRequest.Title
	info.Commit = payload.PullRequest.Source.Commit.Hash
	info.Author = payload.PullRequest.Author.UserName
	info.AvatarURL = payload.PullRequest.Author.Links.Avatar.Href
	return info, nil
}
