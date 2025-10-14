package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/sirupsen/logrus"
	"github.com/tomnomnom/linkheader"
)

const (
	gheAPI                = "/api/v3"
	githubAPI             = "https://api.github.com"
	githubDefaultHostName = "https://github.com"
)

type searchResult struct {
	Items []common.GitHubAccount `json:"items"`
}

// GClient implements a httpclient for github
type GClient struct {
	httpClient *http.Client
}

func (g *GClient) getAccessToken(code string, config *v32.GithubConfig) (string, error) {

	form := url.Values{}
	form.Add("client_id", config.ClientID)
	form.Add("client_secret", config.ClientSecret)
	form.Add("code", code)

	url := g.getURL("TOKEN", config)

	b, err := g.postToGithub(url, form)
	if err != nil {
		return "", fmt.Errorf("github getAccessToken: POST url %v received error from github, err: %v", url, err)
	}

	// Decode the response
	var respMap map[string]interface{}

	if err := json.Unmarshal(b, &respMap); err != nil {
		return "", fmt.Errorf("github getAccessToken: received error unmarshalling response body, err: %v", err)
	}

	if respMap["error"] != nil {
		desc := respMap["error_description"]
		return "", fmt.Errorf("github getAccessToken: received error from github %v, description from github %v", respMap["error"], desc)
	}

	acessToken, ok := respMap["access_token"].(string)
	if !ok {
		return "", fmt.Errorf("github getAccessToken: received error reading accessToken from response %v", respMap)
	}
	return acessToken, nil
}

func (g *GClient) getUser(githubAccessToken string, config *v32.GithubConfig) (common.GitHubAccount, error) {

	url := g.getURL("USER_INFO", config)
	b, _, err := g.getFromGithub(githubAccessToken, url)
	if err != nil {
		logrus.Errorf("Github getGithubUser: GET url %v received error from github, err: %v", url, err)
		return common.GitHubAccount{}, err
	}
	var githubAcct common.GitHubAccount

	if err := json.Unmarshal(b, &githubAcct); err != nil {
		logrus.Errorf("Github getGithubUser: error unmarshalling response, err: %v", err)
		return common.GitHubAccount{}, err
	}

	return githubAcct, nil
}

func (g *GClient) getOrgs(githubAccessToken string, config *v32.GithubConfig) ([]common.GitHubAccount, error) {
	var orgs []common.GitHubAccount

	url := g.getURL("ORG_INFO", config)
	responses, err := g.paginateGithub(githubAccessToken, url)
	if err != nil {
		logrus.Errorf("Github getGithubOrgs: GET url %v received error from github, err: %v", url, err)
		return orgs, err
	}

	for _, b := range responses {
		var orgObjs []common.GitHubAccount
		if err := json.Unmarshal(b, &orgObjs); err != nil {
			logrus.Errorf("Github getGithubOrgs: received error unmarshalling org array, err: %v", err)
			return nil, err
		}
		orgs = append(orgs, orgObjs...)
	}

	return orgs, nil
}

func (g *GClient) getTeams(githubAccessToken string, config *v32.GithubConfig) ([]common.GitHubAccount, error) {
	var teams []common.GitHubAccount

	url := g.getURL("TEAMS", config)
	responses, err := g.paginateGithub(githubAccessToken, url)
	if err != nil {
		logrus.Errorf("Github getGithubTeams: GET url %v received error from github, err: %v", url, err)
		return teams, err
	}
	for _, response := range responses {
		teamObjs, err := g.getTeamInfo(response, config)

		if err != nil {
			logrus.Errorf("Github getGithubTeams: received error unmarshalling teams array, err: %v", err)
			return teams, err
		}
		teams = append(teams, teamObjs...)

	}
	return teams, nil
}

// getOrgTeams returns the teams belonging to an organization.
func (g *GClient) getOrgTeams(githubAccessToken string, config *v32.GithubConfig, org common.GitHubAccount) ([]common.GitHubAccount, error) {
	url := fmt.Sprintf(g.getURL("ORG_TEAMS", config), url.PathEscape(org.Login))
	responses, err := g.paginateGithub(githubAccessToken, url)
	if err != nil {
		logrus.Errorf("Github getGithubTeams: GET url %v received error from github, err: %v", url, err)
		return nil, err
	}

	var teams, respTeams []common.GitHubAccount
	for _, response := range responses {
		respTeams, err = g.getOrgTeamInfo(response, config, org)
		if err != nil {
			logrus.Errorf("Github getOrgTeams: received error unmarshalling teams array, err: %v", err)
			return teams, err
		}
		teams = append(teams, respTeams...)
	}

	return teams, nil
}

// getOrgTeamInfo is similar to getTeamInfo but takes an org as an argument.
func (g *GClient) getOrgTeamInfo(b []byte, config *v32.GithubConfig, org common.GitHubAccount) ([]common.GitHubAccount, error) {
	var teams []common.GitHubAccount
	var teamObjs []common.GitHubTeam
	if err := json.Unmarshal(b, &teamObjs); err != nil {
		logrus.Errorf("Github getTeamInfo: received error unmarshalling team array, err: %v", err)
		return teams, err
	}

	url := g.getURL("TEAM_PROFILE", config)
	for _, team := range teamObjs {
		teams = append(teams, common.GitHubAccount{
			ID:        team.ID,
			Name:      team.Name,
			AvatarURL: org.AvatarURL,
			HTMLURL:   fmt.Sprintf(url, org.Login, team.Slug),
			Login:     team.Slug,
		})
	}

	return teams, nil
}

func (g *GClient) getTeamInfo(b []byte, config *v32.GithubConfig) ([]common.GitHubAccount, error) {
	var teams []common.GitHubAccount
	var teamObjs []common.GitHubTeam
	if err := json.Unmarshal(b, &teamObjs); err != nil {
		logrus.Errorf("Github getTeamInfo: received error unmarshalling team array, err: %v", err)
		return teams, err
	}

	url := g.getURL("TEAM_PROFILE", config)
	for _, team := range teamObjs {
		teams = append(teams, team.ToGitHubAccount(url))
	}

	return teams, nil
}

func (g *GClient) getTeamByID(id string, githubAccessToken string, config *v32.GithubConfig) (common.GitHubAccount, error) {
	var teamAcct common.GitHubAccount

	url := g.getURL("TEAM", config) + id
	b, _, err := g.getFromGithub(githubAccessToken, url)
	if err != nil {
		logrus.Errorf("Github getTeamByID: GET url %v received error from github, err: %v", url, err)
		return teamAcct, err
	}
	var teamObj common.GitHubTeam
	if err := json.Unmarshal(b, &teamObj); err != nil {
		logrus.Errorf("Github getTeamByID: received error unmarshalling team array, err: %v", err)
		return teamAcct, err
	}
	url = g.getURL("TEAM_PROFILE", config)

	return teamObj.ToGitHubAccount(url), nil
}

func (g *GClient) paginateGithub(githubAccessToken string, url string) ([][]byte, error) {
	var responses [][]byte
	var err error
	var response []byte
	nextURL := url
	for nextURL != "" {
		response, nextURL, err = g.getFromGithub(githubAccessToken, nextURL)
		if err != nil {
			return nil, err
		}
		responses = append(responses, response)
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

func (g *GClient) searchUsers(searchTerm, searchType string, githubAccessToken string, config *v32.GithubConfig) ([]common.GitHubAccount, error) {
	if searchType == "group" {
		searchType = orgType
	}

	search := searchTerm
	if searchType != "" {
		search += "+type:" + searchType
	}
	search = URLEncoded(search)
	url := g.getURL("USER_SEARCH", config) + search

	b, _, err := g.getFromGithub(githubAccessToken, url)
	if err != nil {
		// no match on search returns an error. do not log
		return nil, nil
	}

	result := &searchResult{}
	if err := json.Unmarshal(b, result); err != nil {
		return nil, err
	}

	return result.Items, nil
}

// searchTeams searches for teams that match the search term in the organizations the access token has access to.
// At the moment it only does a case-insensitive prefix match on the team's name.
func (g *GClient) searchTeams(searchTerm, githubAccessToken string, config *v32.GithubConfig) ([]common.GitHubAccount, error) {
	orgs, err := g.getOrgs(githubAccessToken, config)
	if err != nil {
		return nil, err
	}

	lowerSearchTerm := strings.ToLower(searchTerm)

	var matches, teams []common.GitHubAccount
	for _, org := range orgs {
		teams, err = g.getOrgTeams(githubAccessToken, config, org)
		if err != nil {
			return nil, err
		}

		for _, team := range teams {
			if !strings.HasPrefix(strings.ToLower(team.Name), lowerSearchTerm) {
				continue
			}

			matches = append(matches, team)
		}

	}

	return matches, nil
}

func (g *GClient) getUserOrgByID(id string, githubAccessToken string, config *v32.GithubConfig) (common.GitHubAccount, error) {
	url := g.getURL("USER_INFO", config) + "/" + id

	b, _, err := g.getFromGithub(githubAccessToken, url)
	if err != nil {
		logrus.Errorf("Github getUserOrgById: GET url %v received error from github, err: %v", url, err)
		return common.GitHubAccount{}, err
	}
	var githubAcct common.GitHubAccount

	if err := json.Unmarshal(b, &githubAcct); err != nil {
		logrus.Errorf("Github getUserOrgById: error unmarshalling response, err: %v", err)
		return common.GitHubAccount{}, err
	}

	return githubAcct, nil
}

// URLEncoded encodes the string
func URLEncoded(str string) string {
	u, err := url.Parse(str)
	if err != nil {
		logrus.Errorf("Error encoding the url: %s, error: %v", str, err)
		return str
	}
	return u.String()
}

func (g *GClient) postToGithub(url string, form url.Values) ([]byte, error) {
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
		return nil, err
	}

	defer resp.Body.Close()
	// Check the status code
	switch resp.StatusCode {
	case 200:
	case 201:
	default:
		var body bytes.Buffer
		io.Copy(&body, resp.Body)
		return nil, fmt.Errorf("request failed, got status code: %d. Response: %s",
			resp.StatusCode, body.Bytes())
	}
	return io.ReadAll(resp.Body)
}

func (g *GClient) getFromGithub(githubAccessToken string, url string) ([]byte, string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Add("Authorization", "token "+githubAccessToken)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.103 Safari/537.36)")
	resp, err := g.httpClient.Do(req)
	if err != nil {
		logrus.Errorf("Received error from github: %v", err)
		return nil, "", err
	}
	defer resp.Body.Close()
	// Check the status code
	switch resp.StatusCode {
	case 200:
	case 201:
	default:
		var body bytes.Buffer
		io.Copy(&body, resp.Body)
		return nil, "", fmt.Errorf("request failed, got status code: %d. Response: %s",
			resp.StatusCode, body.Bytes())
	}

	nextURL := g.nextGithubPage(resp)
	b, err := io.ReadAll(resp.Body)
	return b, nextURL, err
}

func (g *GClient) getURL(endpoint string, config *v32.GithubConfig) string {
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
	case "ORG_TEAMS":
		toReturn = apiEndpoint + "/orgs/%s/teams?per_page=100"
	default:
		toReturn = apiEndpoint
	}

	return toReturn
}
