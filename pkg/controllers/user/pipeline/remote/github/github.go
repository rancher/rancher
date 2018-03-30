package github

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/catalog/git"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/remote/model"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/utils"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
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
	if err := utils.ValidPipelineSpec(pipeline.Spec); err != nil {
		return "", err
	}
	sourceCodeConfig := pipeline.Spec.Stages[0].Steps[0].SourceCodeConfig
	user, repo, err := getUserRepoFromURL(sourceCodeConfig.URL)
	if err != nil {
		return "", err
	}
	events := []string{}
	if pipeline.Spec.TriggerWebhookPush {
		events = append(events, "push")
	}
	if pipeline.Spec.TriggerWebhookPr {
		events = append(events, "pull_request")
	}

	hookURL := fmt.Sprintf("%s?pipelineId=%s:%s", utils.CIEndpoint, pipeline.Namespace, pipeline.Name)
	id, err := c.createGithubWebhook(user, repo, accessToken, hookURL, pipeline.Status.Token, events)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (c *client) DeleteHook(pipeline *v3.Pipeline, accessToken string) error {
	if err := utils.ValidPipelineSpec(pipeline.Spec); err != nil {
		return err
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

	account.Spec.AvatarURL = gitaccount.GetAvatarURL()
	account.Spec.HTMLURL = gitaccount.GetHTMLURL()
	account.Spec.LoginName = gitaccount.GetLogin()
	account.Spec.DisplayName = gitaccount.GetName()

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

func (c *client) getFileContent(filename string, owner string, repo string, ref string, githubAccessToken string) (*github.RepositoryContent, error) {

	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s", c.API, owner, repo, filename)
	if ref != "" {
		url = url + "?ref=" + ref
	}
	resp, err := getFromGithub(githubAccessToken, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	fileContent := &github.RepositoryContent{}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(b, fileContent); err != nil {
		return nil, err
	}
	return fileContent, nil
}

func (c *client) GetPipelineFileInRepo(repoURL string, ref string, accessToken string) ([]byte, error) {
	owner, repo, err := getUserRepoFromURL(repoURL)
	if err != nil {
		return nil, err
	}
	content, err := c.getFileContent(".pipeline.yaml", owner, repo, ref, accessToken)
	if err != nil {
		//look for both suffix
		content, err = c.getFileContent(".pipeline.yml", owner, repo, ref, accessToken)
	}
	if err != nil {
		logrus.Debugf("error GetPipelineFileInRepo - %v", err)
		return nil, nil
	}
	if content.Content != nil {
		b, err := base64.StdEncoding.DecodeString(*content.Content)
		if err != nil {
			return nil, err
		}

		return b, nil
	}
	return nil, nil
}

func (c *client) GetDefaultBranch(repoURL string, accessToken string) (string, error) {
	owner, repo, err := getUserRepoFromURL(repoURL)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/repos/%s/%s", c.API, owner, repo)

	resp, err := getFromGithub(accessToken, url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	repository := &github.Repository{}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if err := json.Unmarshal(b, repository); err != nil {
		return "", err
	}
	if repository.DefaultBranch != nil {
		return *repository.DefaultBranch, nil
	}
	return "", nil

}

func (c *client) GetHeadCommit(repoURL string, branch string, credential *v3.SourceCodeCredential) (string, error) {

	if credential != nil {
		userName := credential.Spec.LoginName
		token := credential.Spec.AccessToken
		repoURL = strings.Replace(repoURL, "://", "://"+userName+":"+token+"@", 1)
	}

	return git.RemoteBranchHeadCommit(repoURL, branch)
}

func convertRepos(repos []github.Repository) []v3.SourceCodeRepository {
	result := []v3.SourceCodeRepository{}
	for _, repo := range repos {
		r := v3.SourceCodeRepository{}

		r.Spec.URL = repo.GetCloneURL()
		r.Spec.Language = repo.GetLanguage()
		r.Spec.DefaultBranch = repo.GetDefaultBranch()

		permissions := repo.GetPermissions()
		r.Spec.Permissions.Pull = permissions["pull"]
		r.Spec.Permissions.Push = permissions["push"]
		r.Spec.Permissions.Admin = permissions["admin"]

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

	if githubAccessToken != "" {
		req.Header.Add("Authorization", "token "+githubAccessToken)
	}
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
