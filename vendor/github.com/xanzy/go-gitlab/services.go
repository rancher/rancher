//
// Copyright 2017, Sander van Harmelen
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package gitlab

import (
	"fmt"
	"net/url"
	"time"
)

// ServicesService handles communication with the services related methods of
// the GitLab API.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/services.html
type ServicesService struct {
	client *Client
}

// Service represents a GitLab service.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/services.html
type Service struct {
	ID                       int        `json:"id"`
	Title                    string     `json:"title"`
	CreatedAt                *time.Time `json:"created_at"`
	UpdatedAt                *time.Time `json:"updated_at"`
	Active                   bool       `json:"active"`
	PushEvents               bool       `json:"push_events"`
	IssuesEvents             bool       `json:"issues_events"`
	ConfidentialIssuesEvents bool       `json:"confidential_issues_events"`
	MergeRequestsEvents      bool       `json:"merge_requests_events"`
	TagPushEvents            bool       `json:"tag_push_events"`
	NoteEvents               bool       `json:"note_events"`
	PipelineEvents           bool       `json:"pipeline_events"`
	JobEvents                bool       `json:"job_events"`
}

// SetGitLabCIServiceOptions represents the available SetGitLabCIService()
// options.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#edit-gitlab-ci-service
type SetGitLabCIServiceOptions struct {
	Token      *string `url:"token,omitempty" json:"token,omitempty"`
	ProjectURL *string `url:"project_url,omitempty" json:"project_url,omitempty"`
}

// SetGitLabCIService sets GitLab CI service for a project.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#edit-gitlab-ci-service
func (s *ServicesService) SetGitLabCIService(pid interface{}, opt *SetGitLabCIServiceOptions, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/services/gitlab-ci", url.QueryEscape(project))

	req, err := s.client.NewRequest("PUT", u, opt, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// DeleteGitLabCIService deletes GitLab CI service settings for a project.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#delete-gitlab-ci-service
func (s *ServicesService) DeleteGitLabCIService(pid interface{}, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/services/gitlab-ci", url.QueryEscape(project))

	req, err := s.client.NewRequest("DELETE", u, nil, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// SetHipChatServiceOptions represents the available SetHipChatService()
// options.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#edit-hipchat-service
type SetHipChatServiceOptions struct {
	Token *string `url:"token,omitempty" json:"token,omitempty" `
	Room  *string `url:"room,omitempty" json:"room,omitempty"`
}

// SetHipChatService sets HipChat service for a project
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#edit-hipchat-service
func (s *ServicesService) SetHipChatService(pid interface{}, opt *SetHipChatServiceOptions, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/services/hipchat", url.QueryEscape(project))

	req, err := s.client.NewRequest("PUT", u, opt, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// DeleteHipChatService deletes HipChat service for project.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#delete-hipchat-service
func (s *ServicesService) DeleteHipChatService(pid interface{}, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/services/hipchat", url.QueryEscape(project))

	req, err := s.client.NewRequest("DELETE", u, nil, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// DroneCIService represents Drone CI service settings.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#drone-ci
type DroneCIService struct {
	Service
	Properties *DroneCIServiceProperties `json:"properties"`
}

// DroneCIServiceProperties represents Drone CI specific properties.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#drone-ci
type DroneCIServiceProperties struct {
	Token                 string `json:"token"`
	DroneURL              string `json:"drone_url"`
	EnableSSLVerification bool   `json:"enable_ssl_verification"`
}

// GetDroneCIService gets Drone CI service settings for a project.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#get-drone-ci-service-settings
func (s *ServicesService) GetDroneCIService(pid interface{}, options ...OptionFunc) (*DroneCIService, *Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("projects/%s/services/drone-ci", url.QueryEscape(project))

	req, err := s.client.NewRequest("GET", u, nil, options)
	if err != nil {
		return nil, nil, err
	}

	svc := new(DroneCIService)
	resp, err := s.client.Do(req, svc)
	if err != nil {
		return nil, resp, err
	}

	return svc, resp, err
}

// SetDroneCIServiceOptions represents the available SetDroneCIService()
// options.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#createedit-drone-ci-service
type SetDroneCIServiceOptions struct {
	Token                 *string `url:"token" json:"token" `
	DroneURL              *string `url:"drone_url" json:"drone_url"`
	EnableSSLVerification *bool   `url:"enable_ssl_verification,omitempty" json:"enable_ssl_verification,omitempty"`
}

// SetDroneCIService sets Drone CI service for a project.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#createedit-drone-ci-service
func (s *ServicesService) SetDroneCIService(pid interface{}, opt *SetDroneCIServiceOptions, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/services/drone-ci", url.QueryEscape(project))

	req, err := s.client.NewRequest("PUT", u, opt, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// DeleteDroneCIService deletes Drone CI service settings for a project.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#delete-drone-ci-service
func (s *ServicesService) DeleteDroneCIService(pid interface{}, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/services/drone-ci", url.QueryEscape(project))

	req, err := s.client.NewRequest("DELETE", u, nil, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// SlackService represents Slack service settings.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#slack
type SlackService struct {
	Service
	Properties *SlackServiceProperties `json:"properties"`
}

// SlackServiceProperties represents Slack specific properties.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#slack
type SlackServiceProperties struct {
	NotifyOnlyBrokenPipelines bool `json:"notify_only_broken_pipelines"`
}

// GetSlackService gets Slack service settings for a project.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#get-slack-service-settings
func (s *ServicesService) GetSlackService(pid interface{}, options ...OptionFunc) (*SlackService, *Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("projects/%s/services/slack", url.QueryEscape(project))

	req, err := s.client.NewRequest("GET", u, nil, options)
	if err != nil {
		return nil, nil, err
	}

	svc := new(SlackService)
	resp, err := s.client.Do(req, svc)
	if err != nil {
		return nil, resp, err
	}

	return svc, resp, err
}

// SetSlackServiceOptions represents the available SetSlackService()
// options.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#edit-slack-service
type SetSlackServiceOptions struct {
	WebHook  *string `url:"webhook,omitempty" json:"webhook,omitempty" `
	Username *string `url:"username,omitempty" json:"username,omitempty" `
	Channel  *string `url:"channel,omitempty" json:"channel,omitempty"`
}

// SetSlackService sets Slack service for a project
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#edit-slack-service
func (s *ServicesService) SetSlackService(pid interface{}, opt *SetSlackServiceOptions, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/services/slack", url.QueryEscape(project))

	req, err := s.client.NewRequest("PUT", u, opt, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// DeleteSlackService deletes Slack service for project.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#delete-slack-service
func (s *ServicesService) DeleteSlackService(pid interface{}, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/services/slack", url.QueryEscape(project))

	req, err := s.client.NewRequest("DELETE", u, nil, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// JiraService represents Jira service settings.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#jira
type JiraService struct {
	Service
	Properties *JiraServiceProperties `json:"properties"`
}

// JiraServiceProperties represents Jira specific properties.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#jira
type JiraServiceProperties struct {
	URL                   *string `url:"url,omitempty" json:"url,omitempty"`
	ProjectKey            *string `url:"project_key,omitempty" json:"project_key,omitempty" `
	Username              *string `url:"username,omitempty" json:"username,omitempty" `
	Password              *string `url:"password,omitempty" json:"password,omitempty" `
	JiraIssueTransitionID *string `url:"jira_issue_transition_id,omitempty" json:"jira_issue_transition_id,omitempty"`
}

// GetJiraService gets Jira service settings for a project.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#get-jira-service-settings
func (s *ServicesService) GetJiraService(pid interface{}, options ...OptionFunc) (*JiraService, *Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("projects/%s/services/jira", url.QueryEscape(project))

	req, err := s.client.NewRequest("GET", u, nil, options)
	if err != nil {
		return nil, nil, err
	}

	svc := new(JiraService)
	resp, err := s.client.Do(req, svc)
	if err != nil {
		return nil, resp, err
	}

	return svc, resp, err
}

// SetJiraServiceOptions represents the available SetJiraService()
// options.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#edit-jira-service
type SetJiraServiceOptions JiraServiceProperties

// SetJiraService sets Jira service for a project
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#edit-jira-service
func (s *ServicesService) SetJiraService(pid interface{}, opt *SetJiraServiceOptions, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/services/jira", url.QueryEscape(project))

	req, err := s.client.NewRequest("PUT", u, opt, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// DeleteJiraService deletes Jira service for project.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#delete-jira-service
func (s *ServicesService) DeleteJiraService(pid interface{}, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/services/jira", url.QueryEscape(project))

	req, err := s.client.NewRequest("DELETE", u, nil, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// JenkinsCIService represents Jenkins CI service settings.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/services.html#jenkins-ci
type JenkinsCIService struct {
	Service
	Properties *JenkinsCIServiceProperties `json:"properties"`
}

// JenkinsCIServiceProperties represents Jenkins CI specific properties.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/services.html#jenkins-ci
type JenkinsCIServiceProperties struct {
	URL         *string `url:"jenkins_url,omitempty" json:"jenkins_url,omitempty"`
	ProjectName *string `url:"project_name,omitempty" json:"project_name,omitempty"`
	Username    *string `url:"username,omitempty" json:"username,omitempty"`
}

// GetJenkinsCIService gets Jenkins CI service settings for a project.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/services.html#get-jenkins-ci-service-settings
func (s *ServicesService) GetJenkinsCIService(pid interface{}, options ...OptionFunc) (*JenkinsCIService, *Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("projects/%s/services/jenkins", url.QueryEscape(project))

	req, err := s.client.NewRequest("GET", u, nil, options)
	if err != nil {
		return nil, nil, err
	}

	svc := new(JenkinsCIService)
	resp, err := s.client.Do(req, svc)
	if err != nil {
		return nil, resp, err
	}

	return svc, resp, err
}

// SetJenkinsCIServiceOptions represents the available SetJenkinsCIService()
// options.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/services.html#jenkins-ci
type SetJenkinsCIServiceOptions struct {
	URL         *string `url:"jenkins_url,omitempty" json:"jenkins_url,omitempty"`
	ProjectName *string `url:"project_name,omitempty" json:"project_name,omitempty"`
	Username    *string `url:"username,omitempty" json:"username,omitempty"`
	Password    *string `url:"password,omitempty" json:"password,omitempty"`
}

// SetJenkinsCIService sets Jenkins service for a project
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/services.html#create-edit-jenkins-ci-service
func (s *ServicesService) SetJenkinsCIService(pid interface{}, opt *SetJenkinsCIServiceOptions, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/services/jenkins", url.QueryEscape(project))

	req, err := s.client.NewRequest("PUT", u, opt, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// DeleteJenkinsCIService deletes Jenkins CI service for project.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#delete-jira-service
func (s *ServicesService) DeleteJenkinsCIService(pid interface{}, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/services/jenkins", url.QueryEscape(project))

	req, err := s.client.NewRequest("DELETE", u, nil, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
