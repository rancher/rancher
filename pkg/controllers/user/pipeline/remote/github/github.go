package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/remote/model"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/utils"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/tomnomnom/linkheader"
	"golang.org/x/oauth2"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

const (
	defaultGithubAPI = "https://api.github.com"
	maxPerPage       = "100"
	gheAPI           = "/api/v3"
)

type client struct {
	Scheme       string
	Host         string
	ClientID     string
	ClientSecret string
	API          string
}

func New(pipeline v3.ClusterPipeline) (model.Remote, error) {
	if pipeline.Spec.GithubConfig == nil {
		return nil, errors.New("github is not configured")
	}
	remote := &client{
		ClientID:     pipeline.Spec.GithubConfig.ClientID,
		ClientSecret: pipeline.Spec.GithubConfig.ClientSecret,
	}
	if pipeline.Spec.GithubConfig.Host != "" && pipeline.Spec.GithubConfig.Host != "github.com" {
		remote.Host = pipeline.Spec.GithubConfig.Host
		if pipeline.Spec.GithubConfig.TLS {
			remote.Scheme = "https://"
		} else {
			remote.Scheme = "http://"
		}
		remote.API = remote.Scheme + remote.Host + gheAPI
	} else {
		remote.Scheme = "https://"
		remote.Host = "github.com"
		remote.API = defaultGithubAPI
	}
	return remote, nil
}

func (c *client) Type() string {
	return "github"
}

func (c *client) CanLogin() bool {
	return true
}

func (c *client) CanRepos() bool {
	return true
}
func (c *client) CanHook() bool {
	return true
}

func (c *client) Login(redirectURL string, code string) (*v3.SourceCodeCredential, error) {
	githubOauthConfig := &oauth2.Config{
		RedirectURL:  redirectURL,
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		Scopes: []string{"repo",
			"admin:repo_hook"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  fmt.Sprintf("%s%s/login/oauth/authorize", c.Scheme, c.Host),
			TokenURL: fmt.Sprintf("%s%s/login/oauth/access_token", c.Scheme, c.Host),
		},
	}

	token, err := githubOauthConfig.Exchange(oauth2.NoContext, code)
	if err != nil {
		return nil, err
	} else if token.TokenType != "bearer" || token.AccessToken == "" {
		return nil, fmt.Errorf("Fail to get accesstoken with oauth config")
	}
	return c.GetAccount(token.AccessToken)
}

func (c *client) CreateHook(pipeline *v3.Pipeline, accessToken string) (string, error) {
	if len(pipeline.Spec.Stages) <= 0 || len(pipeline.Spec.Stages[0].Steps) <= 0 || pipeline.Spec.Stages[0].Steps[0].SourceCodeConfig == nil {
		return "", errors.New("invalid pipeline")
	}
	sourceCodeConfig := pipeline.Spec.Stages[0].Steps[0].SourceCodeConfig
	user, repo, err := getUserRepoFromURL(sourceCodeConfig.URL)
	if err != nil {
		return "", err
	}
	hookURL := fmt.Sprintf("%s?pipelineId=%s:%s", utils.CIEndpoint, pipeline.Namespace, pipeline.Name)
	id, err := c.createGithubWebhook(user, repo, accessToken, hookURL, pipeline.Status.Token)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (c *client) DeleteHook(pipeline *v3.Pipeline, accessToken string) error {
	if len(pipeline.Spec.Stages) <= 0 || len(pipeline.Spec.Stages[0].Steps) <= 0 || pipeline.Spec.Stages[0].Steps[0].SourceCodeConfig == nil {
		return errors.New("invalid pipeline")
	}
	sourceCodeConfig := pipeline.Spec.Stages[0].Steps[0].SourceCodeConfig
	user, repo, err := getUserRepoFromURL(sourceCodeConfig.URL)
	if err != nil {
		return err
	}

	return c.deleteGithubWebhook(user, repo, accessToken, pipeline.Status.WebHookID)
}

func (c *client) ParseHook(r *http.Request) {

}

func (c *client) GetAccount(accessToken string) (*v3.SourceCodeCredential, error) {
	account, err := c.getGithubUser(accessToken)
	if err != nil {
		return nil, err
	}
	remoteAccount := convertAccount(account)
	remoteAccount.Spec.AccessToken = accessToken
	return remoteAccount, nil
}

func (c *client) Repos(account *v3.SourceCodeCredential) ([]v3.SourceCodeRepository, error) {
	if account == nil {
		return nil, fmt.Errorf("empty account")
	}
	accessToken := account.Spec.AccessToken
	return c.getGithubRepos(accessToken)
}

func (c *client) getGithubUser(githubAccessToken string) (*github.User, error) {

	url := c.API + "/user"
	resp, err := getFromGithub(githubAccessToken, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	githubAcct := &github.User{}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(b, githubAcct); err != nil {
		return nil, err
	}
	return githubAcct, nil
}

func convertAccount(gitaccount *github.User) *v3.SourceCodeCredential {

	if gitaccount == nil {
		return nil
	}
	account := &v3.SourceCodeCredential{}
	account.Spec.SourceCodeType = "github"
	if gitaccount.AvatarURL != nil {
		account.Spec.AvatarURL = *gitaccount.AvatarURL
	}
	if gitaccount.HTMLURL != nil {
		account.Spec.HTMLURL = *gitaccount.HTMLURL
	}
	if gitaccount.Login != nil {
		account.Spec.LoginName = *gitaccount.Login
	}
	if gitaccount.Name != nil {
		account.Spec.DisplayName = *gitaccount.Name
		account.Name = strings.ToLower(*gitaccount.Login)
	}
	return account

}

func (c *client) getGithubRepos(githubAccessToken string) ([]v3.SourceCodeRepository, error) {
	url := c.API + "/user/repos"
	var repos []github.Repository
	responses, err := paginateGithub(githubAccessToken, url)
	if err != nil {
		return nil, err
	}
	for _, response := range responses {
		defer response.Body.Close()
		b, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}
		var reposObj []github.Repository
		if err := json.Unmarshal(b, &reposObj); err != nil {
			return nil, err
		}
		repos = append(repos, reposObj...)
	}

	return convertRepos(repos), nil
}

func convertRepos(repos []github.Repository) []v3.SourceCodeRepository {
	result := []v3.SourceCodeRepository{}
	for _, repo := range repos {
		r := v3.SourceCodeRepository{}
		if repo.CloneURL != nil {
			r.Spec.URL = *repo.CloneURL
		}
		if repo.Language != nil {
			r.Spec.Language = *repo.Language
		}
		if repo.Permissions != nil {
			if (*repo.Permissions)["pull"] == true {
				r.Spec.Permissions.Pull = true
			}
			if (*repo.Permissions)["push"] == true {
				r.Spec.Permissions.Push = true
			}
			if (*repo.Permissions)["admin"] == true {
				r.Spec.Permissions.Admin = true
			}
		}
		result = append(result, r)
	}
	return result
}

func paginateGithub(githubAccessToken string, url string) ([]*http.Response, error) {
	var responses []*http.Response

	response, err := getFromGithub(githubAccessToken, url)
	if err != nil {
		return responses, err
	}
	responses = append(responses, response)
	nextURL := nextGithubPage(response)
	for nextURL != "" {
		response, err = getFromGithub(githubAccessToken, nextURL)
		if err != nil {
			return responses, err
		}
		responses = append(responses, response)
		nextURL = nextGithubPage(response)
	}

	return responses, nil
}

func getFromGithub(githubAccessToken string, url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{}
	//set to max 100 per page to reduce query time
	q := req.URL.Query()
	q.Set("per_page", maxPerPage)
	req.URL.RawQuery = q.Encode()

	req.Header.Add("Authorization", "token "+githubAccessToken)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.103 Safari/537.36)")
	resp, err := client.Do(req)
	if err != nil {
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

func nextGithubPage(response *http.Response) string {
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
