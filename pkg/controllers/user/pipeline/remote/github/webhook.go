package github

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/go-github/github"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
)

//create webhook,return id of webhook
func (c *client) createGithubWebhook(user string, repo string, accesstoken string, webhookURL string, secret string, events []string) (string, error) {
	data := user + ":" + accesstoken
	sEnc := base64.StdEncoding.EncodeToString([]byte(data))
	name := "web"
	active := true
	hook := github.Hook{
		Name:   &name,
		Active: &active,
		Config: make(map[string]interface{}),
		Events: events,
	}

	hook.Config["url"] = webhookURL
	hook.Config["content_type"] = "json"
	hook.Config["secret"] = secret
	hook.Config["insecure_ssl"] = "1"

	logrus.Debugf("hook to create:%v", hook)
	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(hook)
	client := http.Client{}
	APIURL := fmt.Sprintf("%s/repos/%s/%s/hooks", c.API, user, repo)
	req, err := http.NewRequest("POST", APIURL, b)
	if err != nil {
		return "", err
	}

	req.Header.Add("Authorization", "Basic "+sEnc)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	logrus.Infof("respData:%v", string(respData))
	if resp.StatusCode > 399 {
		return "", errors.New(string(respData))
	}
	err = json.Unmarshal(respData, &hook)
	if err != nil {
		return "", err
	}
	return strconv.Itoa(hook.GetID()), err
}

func getUserRepoFromURL(repoURL string) (string, string, error) {
	reg := regexp.MustCompile(".*/([^/]*?)/([^/]*?).git")
	match := reg.FindStringSubmatch(repoURL)
	if len(match) != 3 {
		return "", "", fmt.Errorf("error getting user/repo from gitrepoUrl:%v", repoURL)
	}
	return match[1], match[2], nil
}

func (c *client) deleteGithubWebhook(user string, repo string, accesstoken string, id string) error {

	data := user + ":" + accesstoken
	sEnc := base64.StdEncoding.EncodeToString([]byte(data))
	client := http.Client{}
	APIURL := fmt.Sprintf("%s/repos/%s/%s/hooks/%v", c.API, user, repo, id)
	req, err := http.NewRequest("DELETE", APIURL, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", "Basic "+sEnc)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respData, err := ioutil.ReadAll(resp.Body)
	if resp.StatusCode > 399 {
		return errors.New(string(respData))
	}
	logrus.Debugf("after delete,%v,%v", string(respData))
	return err
}
