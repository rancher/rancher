package github

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/google/go-github/github"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/rancher/pkg/pipeline/remote/model"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"github.com/tomnomnom/linkheader"
	"golang.org/x/oauth2"
	"io"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	defaultGithubAPI      = "https://api.github.com"
	defaultGithubHost     = "github.com"
	maxPerPage            = "100"
	gheAPI                = "/api/v3"
	hookConfigURL         = "url"
	hookConfigContentType = "content_type"
	hookConfigSecret      = "secret"
	hookConfigInsecureSSL = "insecure_ssl"
)

type client struct {
	Scheme       string
	Host         string
	ClientID     string
	ClientSecret string
	API          string
}

var defaultClient = &client{
	Scheme: "https://",
	Host:   defaultGithubHost,
	API:    defaultGithubAPI,
}

func New(config *v3.GithubPipelineConfig) (model.Remote, error) {
	if config == nil {
		return defaultClient, nil
	}
	ghClient := &client{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
	}
	if config.Hostname != "" && config.Hostname != defaultGithubHost {
		ghClient.Host = config.Hostname
		if config.TLS {
			ghClient.Scheme = "https://"
		} else {
			ghClient.Scheme = "http://"
		}
		ghClient.API = ghClient.Scheme + ghClient.Host + gheAPI
	} else {
		ghClient.Scheme = "https://"
		ghClient.Host = defaultGithubHost
		ghClient.API = defaultGithubAPI
	}
	return ghClient, nil
}

func (c *client) Type() string {
	return model.GithubType
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

func (c *client) Login(code string) (*v3.SourceCodeCredential, error) {
	githubOauthConfig := &oauth2.Config{
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
	user, repo, err := getUserRepoFromURL(pipeline.Spec.RepositoryURL)
	if err != nil {
		return "", err
	}

	hookURL := fmt.Sprintf("%s/hooks?pipelineId=%s:%s", settings.ServerURL.Get(), pipeline.Namespace, pipeline.Name)
	events := []string{utils.WebhookEventPush, utils.WebhookEventPullRequest}
	name := "web"
	active := true
	hook := &github.Hook{
		Name:   &name,
		Active: &active,
		Config: map[string]interface{}{
			hookConfigURL:         hookURL,
			hookConfigContentType: "json",
			hookConfigSecret:      pipeline.Status.Token,
			hookConfigInsecureSSL: "1",
		},
		Events: events,
	}
	url := fmt.Sprintf("%s/repos/%s/%s/hooks", c.API, user, repo)
	logrus.Debugf("hook to create:%v", hook)
	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(hook)

	resp, err := doRequestToGithub(http.MethodPost, url, accessToken, b)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(respData, hook)
	if err != nil {
		return "", err
	}

	return strconv.Itoa(hook.GetID()), err
}

func (c *client) DeleteHook(pipeline *v3.Pipeline, accessToken string) error {
	user, repo, err := getUserRepoFromURL(pipeline.Spec.RepositoryURL)
	if err != nil {
		return err
	}

	hook, err := c.getHook(pipeline, accessToken)
	if err != nil {
		return err
	}
	if hook != nil {
		url := fmt.Sprintf("%s/repos/%s/%s/hooks/%v", c.API, user, repo, hook.GetID())
		resp, err := doRequestToGithub(http.MethodDelete, url, accessToken, nil)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
	}
	return nil
}

func (c *client) getHook(pipeline *v3.Pipeline, accessToken string) (*github.Hook, error) {
	user, repo, err := getUserRepoFromURL(pipeline.Spec.RepositoryURL)
	if err != nil {
		return nil, err
	}

	var hooks []github.Hook
	var result *github.Hook
	url := fmt.Sprintf("%s/repos/%s/%s/hooks", c.API, user, repo)

	resp, err := getFromGithub(url, accessToken)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(b, &hooks); err != nil {
		return nil, err
	}
	for _, hook := range hooks {
		payloadURL, ok := hook.Config["url"].(string)
		if ok && strings.HasSuffix(payloadURL, fmt.Sprintf("hooks?pipelineId=%s", ref.Ref(pipeline))) {
			result = &hook
		}
	}
	return result, nil
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
	resp, err := getFromGithub(url, githubAccessToken)
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
	account.Spec.SourceCodeType = model.GithubType

	account.Spec.AvatarURL = gitaccount.GetAvatarURL()
	account.Spec.HTMLURL = gitaccount.GetHTMLURL()
	account.Spec.LoginName = gitaccount.GetLogin()
	account.Spec.GitLoginName = gitaccount.GetLogin()
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
	resp, err := getFromGithub(url, githubAccessToken)
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
	content, err := c.getFileContent(utils.PipelineFileYaml, owner, repo, ref, accessToken)
	if err != nil {
		//look for both suffix
		content, err = c.getFileContent(utils.PipelineFileYml, owner, repo, ref, accessToken)
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

func (c *client) SetPipelineFileInRepo(repoURL string, ref string, accessToken string, content []byte) error {

	owner, repo, err := getUserRepoFromURL(repoURL)
	if err != nil {
		return err
	}

	currentContent, err := c.getFileContent(utils.PipelineFileYml, owner, repo, ref, accessToken)
	currentFileName := utils.PipelineFileYml
	if err != nil {
		if httpErr, ok := err.(*httperror.APIError); !ok || httpErr.Code.Status != http.StatusNotFound {
			return err
		}
		//look for both suffix
		currentContent, err = c.getFileContent(utils.PipelineFileYaml, owner, repo, ref, accessToken)
		if err != nil {
			if httpErr, ok := err.(*httperror.APIError); !ok || httpErr.Code.Status != http.StatusNotFound {
				return err
			}
		} else {
			currentFileName = utils.PipelineFileYaml
		}
	}

	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s", c.API, owner, repo, currentFileName)
	message := "Create .rancher-pipeline.yml file"

	//contentStr := base64.StdEncoding.EncodeToString(content)
	options := &github.RepositoryContentFileOptions{
		Message: &message,
		Branch:  &ref,
		Content: content,
	}

	if currentContent != nil {
		//update pipeline file
		message = fmt.Sprintf("Update %s file", currentFileName)
		options.Message = &message
		options.SHA = currentContent.SHA
	}

	b, err := json.Marshal(options)
	if err != nil {
		return err
	}
	reader := bytes.NewReader(b)
	resp, err := doRequestToGithub(http.MethodPut, url, accessToken, reader)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (c *client) GetDefaultBranch(repoURL string, accessToken string) (string, error) {
	owner, repo, err := getUserRepoFromURL(repoURL)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/repos/%s/%s", c.API, owner, repo)

	resp, err := getFromGithub(url, accessToken)
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

func (c *client) GetBranches(repoURL string, accessToken string) ([]string, error) {
	owner, repo, err := getUserRepoFromURL(repoURL)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/repos/%s/%s/branches", c.API, owner, repo)

	resp, err := getFromGithub(url, accessToken)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	branches := []github.Branch{}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(b, &branches); err != nil {
		return nil, err
	}
	result := []string{}
	for _, b := range branches {
		result = append(result, b.GetName())
	}

	return result, nil
}

func (c *client) GetHeadInfo(repoURL string, branch string, accessToken string) (*model.BuildInfo, error) {
	owner, repo, err := getUserRepoFromURL(repoURL)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/repos/%s/%s/commits/%s", c.API, owner, repo, branch)

	resp, err := getFromGithub(url, accessToken)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	commit := github.RepositoryCommit{}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(b, &commit); err != nil {
		return nil, err
	}
	info := &model.BuildInfo{}
	info.Commit = commit.GetSHA()
	info.Ref = branch
	info.Branch = branch
	info.Message = commit.Commit.GetMessage()
	info.HTMLLink = commit.GetURL()
	info.Email = commit.Commit.Author.GetEmail()
	info.AvatarURL = commit.Author.GetAvatarURL()
	info.Author = commit.Author.GetLogin()

	return info, nil
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

	response, err := getFromGithub(url, githubAccessToken)
	if err != nil {
		return responses, err
	}
	responses = append(responses, response)
	nextURL := nextGithubPage(response)
	for nextURL != "" {
		response, err = getFromGithub(nextURL, githubAccessToken)
		if err != nil {
			return responses, err
		}
		responses = append(responses, response)
		nextURL = nextGithubPage(response)
	}

	return responses, nil
}

func getFromGithub(url string, accessToken string) (*http.Response, error) {
	return doRequestToGithub(http.MethodGet, url, accessToken, nil)
}

func doRequestToGithub(method string, url string, accessToken string, body io.Reader) (*http.Response, error) {
	logrus.Debug("doRequestToGithub", method, url)
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	client := &http.Client{
		Timeout: 15 * time.Second,
	}
	//set to max 100 per page to reduce query time
	if method == http.MethodGet {
		q := req.URL.Query()
		q.Set("per_page", maxPerPage)
		req.URL.RawQuery = q.Encode()
	}
	if accessToken != "" {
		req.Header.Add("Authorization", "token "+accessToken)
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.103 Safari/537.36)")
	resp, err := client.Do(req)
	if err != nil {
		return resp, err
	}
	// Check the status code
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		var body bytes.Buffer
		io.Copy(&body, resp.Body)
		return resp, httperror.NewAPIErrorLong(resp.StatusCode, "", body.String())
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

func getUserRepoFromURL(repoURL string) (string, string, error) {
	reg := regexp.MustCompile(".*/([^/]*?)/([^/]*?).git")
	match := reg.FindStringSubmatch(repoURL)
	if len(match) != 3 {
		return "", "", fmt.Errorf("error getting user/repo from gitrepoUrl:%v", repoURL)
	}
	return match[1], match[2], nil
}
