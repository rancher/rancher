package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/tomnomnom/linkheader"

	"github.com/rancher/types/apis/management.cattle.io/v3"
)

const (
	gheAPI                = "/api/v3"
	githubAPI             = "https://api.github.com"
	githubDefaultHostName = "https://github.com"
)

//GClient implements a httpclient for github
type GClient struct {
	httpClient *http.Client
}

func (g *GClient) getAccessToken(code string, config *v3.GithubConfig) (string, error) {

	form := url.Values{}
	form.Add("client_id", config.ClientID)
	form.Add("client_secret", config.ClientSecret)
	form.Add("code", code)

	url := g.getURL("TOKEN", config)

	resp, err := g.postToGithub(url, form)
	if err != nil {
		logrus.Errorf("Github getAccessToken: GET url %v received error from github, err: %v", url, err)
		return "", err
	}
	defer resp.Body.Close()

	// Decode the response
	var respMap map[string]interface{}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Errorf("Github getAccessToken: received error reading response body, err: %v", err)
		return "", err
	}

	if err := json.Unmarshal(b, &respMap); err != nil {
		logrus.Errorf("Github getAccessToken: received error unmarshalling response body, err: %v", err)
		return "", err
	}

	if respMap["error"] != nil {
		desc := respMap["error_description"]
		logrus.Errorf("Received Error from github %v, description from github %v", respMap["error"], desc)
		return "", fmt.Errorf("Received Error from github %v, description from github %v", respMap["error"], desc)
	}

	acessToken, ok := respMap["access_token"].(string)
	if !ok {
		return "", fmt.Errorf("Received Error reading accessToken from response %v", respMap)
	}
	return acessToken, nil
}

func (g *GClient) getUser(githubAccessToken string, config *v3.GithubConfig) (Account, error) {

	url := g.getURL("USER_INFO", config)
	resp, err := g.getFromGithub(githubAccessToken, url)
	if err != nil {
		logrus.Errorf("Github getGithubUser: GET url %v received error from github, err: %v", url, err)
		return Account{}, err
	}
	defer resp.Body.Close()
	var githubAcct Account

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Errorf("Github getGithubUser: error reading response, err: %v", err)
		return Account{}, err
	}

	if err := json.Unmarshal(b, &githubAcct); err != nil {
		logrus.Errorf("Github getGithubUser: error unmarshalling response, err: %v", err)
		return Account{}, err
	}

	return githubAcct, nil
}

func (g *GClient) getOrgs(githubAccessToken string, config *v3.GithubConfig) ([]Account, error) {
	var orgs []Account

	url := g.getURL("ORG_INFO", config)
	responses, err := g.paginateGithub(githubAccessToken, url)
	if err != nil {
		logrus.Errorf("Github getGithubOrgs: GET url %v received error from github, err: %v", url, err)
		return orgs, err
	}

	for _, response := range responses {
		defer response.Body.Close()
		var orgObjs []Account
		b, err := ioutil.ReadAll(response.Body)
		if err != nil {
			logrus.Errorf("Github getGithubOrgs: error reading the response from github, err: %v", err)
			return orgs, err
		}
		if err := json.Unmarshal(b, &orgObjs); err != nil {
			logrus.Errorf("Github getGithubOrgs: received error unmarshalling org array, err: %v", err)
			return orgs, err
		}
		for _, orgObj := range orgObjs {
			orgs = append(orgs, orgObj)
		}
	}

	return orgs, nil
}

func (g *GClient) getTeams(githubAccessToken string, config *v3.GithubConfig) ([]Account, error) {
	var teams []Account

	url := g.getURL("TEAMS", config)
	responses, err := g.paginateGithub(githubAccessToken, url)
	if err != nil {
		logrus.Errorf("Github getGithubTeams: GET url %v received error from github, err: %v", url, err)
		return teams, err
	}
	for _, response := range responses {
		defer response.Body.Close()
		teamObjs, err := g.getTeamInfo(response, config)

		if err != nil {
			logrus.Errorf("Github getGithubTeams: received error unmarshalling teams array, err: %v", err)
			return teams, err
		}
		for _, teamObj := range teamObjs {
			teams = append(teams, teamObj)
		}

	}
	return teams, nil
}

func (g *GClient) getTeamInfo(response *http.Response, config *v3.GithubConfig) ([]Account, error) {
	var teams []Account
	b, err := ioutil.ReadAll(response.Body)
	if err != nil {
		logrus.Errorf("Github getTeamInfo: error reading the response from github, err: %v", err)
		return teams, err
	}
	var teamObjs []Team
	if err := json.Unmarshal(b, &teamObjs); err != nil {
		logrus.Errorf("Github getTeamInfo: received error unmarshalling team array, err: %v", err)
		return teams, err
	}

	url := g.getURL("TEAM_PROFILE", config)
	for _, team := range teamObjs {
		teamAcct := Account{}
		team.toGithubAccount(url, &teamAcct)
		teams = append(teams, teamAcct)
	}

	return teams, nil
}

func (g *GClient) getTeamByID(id string, githubAccessToken string, config *v3.GithubConfig) (Account, error) {
	var teamAcct Account

	url := g.getURL("TEAM", config) + id
	response, err := g.getFromGithub(githubAccessToken, url)
	if err != nil {
		logrus.Errorf("Github getTeamByID: GET url %v received error from github, err: %v", url, err)
		return teamAcct, err
	}
	b, err := ioutil.ReadAll(response.Body)
	if err != nil {
		logrus.Errorf("Github getTeamByID: error reading the response from github, err: %v", err)
		return teamAcct, err
	}
	var teamObj Team
	if err := json.Unmarshal(b, &teamObj); err != nil {
		logrus.Errorf("Github getTeamByID: received error unmarshalling team array, err: %v", err)
		return teamAcct, err
	}
	url = g.getURL("TEAM_PROFILE", config)
	teamObj.toGithubAccount(url, &teamAcct)

	return teamAcct, nil
}

func (g *GClient) paginateGithub(githubAccessToken string, url string) ([]*http.Response, error) {
	var responses []*http.Response

	response, err := g.getFromGithub(githubAccessToken, url)
	if err != nil {
		return responses, err
	}
	responses = append(responses, response)
	nextURL := g.nextGithubPage(response)
	for nextURL != "" {
		response, err = g.getFromGithub(githubAccessToken, nextURL)
		if err != nil {
			return responses, err
		}
		responses = append(responses, response)
		nextURL = g.nextGithubPage(response)
	}

	return responses, nil
}

func (g *GClient) nextGithubPage(response *http.Response) string {
	header := response.Header.Get("link")

	if header != "" {
		links := linkheader.Parse(header)
		for _, link := range links {
			if link.Rel == "next" {
				return link.URL
			}
		}
	}

	return ""
}

func (g *GClient) getUserByName(username string, githubAccessToken string, config *v3.GithubConfig) (*Account, error) {
	_, err := g.getOrgByName(username, githubAccessToken, config)
	if err == nil {
		logrus.Info("There is a org by this name, not looking fo the user entity by name %v", username)
		return nil, nil
	}

	username = URLEncoded(username)
	url := g.getURL("USERS", config) + username

	resp, err := g.getFromGithub(githubAccessToken, url)
	if err != nil {
		// no match on search returns an error. do not log
		return nil, nil
	}
	defer resp.Body.Close()
	githubAcct := &Account{}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(b, githubAcct); err != nil {
		return nil, err
	}

	return githubAcct, nil
}

func (g *GClient) getOrgByName(org string, githubAccessToken string, config *v3.GithubConfig) (Account, error) {
	org = URLEncoded(org)
	url := g.getURL("ORGS", config) + org

	resp, err := g.getFromGithub(githubAccessToken, url)
	if err != nil {
		logrus.Debugf("Github getGithubOrgByName: GET url %v received error from github, err: %v", url, err)
		return Account{}, err
	}
	defer resp.Body.Close()
	var githubAcct Account

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Errorf("Github getGithubOrgByName: error reading response, err: %v", err)
		return Account{}, err
	}

	if err := json.Unmarshal(b, &githubAcct); err != nil {
		logrus.Errorf("Github getGithubOrgByName: error unmarshalling response, err: %v", err)
		return Account{}, err
	}

	return githubAcct, nil
}

func (g *GClient) getUserOrgByID(id string, githubAccessToken string, config *v3.GithubConfig) (Account, error) {
	url := g.getURL("USER_INFO", config) + "/" + id

	resp, err := g.getFromGithub(githubAccessToken, url)
	if err != nil {
		logrus.Errorf("Github getUserOrgById: GET url %v received error from github, err: %v", url, err)
		return Account{}, err
	}
	defer resp.Body.Close()
	var githubAcct Account

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Errorf("Github getUserOrgById: error reading response, err: %v", err)
		return Account{}, err
	}

	if err := json.Unmarshal(b, &githubAcct); err != nil {
		logrus.Errorf("Github getUserOrgById: error unmarshalling response, err: %v", err)
		return Account{}, err
	}

	return githubAcct, nil
}

//URLEncoded encodes the string
func URLEncoded(str string) string {
	u, err := url.Parse(str)
	if err != nil {
		logrus.Errorf("Error encoding the url: %s, error: %v", str, err)
		return str
	}
	return u.String()
}

func (g *GClient) postToGithub(url string, form url.Values) (*http.Response, error) {
	req, err := http.NewRequest("POST", url, strings.NewReader(form.Encode()))
	if err != nil {
		logrus.Error(err)
	}
	req.PostForm = form
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Accept", "application/json")
	resp, err := g.httpClient.Do(req)
	if err != nil {
		logrus.Errorf("Received error from github: %v", err)
		return resp, err
	}
	// Check the status code
	switch resp.StatusCode {
	case 200:
	case 201:
	default:
		var body bytes.Buffer
		io.Copy(&body, resp.Body)
		return resp, fmt.Errorf("Request failed, got status code: %d. Response: %s",
			resp.StatusCode, body.Bytes())
	}
	return resp, nil
}

func (g *GClient) getFromGithub(githubAccessToken string, url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logrus.Error(err)
	}
	req.Header.Add("Authorization", "token "+githubAccessToken)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.103 Safari/537.36)")
	resp, err := g.httpClient.Do(req)
	if err != nil {
		logrus.Errorf("Received error from github: %v", err)
		return resp, err
	}
	// Check the status code
	switch resp.StatusCode {
	case 200:
	case 201:
	default:
		var body bytes.Buffer
		io.Copy(&body, resp.Body)
		return resp, fmt.Errorf("Request failed, got status code: %d. Response: %s",
			resp.StatusCode, body.Bytes())
	}
	return resp, nil
}

func (g *GClient) getURL(endpoint string, config *v3.GithubConfig) string {

	var hostName, apiEndpoint, toReturn string

	if config.Hostname != "" {
		scheme := "http://"
		if config.TLS {
			scheme = "https://"
		}
		hostName = scheme + config.Hostname
		if hostName == githubDefaultHostName {
			apiEndpoint = githubAPI
		} else {
			apiEndpoint = scheme + config.Hostname + gheAPI
		}
	} else {
		hostName = githubDefaultHostName
		apiEndpoint = githubAPI
	}

	switch endpoint {
	case "API":
		toReturn = apiEndpoint
	case "TOKEN":
		toReturn = hostName + "/login/oauth/access_token"
	case "USERS":
		toReturn = apiEndpoint + "/users/"
	case "ORGS":
		toReturn = apiEndpoint + "/orgs/"
	case "USER_INFO":
		toReturn = apiEndpoint + "/user"
	case "ORG_INFO":
		toReturn = apiEndpoint + "/user/orgs?per_page=1"
	case "USER_PICTURE":
		toReturn = "https://avatars.githubusercontent.com/u/" + endpoint + "?v=3&s=72"
	case "USER_SEARCH":
		toReturn = apiEndpoint + "/search/users?q="
	case "TEAM":
		toReturn = apiEndpoint + "/teams/"
	case "TEAMS":
		toReturn = apiEndpoint + "/user/teams?per_page=100"
	case "TEAM_PROFILE":
		toReturn = hostName + "/orgs/%s/teams/%s"
	default:
		toReturn = apiEndpoint
	}

	return toReturn
}
