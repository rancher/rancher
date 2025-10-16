package githubapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/go-github/v73/github"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/sirupsen/logrus"
	"github.com/tomnomnom/linkheader"
	"k8s.io/utils/ptr"
)

const (
	gheAPI                = "/api/v3"
	githubAPI             = "https://api.github.com"
	githubDefaultHostName = "https://github.com"

	// used to limit the size of the response from GitHub = 5MiB
	maxGitHubBodySize int64 = 1024 * 1024 * 5
)

// githubAppClient implements client for GitHub using a GitHub App.
type githubAppClient struct {
	httpClient *http.Client
}

func (g *githubAppClient) getAccessToken(ctx context.Context, code string, config *apiv3.GithubAppConfig) (string, error) {
	form := url.Values{}
	form.Add("client_id", config.ClientID)
	form.Add("client_secret", config.ClientSecret)
	form.Add("code", code)

	url := getAPIURL("TOKEN", config)

	b, err := g.postToGithub(ctx, url, form)
	if err != nil {
		return "", fmt.Errorf("github getAccessToken: POST url %v received error from github, err: %v", url, err)
	}

	// Decode the response
	var respMap map[string]any

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

func (g *githubAppClient) getUser(ctx context.Context, githubAccessToken string, config *apiv3.GithubAppConfig) (common.GitHubAccount, error) {
	url := getAPIURL("USER_INFO", config)
	b, _, err := g.getFromGithub(ctx, githubAccessToken, url)
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

// TODO: These two need to use the cache.
func (g *githubAppClient) getOrgsForUser(ctx context.Context, username string, config *apiv3.GithubAppConfig) ([]common.GitHubAccount, error) {
	data, err := getGitHubAppDataFromConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	return data.listOrgsForUser(username), nil
}

func (g *githubAppClient) getTeamsForUser(ctx context.Context, username string, config *apiv3.GithubAppConfig) ([]common.GitHubAccount, error) {
	data, err := getGitHubAppDataFromConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	return data.listTeamsForUser(username), nil
}

func (g *githubAppClient) searchUsers(ctx context.Context, searchTerm, searchType string, config *apiv3.GithubAppConfig) ([]common.GitHubAccount, error) {
	client, err := getInstallationClientFromConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	result, _, err := client.Search.Users(ctx, searchTerm, &github.SearchOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to search for users with term: %s: %w", searchTerm, err)
	}

	var searchResult []common.GitHubAccount
	// TODO: Incomplete searches?
	for _, user := range result.Users {
		m := member{
			gitHubObject: gitHubObject{
				name:      ptr.Deref(user.Name, ""),
				login:     ptr.Deref(user.Login, ""),
				avatarURL: ptr.Deref(user.AvatarURL, ""),
				id:        int(ptr.Deref(user.ID, 0)),
				htmlURL:   ptr.Deref(user.HTMLURL, ""),
			},
		}
		searchResult = append(searchResult, m.toAccount())
	}

	// Cache this
	data, err := getGitHubAppDataFromConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	searchResult = append(searchResult, data.searchOrgs(searchTerm)...)

	return searchResult, nil
}

func (g *githubAppClient) searchTeams(ctx context.Context, searchTerm string, config *apiv3.GithubAppConfig) ([]common.GitHubAccount, error) {
	data, err := getGitHubAppDataFromConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	return data.searchTeams(searchTerm), nil
}

func (g *githubAppClient) getTeamByID(ctx context.Context, id int, config *apiv3.GithubAppConfig) (common.GitHubAccount, error) {
	return common.GitHubAccount{}, errors.New("fail getTeamByID")
}

func (g *githubAppClient) getUserOrgByID(ctx context.Context, id int, config *apiv3.GithubAppConfig) (common.GitHubAccount, error) {
	data, err := getGitHubAppDataFromConfig(ctx, config)
	if err != nil {
		return common.GitHubAccount{}, err
	}

	acct := data.findOrgByID(id)
	if acct != nil {
		return *acct, nil
	}

	// This does not return the user "DisplayName" because it's not part of the
	// membership response.
	acct = data.findMemberByID(id)
	if acct != nil {
		return *acct, nil
	}

	return common.GitHubAccount{}, fmt.Errorf("unknown ID %v", id)
}

func (g *githubAppClient) postToGithub(ctx context.Context, url string, form url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
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

	body, err := io.ReadAll(&io.LimitedReader{R: resp.Body, N: maxGitHubBodySize})
	if err != nil {
		return nil, fmt.Errorf("reading response from GitHub: %w", err)
	}

	// Check the status code
	switch resp.StatusCode {
	case 200:
	case 201:
	default:
		return nil, fmt.Errorf("request failed, got status code: %d. Response: %s",
			resp.StatusCode, body)
	}

	return body, nil
}

func (g *githubAppClient) getFromGithub(ctx context.Context, githubAccessToken string, url string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Add("Authorization", "token "+githubAccessToken)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("User-agent", "rancher/github-app-client")
	resp, err := g.httpClient.Do(req)
	if err != nil {
		logrus.Errorf("Received error from github: %v", err)
		return nil, "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(&io.LimitedReader{R: resp.Body, N: maxGitHubBodySize})
	if err != nil {
		return nil, "", fmt.Errorf("reading response from GitHub: %w", err)
	}

	// Check the status code
	switch resp.StatusCode {
	case 200:
	case 201:
	default:
		return nil, "", fmt.Errorf("request failed, got status code: %d. Response: %s",
			resp.StatusCode, body)
	}
	nextURL := g.nextGithubPage(resp)

	return body, nextURL, nil
}

func getAPIURL(endpoint string, config *apiv3.GithubAppConfig) string {
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

func (g *githubAppClient) nextGithubPage(response *http.Response) string {
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

// TODO: Move this into a method on the struct and cache!
func getGitHubAppDataFromConfig(ctx context.Context, config *apiv3.GithubAppConfig) (*gitHubAppData, error) {
	appID, installationID, err := getInstallationAndAppIDFromConfig(config)
	if err != nil {
		return nil, err
	}

	return getDataForApp(ctx, appID, []byte(config.PrivateKey), installationID, getAPIURL("", config))
}

// TODO: Move this into a method on the struct and cache!
func getInstallationClientFromConfig(ctx context.Context, config *apiv3.GithubAppConfig) (*github.Client, error) {
	appID, installationID, err := getInstallationAndAppIDFromConfig(config)
	if err != nil {
		return nil, err
	}

	return getInstallationClient(ctx, appID, []byte(config.PrivateKey), installationID, getAPIURL("", config))
}

func getInstallationAndAppIDFromConfig(config *apiv3.GithubAppConfig) (int64, int64, error) {
	appID, err := strconv.ParseInt(config.AppID, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parsing AppID: %w", err)
	}

	var installationID int64
	if config.InstallationID != "" {
		parsed, err := strconv.ParseInt(config.InstallationID, 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("parsing InstallationID: %w", err)
		}
		installationID = parsed
	}

	return appID, installationID, nil
}
