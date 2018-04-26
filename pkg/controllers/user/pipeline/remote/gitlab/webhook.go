package gitlab

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/go-querystring/query"
	"github.com/xanzy/go-gitlab"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"
)

//create webhook,return id of webhook
func (c *client) createGitlabWebhook(user string, repo string, accesstoken string, webhookURL string, secret string) (string, error) {

	project := url.QueryEscape(user + "/" + repo)
	client := http.Client{
		Timeout: 15 * time.Second,
	}
	APIURL := fmt.Sprintf(c.API+"/projects/%s/hooks", project)
	req, err := http.NewRequest("POST", APIURL, nil)

	opt := &gitlab.AddProjectHookOptions{
		PushEvents:          gitlab.Bool(true),
		MergeRequestsEvents: gitlab.Bool(true),
		TagPushEvents:       gitlab.Bool(true),
		URL:                 gitlab.String(webhookURL),
		EnableSSLVerification: gitlab.Bool(false),
		Token: gitlab.String(secret),
	}
	q, err := query.Values(opt)
	if err != nil {
		return "", err
	}
	req.URL.RawQuery = q.Encode()
	req.Header.Add("Authorization", "Bearer "+accesstoken)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode > 399 {
		return "", errors.New(string(respData))
	}
	hook := gitlab.ProjectHook{}
	err = json.Unmarshal(respData, &hook)
	if err != nil {
		return "", err
	}
	return strconv.Itoa(hook.ID), nil
}

func getUserRepoFromURL(repoURL string) (string, string, error) {
	reg := regexp.MustCompile(".*/([^/]*?)/([^/]*?).git")
	match := reg.FindStringSubmatch(repoURL)
	if len(match) != 3 {
		return "", "", fmt.Errorf("error getting user/repo from gitrepoUrl:%v", repoURL)
	}
	return match[1], match[2], nil
}

func (c *client) deleteGitlabWebhook(user string, repo string, accesstoken string, id string) error {

	client := http.Client{
		Timeout: 15 * time.Second,
	}
	project := url.QueryEscape(user + "/" + repo)
	APIURL := fmt.Sprintf(c.API+"/projects/%s/hooks/%s", project, id)
	req, err := http.NewRequest("DELETE", APIURL, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", "Bearer "+accesstoken)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode > 399 {
		return errors.New(string(respData))
	}
	return nil
}
