package drivers

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	v3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/pipeline/remote/model"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
)

const (
	GithubWebhookHeader   = "X-GitHub-Event"
	githubSignatureHeader = "X-Hub-Signature"
	githubPingEvent       = "ping"
	githubPushEvent       = "push"
	githubPREvent         = "pull_request"

	githubActionOpen = "opened"
	githubActionSync = "synchronize"

	githubStateOpen = "open"
)

type GithubDriver struct {
	PipelineLister             v3.PipelineLister
	PipelineExecutions         v3.PipelineExecutionInterface
	SourceCodeCredentials      v3.SourceCodeCredentialInterface
	SourceCodeCredentialLister v3.SourceCodeCredentialLister
}

func (g GithubDriver) Execute(req *http.Request) (int, error) {
	var signature string
	if signature = req.Header.Get(githubSignatureHeader); len(signature) == 0 {
		return http.StatusUnprocessableEntity, errors.New("github webhook missing signature")
	}
	event := req.Header.Get(GithubWebhookHeader)
	if event == githubPingEvent {
		return http.StatusOK, nil
	} else if event != githubPushEvent && event != githubPREvent {
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
	if match := verifyGithubWebhookSignature([]byte(pipeline.Status.Token), signature, body); !match {
		return http.StatusUnprocessableEntity, errors.New("github webhook invalid signature")
	}

	if pipeline.Status.PipelineState == "inactive" {
		return http.StatusUnavailableForLegalReasons, errors.New("Pipeline is not active")
	}

	info := &model.BuildInfo{}
	if event == githubPushEvent {
		info, err = parsePushPayload(body)
		if err != nil {
			return http.StatusUnprocessableEntity, err
		}
	} else if event == githubPREvent {
		info, err = parsePullRequestPayload(body)
		if err != nil {
			return http.StatusUnprocessableEntity, err
		}
	}

	return validateAndGeneratePipelineExecution(g.PipelineExecutions, g.SourceCodeCredentials, g.SourceCodeCredentialLister, info, pipeline)
}

func verifyGithubWebhookSignature(secret []byte, signature string, body []byte) bool {
	const signaturePrefix = "sha1="
	const signatureLength = 45 // len(SignaturePrefix) + len(hex(sha1))

	if len(signature) != signatureLength || !strings.HasPrefix(signature, signaturePrefix) {
		return false
	}

	actual := make([]byte, 20)
	hex.Decode(actual, []byte(signature[5:]))
	computed := hmac.New(sha1.New, secret)
	computed.Write(body)

	return hmac.Equal([]byte(computed.Sum(nil)), actual)
}

func parsePushPayload(raw []byte) (*model.BuildInfo, error) {
	info := &model.BuildInfo{}
	payload := &github.PushEvent{}
	if err := json.Unmarshal(raw, payload); err != nil {
		return nil, err
	}
	info.TriggerType = utils.TriggerTypeWebhook
	info.Commit = payload.HeadCommit.GetID()
	info.Ref = payload.GetRef()
	info.HTMLLink = payload.HeadCommit.GetURL()
	info.Message = payload.HeadCommit.GetMessage()
	info.Email = payload.HeadCommit.Author.GetEmail()
	info.AvatarURL = payload.Sender.GetAvatarURL()
	info.Author = payload.Sender.GetLogin()
	info.Sender = payload.Sender.GetLogin()

	ref := payload.GetRef()
	if strings.HasPrefix(ref, RefsTagPrefix) {
		//git tag is triggered as a push event
		info.Event = utils.WebhookEventTag
		info.Branch = strings.TrimPrefix(ref, RefsTagPrefix)

	} else {
		info.Event = utils.WebhookEventPush
		info.Branch = strings.TrimPrefix(payload.GetRef(), RefsBranchPrefix)
	}
	return info, nil
}

func parsePullRequestPayload(raw []byte) (*model.BuildInfo, error) {
	info := &model.BuildInfo{}
	payload := &github.PullRequestEvent{}
	if err := json.Unmarshal(raw, payload); err != nil {
		return nil, err
	}

	action := payload.GetAction()
	if action != githubActionOpen && action != githubActionSync {
		return nil, fmt.Errorf("no trigger for %s action", action)
	}
	if payload.PullRequest.GetState() != githubStateOpen {
		return nil, fmt.Errorf("no trigger for closed pull requests")
	}

	info.TriggerType = utils.TriggerTypeWebhook
	info.Event = utils.WebhookEventPullRequest
	info.Branch = payload.PullRequest.Base.GetRef()
	info.Ref = fmt.Sprintf("refs/pull/%d/head", payload.PullRequest.GetNumber())
	info.HTMLLink = payload.PullRequest.GetHTMLURL()
	info.Title = payload.PullRequest.GetTitle()
	info.Message = payload.PullRequest.GetTitle()
	info.Commit = payload.PullRequest.Head.GetSHA()
	info.Author = payload.PullRequest.User.GetLogin()
	info.AvatarURL = payload.PullRequest.User.GetAvatarURL()
	info.Email = payload.PullRequest.User.GetEmail()
	info.Sender = payload.Sender.GetLogin()
	return info, nil
}
