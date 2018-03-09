package drivers

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/utils"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"strings"
)

const GithubWebhookHeader = "X-GitHub-Event"

type GithubDriver struct {
	Pipelines          v3.PipelineInterface
	PipelineExecutions v3.PipelineExecutionInterface
}

func (g GithubDriver) Execute(req *http.Request) (int, error) {
	var signature string
	if signature = req.Header.Get("X-Hub-Signature"); len(signature) == 0 {
		return http.StatusUnprocessableEntity, errors.New("github webhook missing signature")
	}
	event := req.Header.Get(GithubWebhookHeader)
	if event == "ping" {
		return http.StatusOK, nil
	} else if event != "push" && event != "pull_request" {
		return http.StatusUnprocessableEntity, fmt.Errorf("not trigger for event:%s", event)
	}

	pipelineID := req.URL.Query().Get("pipelineId")
	parts := strings.Split(pipelineID, ":")
	if len(parts) < 0 {
		return http.StatusUnprocessableEntity, fmt.Errorf("pipeline id '%s' is not valid", pipelineID)
	}
	ns := parts[0]
	id := parts[1]
	pipeline, err := g.Pipelines.GetNamespaced(ns, id, metav1.GetOptions{})
	if err != nil {
		return http.StatusInternalServerError, err
	}
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return http.StatusUnprocessableEntity, err
	}
	if match := VerifyGithubWebhookSignature([]byte(pipeline.Status.Token), signature, body); !match {
		return http.StatusUnprocessableEntity, errors.New("github webhook invalid signature")
	}

	if pipeline.Status.PipelineState == "inactive" {
		return http.StatusUnavailableForLegalReasons, errors.New("Pipeline is not active")
	}

	if (event == "push" && !pipeline.Spec.TriggerWebhookPush) ||
		(event == "pull_request" && !pipeline.Spec.TriggerWebhookPr) {
		return http.StatusUnavailableForLegalReasons, fmt.Errorf("trigger for event '%s' is disabled", event)
	}

	if err := utils.ValidPipelineSpec(pipeline.Spec); err != nil {
		return http.StatusInternalServerError, err
	}

	ref, branch, commit := "", "", ""
	if event == "push" {
		payload := &github.PushEvent{}
		if err := json.Unmarshal(body, payload); err != nil {
			return http.StatusUnprocessableEntity, err
		}
		ref = payload.GetRef()
		branch = strings.TrimLeft(payload.GetRef(), "refs/heads/")
		commit = payload.HeadCommit.GetID()
	} else if event == "pull_request" {
		payload := &github.PullRequestEvent{}
		if err := json.Unmarshal(body, payload); err != nil {
			return http.StatusUnprocessableEntity, err
		}
		ref = fmt.Sprintf("refs/pull/%d/head", payload.PullRequest.GetNumber())
		branch = payload.PullRequest.Base.GetRef()
		commit = payload.PullRequest.Head.GetSHA()
	}

	if !VerifyBranch(pipeline.Spec.Stages[0].Steps[0].SourceCodeConfig, branch) {
		return http.StatusUnprocessableEntity, errors.New("Error Ref is not match")
	}

	logrus.Debugf("receieve github webhook, triggered '%s' on branch '%s'", pipeline.Spec.DisplayName, ref)
	if _, err := utils.GenerateExecution(g.Pipelines, g.PipelineExecutions, pipeline, utils.TriggerTypeWebhook, "", ref, commit); err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}

func VerifyGithubWebhookSignature(secret []byte, signature string, body []byte) bool {

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

func VerifyBranch(config *v3.SourceCodeConfig, branch string) bool {
	if config.BranchCondition == "all" {
		return true
	} else if config.BranchCondition == "except" {
		if config.Branch == branch {
			return false
		}
		return true
	} else if config.Branch == branch {
		return true

	}

	return false
}
